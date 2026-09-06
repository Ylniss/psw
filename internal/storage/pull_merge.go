package storage

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/awnumar/memguard"
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
	records, err := LoadCommitRecords(remoteSHA, mainPassword)
	if err != nil {
		return nil, ErrForkUndecryptable
	}
	if err := gitFastForward(remoteSHA); err != nil {
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

	forkRecords, err := LoadCommitRecords(forkSHA, mainPassword)
	if err != nil {
		return nil, ErrForkUndecryptable
	}
	remoteRecords, err := LoadCommitRecords(remoteSHA, mainPassword)
	if err != nil {
		return nil, ErrForkUndecryptable
	}
	localStore, err := Decrypt(mainPassword)
	if err != nil {
		return nil, fmt.Errorf("decrypt local: %w", err)
	}

	merged, summary := mergeRecords(forkRecords, localStore.Records, remoteRecords)

	mergedJSON, err := (&Storage{Records: merged}).MarshalRecords()
	if err != nil {
		return nil, fmt.Errorf("marshal merged: %w", err)
	}
	if err := EncryptToStorage(mergedJSON, mainPassword); err != nil {
		return nil, fmt.Errorf("encrypt merged: %w", err)
	}

	msg := summary.mergeMessage()
	if err := gitStorageMergeCommit(localSHA, remoteSHA, msg); err != nil {
		return nil, err
	}

	summary.printSummary()
	return &Storage{MainPassword: mainPassword, Records: merged}, nil
}
