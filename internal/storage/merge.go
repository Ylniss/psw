package storage

import (
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
// Names match case-insensitively. On a content tie, remote wins.
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
		case inLocal && inRemote && !inFork:
			// F0 L1 R1
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
		case inLocal && inRemote && inFork:
			// F1 L1 R1
			chosen, action := pickByMTime(localRec, remoteRec)
			merged = append(merged, chosen)
			if action != actionUnchanged {
				summary.changes = append(summary.changes, mergeChange{chosen.Name, action})
			}
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
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
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
	return a.Name == b.Name && a.Username == b.Username && a.Password == b.Password && a.Value == b.Value
}

// printIfNonempty emits one yellow stderr line per change category.
func (m mergeSummary) printIfNonempty() {
	if len(m.changes) == 0 {
		return
	}
	var added, replaced, dropped, kept []string
	for _, c := range m.changes {
		switch c.action {
		case actionAddedFromRemote:
			added = append(added, c.name)
		case actionReplacedFromRemote:
			replaced = append(replaced, c.name)
		case actionDroppedByRemote:
			dropped = append(dropped, c.name)
		case actionKeptLocalOverRemoval, actionKeptLocalNewer:
			kept = append(kept, c.name)
		}
	}
	if len(added) > 0 {
		printWarn("Pulled %d new records from remote: %s", len(added), strings.Join(added, ", "))
	}
	if len(replaced) > 0 {
		printWarn("Replaced %d records with newer version from remote: %s", len(replaced), strings.Join(replaced, ", "))
	}
	if len(dropped) > 0 {
		printWarn("Dropped %d records removed on remote: %s", len(dropped), strings.Join(dropped, ", "))
	}
	if len(kept) > 0 {
		printWarn("Kept %d local records (newer than remote): %s", len(kept), strings.Join(kept, ", "))
	}
}
