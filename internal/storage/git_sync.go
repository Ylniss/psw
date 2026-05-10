package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	color "github.com/TwiN/go-color"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"

	"github.com/ylniss/psw/internal/ui"
)

// ErrForkUndecryptable: fork or remote storage.psw can't be decrypted with
// the current main password (usually because `change main` ran elsewhere).
var ErrForkUndecryptable = errors.New("storage was re-encrypted under a different main password since the last sync")

// ForkUndecryptableUserMessage is the banner shown for ErrForkUndecryptable.
const ForkUndecryptableUserMessage = "Storage was re-encrypted under a different main password since the last sync. Push from the device that ran 'change main' first, then retry on this device."

// shouldUseRemote reports whether fetch/push are allowed.
// False when: PSW_GIT=0, PSW_GIT_REMOTE=0, or no remote in config.
func shouldUseRemote() bool {
	if os.Getenv("PSW_GIT") == "0" {
		return false
	}
	if os.Getenv("PSW_GIT_REMOTE") == "0" {
		return false
	}
	if !Paths.gitRepoExists {
		return false
	}
	return AppConfig.Remote != ""
}

// gitNetworkTimeout caps fetch/push to prevent indefinite hangs.
const gitNetworkTimeout = 5 * time.Second

// WarnSink, when non-nil, receives warning messages instead of stderr.
// Set by hosts that own the screen (e.g. menu mode).
var WarnSink func(string)

// Warn writes a yellow line through WarnSink, or stderr when sink is nil.
func Warn(format string, args ...any) {
	msg := color.InYellow(fmt.Sprintf(format, args...))
	if WarnSink != nil {
		WarnSink(msg)
		return
	}
	fmt.Fprintln(os.Stderr, msg)
}

// runGit and runGitNetwork are the shell-git fallback path for auth/signing cases go-git can't handle.

// runGit runs git in the storage dir; returns combined stdout+stderr.
func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = Paths.storagePath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runGitNetwork is runGit with gitNetworkTimeout. Used as fetch/push fallback.
func runGitNetwork(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitNetworkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = Paths.storagePath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// redactURL replaces the password in a URL with "xxxxx" for safe logging.
func redactURL(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return u
	}
	return p.Redacted()
}

// printWarn delegates to Warn — kept as a thin wrapper so existing callers
// across the package don't need to be updated when a host installs WarnSink.
func printWarn(format string, args ...any) { Warn(format, args...) }

// shouldFallbackToShell reports whether a go-git network error means we should retry via shell git: our own pre-call sentinel, go-git's transport sentinels, or the SSH "no supported methods" handshake error.
func shouldFallbackToShell(err error) bool {
	if errors.Is(err, ErrAuthRequiresHelper) {
		return true
	}
	if errors.Is(err, transport.ErrAuthenticationRequired) {
		return true
	}
	if errors.Is(err, transport.ErrAuthorizationFailed) {
		return true
	}
	if errors.Is(err, transport.ErrInvalidAuthMethod) {
		return true
	}
	if err != nil && strings.Contains(err.Error(), "no supported methods remain") {
		return true
	}
	return false
}

// GitFetch fetches origin/<branch>. Tries go-git first; falls back to shell
// `git fetch` when auth requires a helper and `git` is on PATH.
func GitFetch() error {
	if !shouldUseRemote() {
		return nil
	}
	if err := ensureGitRemote(); err != nil {
		return fmt.Errorf("ensure remote: %w", err)
	}
	branch, err := detectBranch()
	if err != nil {
		return err
	}
	err = ui.WithSpinner("Pulling from remote", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), gitNetworkTimeout)
		defer cancel()
		goGitErr := gitFetchGoGit(ctx, branch)
		if !shouldFallbackToShell(goGitErr) {
			return goGitErr
		}
		if !gitOnPath() {
			return goGitErr
		}
		slog.Debug("falling back to shell git fetch", "go_git_err", goGitErr)
		out, shellErr := runGitNetwork("fetch", "origin", branch)
		if shellErr != nil {
			return fmt.Errorf("%s", strings.TrimSpace(out))
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("git fetch %s: %w", redactURL(AppConfig.Remote), err)
	}
	slog.Debug("git fetch ok", "remote", redactURL(AppConfig.Remote), "branch", branch)
	return nil
}

func gitFetchGoGit(ctx context.Context, branch string) error {
	repo, err := openRepo()
	if err != nil {
		return err
	}
	auth, err := gitAuth(AppConfig.Remote)
	if err != nil {
		return err
	}
	refspec := config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/remotes/origin/%s", branch, branch))
	err = repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{refspec},
		Auth:       auth,
	})
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

