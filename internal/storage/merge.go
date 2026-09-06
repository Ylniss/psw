package storage

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strings"
)

type mergeAction int

const (
	actionUnchanged mergeAction = iota
	actionAddedFromRemote
	actionReplacedFromRemote
	actionDroppedByRemote
	actionKeptLocalOverRemoval
	actionKeptLocalNewer
)

type mergeChange struct {
	name   string
	action mergeAction
}

type mergeSummary struct {
	changes []mergeChange
}

// mergeRecords does a 3-way merge using MTime as the last-write-wins (LWW) signal.
// Names match case-insensitively. On an mtime tie, remote wins.
//
// F=fork present, L=local present, R=remote present:
//
//	F0 L1 R0 → keep L          (local-added since fork)
//	F0 L0 R1 → take R          (remote-added since fork)
//	F0 L1 R1 → mtime LWW       (independent additions, same name)
//	F1 L1 R0 → L if L>F else drop  (modification beats removal; otherwise honor removal)
//	F1 L0 R1 → R if R>F else drop  (symmetric)
//	F1 L1 R1 → mtime LWW       (standard content reconcile)
//	F1 L0 R0 → drop            (both removed)
func mergeRecords(fork, local, remote []Record) ([]Record, mergeSummary) {
	forkByName := indexByName(fork)
	localByName := indexByName(local)
	remoteByName := indexByName(remote)

	keys := unionKeys(forkByName, localByName, remoteByName)
	slices.Sort(keys)

	var merged []Record
	var summary mergeSummary

	for _, k := range keys {
		forkRec, inFork := forkByName[k]
		localRec, inLocal := localByName[k]
		remoteRec, inRemote := remoteByName[k]

		switch {
		case !inLocal && !inRemote:
			// both removed (F1 L0 R0) or never existed (F0 L0 R0); drop
			continue
		case inLocal && !inFork && !inRemote:
			// F0 L1 R0
			merged = append(merged, localRec)
		case !inLocal && !inFork && inRemote:
			// F0 L0 R1
			merged = append(merged, remoteRec)
			summary.changes = append(summary.changes, mergeChange{remoteRec.Name, actionAddedFromRemote})
		case inLocal && inRemote:
			// F0 L1 R1 and F1 L1 R1 — mtime LWW either way
			chosen, action := pickByMTime(localRec, remoteRec)
			merged = append(merged, chosen)
			if action != actionUnchanged {
				summary.changes = append(summary.changes, mergeChange{chosen.Name, action})
			}
		case inLocal && !inRemote && inFork:
			// F1 L1 R0
			if localRec.MTime > forkRec.MTime {
				merged = append(merged, localRec)
				summary.changes = append(summary.changes, mergeChange{localRec.Name, actionKeptLocalOverRemoval})
			} else {
				summary.changes = append(summary.changes, mergeChange{localRec.Name, actionDroppedByRemote})
			}
		case !inLocal && inRemote && inFork:
			// F1 L0 R1
			if remoteRec.MTime > forkRec.MTime {
				merged = append(merged, remoteRec)
				summary.changes = append(summary.changes, mergeChange{remoteRec.Name, actionAddedFromRemote})
			}
			// local removed; remote unchanged → drop silently
		}
	}

	slices.SortFunc(merged, func(a, b Record) int {
		return strings.Compare(a.Name, b.Name)
	})
	return merged, summary
}

func indexByName(records []Record) map[string]Record {
	m := make(map[string]Record, len(records))
	for _, r := range records {
		m[strings.ToLower(r.Name)] = r
	}
	return m
}

func unionKeys(indexes ...map[string]Record) []string {
	seen := map[string]struct{}{}
	for _, idx := range indexes {
		for k := range idx {
			seen[k] = struct{}{}
		}
	}
	return slices.Collect(maps.Keys(seen))
}

// pickByMTime: byte-equal → unchanged (use local); higher mtime wins; tie → remote.
func pickByMTime(local, remote Record) (Record, mergeAction) {
	if recordsEqual(local, remote) {
		return local, actionUnchanged
	}
	if local.MTime > remote.MTime {
		return local, actionKeptLocalNewer
	}
	return remote, actionReplacedFromRemote
}

func recordsEqual(a, b Record) bool {
	return a.Name == b.Name && a.Username == b.Username &&
		bytes.Equal(a.Password, b.Password) && bytes.Equal(a.Value, b.Value)
}

// mergeBuckets is mergeSummary.changes split by action category.
type mergeBuckets struct {
	added, replaced, dropped, kept []string
}

func (m mergeSummary) bucket() mergeBuckets {
	var b mergeBuckets
	for _, c := range m.changes {
		switch c.action {
		case actionAddedFromRemote:
			b.added = append(b.added, c.name)
		case actionReplacedFromRemote:
			b.replaced = append(b.replaced, c.name)
		case actionDroppedByRemote:
			b.dropped = append(b.dropped, c.name)
		case actionKeptLocalOverRemoval, actionKeptLocalNewer:
			b.kept = append(b.kept, c.name)
		}
	}
	return b
}

// printSummary emits one Warn line per non-empty change category.
func (m mergeSummary) printSummary() {
	if len(m.changes) == 0 {
		return
	}
	b := m.bucket()
	if len(b.added) > 0 {
		Warn("Pulled %d new records from remote: %s", len(b.added), strings.Join(b.added, ", "))
	}
	if len(b.replaced) > 0 {
		Warn("Replaced %d records with newer version from remote: %s", len(b.replaced), strings.Join(b.replaced, ", "))
	}
	if len(b.dropped) > 0 {
		Warn("Dropped %d records removed on remote: %s", len(b.dropped), strings.Join(b.dropped, ", "))
	}
	if len(b.kept) > 0 {
		Warn("Kept %d local records (newer than remote): %s", len(b.kept), strings.Join(b.kept, ", "))
	}
}

// mergeMessage renders the git commit subject for a merge.
func (m mergeSummary) mergeMessage() string {
	b := m.bucket()
	var parts []string
	if len(b.added) > 0 {
		parts = append(parts, fmt.Sprintf("+%d", len(b.added)))
	}
	if len(b.replaced) > 0 {
		parts = append(parts, fmt.Sprintf("~%d", len(b.replaced)))
	}
	if len(b.dropped) > 0 {
		parts = append(parts, fmt.Sprintf("-%d", len(b.dropped)))
	}
	if len(b.kept) > 0 {
		parts = append(parts, fmt.Sprintf("kept-local %d", len(b.kept)))
	}
	if len(parts) == 0 {
		return "merge: no record changes"
	}
	return "merge: " + strings.Join(parts, ", ")
}
