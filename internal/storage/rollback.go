package storage

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/awnumar/memguard"
)

// LoadCommitRecords reads <ref>:storage.psw and returns the decoded record
// slice. Any decrypt failure maps to ErrForkUndecryptable — same posture as
// fastForward / divergentMerge in pull_merge.go.
func LoadCommitRecords(ref string, password *memguard.Enclave) ([]Record, error) {
	blob, err := gitShowBlob(ref, Paths.storageFileName)
	if err != nil {
		return nil, fmt.Errorf("read commit blob: %w", err)
	}
	pwBuf, err := password.Open()
	if err != nil {
		return nil, fmt.Errorf("open password enclave: %w", err)
	}
	defer pwBuf.Destroy()
	plain, err := decryptBlob(blob, pwBuf.Bytes())
	if err != nil {
		return nil, ErrForkUndecryptable
	}
	defer memguard.WipeBytes(plain)
	var records []Record
	if err := json.Unmarshal(plain, &records); err != nil {
		return nil, fmt.Errorf("parse commit records: %w", err)
	}
	return records, nil
}

// ApplyRollback replaces store.Records with a copy of the supplied snapshot
// (each MTime stamped to now), persists, then commits. Push is best-effort
// inside GitCommit.
func ApplyRollback(store *Storage, target LogEntry, records []Record) error {
	now := time.Now().UnixMilli()
	stamped := make([]Record, len(records))
	for i, r := range records {
		r.MTime = now
		stamped[i] = r
	}
	store.Records = stamped
	if err := store.Save(); err != nil {
		return err
	}
	GitCommit(fmt.Sprintf("rollback to %s: %s", target.ShortSHA, target.Message))
	return nil
}

// HeadShortSHA returns the 7-char short SHA of HEAD.
func HeadShortSHA() (string, error) {
	sha, err := revParse("HEAD")
	if err != nil {
		return "", err
	}
	return sha[:7], nil
}

// RollbackPicks returns the rollback candidates as picker labels, newest
// first, plus the label→entry map a selection is resolved through. HEAD is
// dropped — rolling back to it would change nothing. No candidates → empty
// labels; the caller words its own "nothing to roll back to" message.
func RollbackPicks() (labels []string, byLabel map[string]LogEntry, err error) {
	entries, err := GitLog()
	if err != nil {
		return nil, nil, err
	}
	headShort, err := HeadShortSHA()
	if err != nil {
		return nil, nil, err
	}
	picks := make([]LogEntry, 0, len(entries))
	for _, e := range entries {
		if e.ShortSHA != headShort {
			picks = append(picks, e)
		}
	}
	slices.Reverse(picks) // newest first — most likely rollback targets at the top
	labels = make([]string, len(picks))
	byLabel = make(map[string]LogEntry, len(picks))
	for i, e := range picks {
		label := fmt.Sprintf("%s  %s  %s", e.ShortSHA, e.Time.Format("2006-01-02 15:04"), e.Message)
		labels[i] = label
		byLabel[label] = e
	}
	return labels, byLabel, nil
}