// GitPush pushes the current branch to origin. Errors print yellow to stderr;
// the function never returns an error.
func GitPush() {
	if !shouldUseRemote() {
		return
	}
	if err := ensureGitRemote(); err != nil {
		printWarn("git remote setup failed: %v", err)
		return
	}
	branch, err := detectBranch()
	if err != nil {
		printWarn("git push: %v", err)
		return
	}
	err = ui.WithSpinner("Pushing to remote", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), gitNetworkTimeout)
		defer cancel()
		goGitErr := gitPushGoGit(ctx, branch)
		if !shouldFallbackToShell(goGitErr) {
			return goGitErr
		}
		if !gitOnPath() {
			return goGitErr
		}
		slog.Debug("falling back to shell git push", "go_git_err", goGitErr)
		out, shellErr := runGitNetwork("push", "origin", branch)
		if shellErr != nil {
			return fmt.Errorf("%s", strings.TrimSpace(out))
		}
		return nil
	})
	if err != nil {
		slog.Debug("git push failed", "remote", redactURL(AppConfig.Remote), "branch", branch, "err", err)
		printWarn("git push to %s failed: %v", redactURL(AppConfig.Remote), err)
		return
	}
	slog.Debug("git push ok", "remote", redactURL(AppConfig.Remote), "branch", branch)
}

func gitPushGoGit(ctx context.Context, branch string) error {
	repo, err := openRepo()
	if err != nil {
		return err
	}
	auth, err := gitAuth(AppConfig.Remote)
	if err != nil {
		return err
	}
	refspec := config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch))
	err = repo.PushContext(ctx, &git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{refspec},
		Auth:       auth,
	})
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

// gitPullAndMerge fetches origin/<branch> and reconciles before mutation.
// Worktree.Pull only handles fast-forward; the divergent case here decrypts
// both sides and runs the 3-way smart merge.
//
// Returns:
//
//	nil                  — success, no-op, or warning printed
//	ErrForkUndecryptable — fork or remote can't decrypt with current password
//	wrapped error        — unexpected git failure
func gitPullAndMerge(mainPassword string) error {
	if !shouldUseRemote() {
		return nil
	}

	if err := GitFetch(); err != nil {
		printWarn("%v (continuing with local state)", err)
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
	hash, err := parseFullHash(remoteSHA)
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

	mergedJSON, err := marshalRecords(merged)
	if err != nil {
		return err
	}
	if err := EncryptStringToStorage(mergedJSON, mainPassword); err != nil {
		return fmt.Errorf("encrypt merged: %w", err)
	}

	localHash, err := parseFullHash(localSHA)
	if err != nil {
		return err
	}
	remoteHash, err := parseFullHash(remoteSHA)
	if err != nil {
		return err
	}
	msg := buildMergeMessage(summary)
	if err := gitStorageMergeCommit(localHash, remoteHash, msg); err != nil {
		return err
	}

	summary.printIfNonempty()
	return nil
}

// parseFullHash validates a 40-hex SHA-1 string. plumbing.NewHash silently
// returns ZeroHash on malformed input; this catches that.
func parseFullHash(sha string) (plumbing.Hash, error) {
	if len(sha) != 40 {
		return plumbing.ZeroHash, fmt.Errorf("invalid sha length %d (want 40): %q", len(sha), sha)
	}
	for _, r := range sha {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return plumbing.ZeroHash, fmt.Errorf("invalid sha hex: %q", sha)
		}
	}
	return plumbing.NewHash(sha), nil
}

// marshalRecords serializes records to indented JSON via Storage.ToJson.
func marshalRecords(records []Record) (string, error) {
	out, err := (&Storage{Records: records}).ToJson()
	if err != nil {
		return "", fmt.Errorf("marshal merged: %w", err)
	}
	return out, nil
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
	var added, replaced, dropped, kept int
	for _, c := range s.changes {
		switch c.action {
		case actionAddedFromRemote:
			added++
		case actionReplacedFromRemote:
			replaced++
		case actionDroppedByRemote:
			dropped++
		case actionKeptLocalOverRemoval, actionKeptLocalNewer:
			kept++
		}
	}
	var parts []string
	if added > 0 {
		parts = append(parts, fmt.Sprintf("+%d", added))
	}
	if replaced > 0 {
		parts = append(parts, fmt.Sprintf("~%d", replaced))
	}
	if dropped > 0 {
		parts = append(parts, fmt.Sprintf("-%d", dropped))
	}
	if kept > 0 {
		parts = append(parts, fmt.Sprintf("kept-local %d", kept))
	}
	if len(parts) == 0 {
		return "merge: no record changes"
	}
	return "merge: " + strings.Join(parts, ", ")
}
