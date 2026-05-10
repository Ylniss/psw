package storage

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

// ErrForkUndecryptable: fork or remote storage.psw can't be decrypted with
// the current main password (usually because `change main` ran elsewhere).
var ErrForkUndecryptable = errors.New("storage was re-encrypted under a different main password since the last sync")

// ForkUndecryptableUserMessage is the banner shown for ErrForkUndecryptable.
const ForkUndecryptableUserMessage = "Storage was re-encrypted under a different main password since the last sync. Push from the device that ran 'change main' first, then retry on this device."

// GitPullAndMerge fetches origin/<branch> and reconciles. Worktree.Pull
// only handles fast-forward; the divergent case here decrypts both sides
// and runs the 3-way smart merge.
//
// Returns:
//
//	nil                  — success, no-op, or warning printed
//	ErrForkUndecryptable — fork or remote can't decrypt with current password
//	wrapped error        — unexpected git failure
func GitPullAndMerge(mainPassword string) error {
	if !shouldUseRemote() {
		return nil
	}

	if err := GitFetch(); err != nil {
		Warn("%v (continuing with local state)", err)
		return nil
	}

	branch, err := detectBranch()
	if err != nil {
		return err
	}
	remoteRef := "refs/remotes/origin/" + branch

	// First-push case: remote tracking ref doesn't exist yet.
	remoteSHA, err := revParse(remoteRef)
	if err != nil {
		return nil
	}
	localSHA, err := revParse("HEAD")
	if err != nil {
		return err
	}
	if localSHA == remoteSHA {
		return nil
	}
	// Already ahead.
	if isAncestor(remoteSHA, localSHA) {
		return nil
	}
	// Fast-forward.
	if isAncestor(localSHA, remoteSHA) {
		return fastForward(remoteSHA, mainPassword)
	}
	// Divergent: 3-way merge.
	return divergentMerge(remoteSHA, mainPassword)
}

func fastForward(remoteSHA, mainPassword string) error {
	if _, err := decryptBlobToRecords(remoteSHA, mainPassword); err != nil {
		return ErrForkUndecryptable
	}
	hash, err := parseSHA1Hash(remoteSHA)
	if err != nil {
		return err
	}
	if err := gitFastForward(hash); err != nil {
		return err
	}
	slog.Debug("git fast-forwarded", "to", remoteSHA)
	return nil
}

func divergentMerge(remoteSHA, mainPassword string) error {
	localSHA, err := revParse("HEAD")
	if err != nil {
		return err
	}
	forkSHA, err := gitMergeBase(localSHA, remoteSHA)
	if err != nil {
		return err
	}

	forkRecords, err := decryptBlobToRecords(forkSHA, mainPassword)
	if err != nil {
		return ErrForkUndecryptable
	}
	remoteRecords, err := decryptBlobToRecords(remoteSHA, mainPassword)
	if err != nil {
		return ErrForkUndecryptable
	}
	localPlain, err := DecryptStringFromStorage(mainPassword)
	if err != nil {
		return fmt.Errorf("decrypt local: %w", err)
	}
	var localRecords []Record
	if err := json.Unmarshal([]byte(localPlain), &localRecords); err != nil {
		return fmt.Errorf("parse local records: %w", err)
	}

	merged, summary := mergeRecords(forkRecords, localRecords, remoteRecords)

	mergedJSON, err := (&Storage{Records: merged}).ToJSON()
	if err != nil {
		return fmt.Errorf("marshal merged: %w", err)
	}
	if err := EncryptStringToStorage(mergedJSON, mainPassword); err != nil {
		return fmt.Errorf("encrypt merged: %w", err)
	}

	localHash, err := parseSHA1Hash(localSHA)
	if err != nil {
		return err
	}
	remoteHash, err := parseSHA1Hash(remoteSHA)
	if err != nil {
		return err
	}
	msg := buildMergeMessage(summary)
	if err := gitStorageMergeCommit(localHash, remoteHash, msg); err != nil {
		return err
	}

	summary.printSummary()
	return nil
}

// parseSHA1Hash validates a 40-hex SHA-1 string. plumbing.NewHash silently
// returns ZeroHash on malformed input; this catches that.
func parseSHA1Hash(sha string) (plumbing.Hash, error) {
	if len(sha) != 40 {
		return plumbing.ZeroHash, fmt.Errorf("invalid sha length %d (want 40): %q", len(sha), sha)
	}
	if _, err := hex.DecodeString(sha); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("invalid sha hex: %q", sha)
	}
	return plumbing.NewHash(sha), nil
}

func decryptBlobToRecords(ref, password string) ([]Record, error) {
	blob, err := gitShowBlob(ref, Paths.storageFileName)
	if err != nil {
		return nil, fmt.Errorf("git show %s: %w", ref, err)
	}
	plain, err := decryptBytes(blob, password)
	if err != nil {
		return nil, err
	}
	var records []Record
	if err := json.Unmarshal([]byte(plain), &records); err != nil {
		return nil, fmt.Errorf("parse records: %w", err)
	}
	return records, nil
}

func buildMergeMessage(s mergeSummary) string {
	b := s.bucket()
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
