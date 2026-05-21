package storage

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/awnumar/memguard"
	"github.com/go-git/go-git/v5/plumbing"
)

// ErrForkUndecryptable: fork or remote storage.psw can't be decrypted with
// the current main password (usually because `change main` ran elsewhere).
var ErrForkUndecryptable = errors.New("storage was re-encrypted under a different main password since the last sync")

// ForkUndecryptableUserMessage is the banner shown for ErrForkUndecryptable.
const ForkUndecryptableUserMessage = "Your main password was changed on another device. Push from that device first, then retry here."

// GitPullAndMerge fetches origin/<branch> and reconciles. Worktree.Pull
// only handles fast-forward; the divergent case here decrypts both sides
// and runs the 3-way smart merge.
//
// Returns:
//
//	(s, nil)                    — merge ran; s is decrypted, callers skip Decrypt.
//	(nil, nil)                  — no-op: no remote, fetch warned, in sync, or ahead.
//	(nil, ErrForkUndecryptable) — fork/remote can't decrypt under current password.
//	(nil, wrapped err)          — unexpected git failure.
func GitPullAndMerge(mainPassword *memguard.Enclave) (*Storage, error) {
	if !shouldUseRemote() {
		return nil, nil
	}

	if err := GitFetch(); err != nil {
		Warn("%v (continuing with local state)", err)
		return nil, nil
	}

	branch, err := detectBranch()
	if err != nil {
		return nil, err
	}
	remoteRef := "refs/remotes/origin/" + branch

	// First-push case: remote tracking ref doesn't exist yet.
	remoteSHA, err := revParse(remoteRef)
	if err != nil {
		return nil, nil
	}
	localSHA, err := revParse("HEAD")
	if err != nil {
		return nil, err
	}
	if localSHA == remoteSHA {
		return nil, nil
	}
	// Already ahead.
	if isAncestor(remoteSHA, localSHA) {
		return nil, nil
	}
	// Fast-forward.
	if isAncestor(localSHA, remoteSHA) {
		return fastForward(remoteSHA, mainPassword)
	}
	// Divergent: 3-way merge.
	return divergentMerge(remoteSHA, mainPassword)
}

func fastForward(remoteSHA string, mainPassword *memguard.Enclave) (*Storage, error) {
	records, err := decryptBlobToRecords(remoteSHA, mainPassword)
	if err != nil {
		return nil, ErrForkUndecryptable
	}
	hash, err := parseSHA1Hash(remoteSHA)
	if err != nil {
		return nil, err
	}
	if err := gitFastForward(hash); err != nil {
		return nil, err
	}
	slog.Debug("git fast-forwarded", "to", remoteSHA)
	return &Storage{MainPassword: mainPassword, Records: records}, nil
}

func divergentMerge(remoteSHA string, mainPassword *memguard.Enclave) (*Storage, error) {
	localSHA, err := revParse("HEAD")
	if err != nil {
		return nil, err
	}
	forkSHA, err := gitMergeBase(localSHA, remoteSHA)
	if err != nil {
		return nil, err
	}

	forkRecords, err := decryptBlobToRecords(forkSHA, mainPassword)
	if err != nil {
		return nil, ErrForkUndecryptable
	}
	remoteRecords, err := decryptBlobToRecords(remoteSHA, mainPassword)
	if err != nil {
		return nil, ErrForkUndecryptable
	}
	localPlain, err := DecryptFromStorage(mainPassword)
	if err != nil {
		return nil, fmt.Errorf("decrypt local: %w", err)
	}
	defer memguard.WipeBytes(localPlain)
	var localRecords []Record
	if err := json.Unmarshal(localPlain, &localRecords); err != nil {
		return nil, fmt.Errorf("parse local records: %w", err)
	}

	merged, summary := mergeRecords(forkRecords, localRecords, remoteRecords)

	mergedJSON, err := (&Storage{Records: merged}).MarshalRecords()
	if err != nil {
		return nil, fmt.Errorf("marshal merged: %w", err)
	}
	if err := EncryptToStorage(mergedJSON, mainPassword); err != nil {
		return nil, fmt.Errorf("encrypt merged: %w", err)
	}

	localHash, err := parseSHA1Hash(localSHA)
	if err != nil {
		return nil, err
	}
	remoteHash, err := parseSHA1Hash(remoteSHA)
	if err != nil {
		return nil, err
	}
	msg := buildMergeMessage(summary)
	if err := gitStorageMergeCommit(localHash, remoteHash, msg); err != nil {
		return nil, err
	}

	summary.printSummary()
	return &Storage{MainPassword: mainPassword, Records: merged}, nil
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

func decryptBlobToRecords(ref string, password *memguard.Enclave) ([]Record, error) {
	blob, err := gitShowBlob(ref, Paths.storageFileName)
	if err != nil {
		return nil, fmt.Errorf("git show %s: %w", ref, err)
	}
	pwBuf, err := password.Open()
	if err != nil {
		return nil, fmt.Errorf("open password enclave: %w", err)
	}
	defer pwBuf.Destroy()
	plain, err := decryptBlob(blob, pwBuf.Bytes())
	if err != nil {
		return nil, err
	}
	defer memguard.WipeBytes(plain)
	var records []Record
	if err := json.Unmarshal(plain, &records); err != nil {
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
