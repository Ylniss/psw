package storage

import (
	"encoding/json"
	"fmt"
	"time"
)

// LoadCommitRecords reads <ref>:storage.psw and returns the decoded record
// slice. Any decrypt failure maps to ErrForkUndecryptable — same posture as
// fastForward / divergentMerge in pull_merge.go.
func LoadCommitRecords(ref, password string) ([]Record, error) {
	blob, err := gitShowBlob(ref, Paths.storageFileName)
	if err != nil {
		return nil, fmt.Errorf("read commit blob: %w", err)
	}
	plain, err := decryptBytes(blob, password)
	if err != nil {
		return nil, ErrForkUndecryptable
	}
	var records []Record
	if err := json.Unmarshal([]byte(plain), &records); err != nil {
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
	if len(sha) < 7 {
		return "", fmt.Errorf("HEAD sha too short: %q", sha)
	}
	return sha[:7], nil
}
