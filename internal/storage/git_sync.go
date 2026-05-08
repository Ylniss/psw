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

	"github.com/ylniss/psw/internal/ui"
)

// ErrForkUndecryptable: fork or remote storage.psw can't be decrypted with
// the current main password (usually because `change main` ran elsewhere).
var ErrForkUndecryptable = errors.New("storage was re-encrypted under a different main password since the last sync")

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

func detectBranch() (string, error) {
	out, err := runGit("symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("detect branch: %s", strings.TrimSpace(out))
	}
	return strings.TrimSpace(out), nil
}

// gitNetworkTimeout caps fetch/push to prevent indefinite hangs.
const gitNetworkTimeout = 30 * time.Second

// runGit runs git in the storage dir; returns combined stdout+stderr.
func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = Paths.storagePath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runGitNetwork is runGit with gitNetworkTimeout. Use for fetch/push only.
func runGitNetwork(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitNetworkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = Paths.storagePath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runGitNetworkSpinner wraps runGitNetwork with a labeled spinner on stderr.
func runGitNetworkSpinner(label string, args ...string) (string, error) {
	var out string
	err := ui.WithSpinner(label, func() error {
		var runErr error
		out, runErr = runGitNetwork(args...)
		return runErr
	})
	return out, err
}

// runGitStdout runs git and returns stdout. On error, the error includes stderr.
func runGitStdout(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = Paths.storagePath
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, errors.New(strings.TrimSpace(stderr.String()))
	}
	return []byte(stdout.String()), nil
}

// ensureGitRemote sets origin to AppConfig.Remote (creates or updates).
func ensureGitRemote() error {
	out, err := runGit("remote", "get-url", "origin")
	if err != nil {
		if addOutput, addErr := runGit("remote", "add", "origin", AppConfig.Remote); addErr != nil {
			return fmt.Errorf("git remote add: %s", strings.TrimSpace(addOutput))
		}
		return nil
	}
	if strings.TrimSpace(out) == AppConfig.Remote {
		return nil
	}
	if setOutput, err := runGit("remote", "set-url", "origin", AppConfig.Remote); err != nil {
		return fmt.Errorf("git remote set-url: %s", strings.TrimSpace(setOutput))
	}
	return nil
}

// redactURL replaces the password in a URL with "xxxxx" for safe logging.
func redactURL(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return u
	}
	return p.Redacted()
}

// printWarn writes a yellow line to stderr.
func printWarn(format string, args ...any) {
	fmt.Fprintln(os.Stderr, color.InYellow(fmt.Sprintf(format, args...)))
}

// gitShowBlob returns the bytes of <path> at git ref <ref>.
func gitShowBlob(ref, path string) ([]byte, error) {
	return runGitStdout("show", ref+":"+path)
}

func gitMergeBase(a, b string) (string, error) {
	out, err := runGit("merge-base", a, b)
	if err != nil {
		return "", fmt.Errorf("merge-base: %s", strings.TrimSpace(out))
	}
	return strings.TrimSpace(out), nil
}

func revParse(ref string) (string, error) {
	out, err := runGit("rev-parse", ref)
	if err != nil {
		return "", fmt.Errorf("rev-parse %s: %s", ref, strings.TrimSpace(out))
	}
	return strings.TrimSpace(out), nil
}

// isAncestor reports whether a is an ancestor of b.
func isAncestor(a, b string) bool {
	_, err := runGit("merge-base", "--is-ancestor", a, b)
	return err == nil
}

// GitFetch fetches origin/<branch>.
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
	out, err := runGitNetworkSpinner("Pulling from remote", "fetch", "origin", branch)
	if err != nil {
		return fmt.Errorf("git fetch %s: %s", redactURL(AppConfig.Remote), strings.TrimSpace(out))
	}
	slog.Debug("git fetch ok", "remote", redactURL(AppConfig.Remote), "branch", branch)
	return nil
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
	out, err := runGitNetworkSpinner("Pushing to remote", "push", "origin", branch)
	if err != nil {
		slog.Debug("git push failed", "remote", redactURL(AppConfig.Remote), "branch", branch, "output", out)
		printWarn("git push to %s failed: %s", redactURL(AppConfig.Remote), strings.TrimSpace(out))
		return
	}
	slog.Debug("git push ok", "remote", redactURL(AppConfig.Remote), "branch", branch)
}

// gitPullAndMerge fetches origin/<branch> and reconciles before mutation.
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

	// Remote tracking ref doesn't exist yet (first push not done).
	if _, err := runGit("rev-parse", "--verify", remoteRef); err != nil {
		return nil
	}

	localSHA, err := revParse("HEAD")
	if err != nil {
		return err
	}
	remoteSHA, err := revParse(remoteRef)
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
	if out, err := runGit("merge", "--ff-only", remoteSHA); err != nil {
		return fmt.Errorf("git merge --ff-only: %s", strings.TrimSpace(out))
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

	mergedJSON, err := (&Storage{Records: merged}).ToJson()
	if err != nil {
		return fmt.Errorf("marshal merged: %w", err)
	}
	if err := EncryptStringToStorage(mergedJSON, mainPassword); err != nil {
		return fmt.Errorf("encrypt merged: %w", err)
	}

	// Two-parent merge commit: -s ours keeps local tree, then we overwrite storage.psw.
	if out, err := runGit("merge", "--no-commit", "--no-ff", "-s", "ours", remoteSHA); err != nil {
		return fmt.Errorf("git merge -s ours: %s", strings.TrimSpace(out))
	}
	if out, err := runGit("add", Paths.storageFileName); err != nil {
		runGit("merge", "--abort")
		return fmt.Errorf("git add merged: %s", strings.TrimSpace(out))
	}
	msg := buildMergeMessage(summary)
	if out, err := runGit("commit", "--message="+msg); err != nil {
		runGit("merge", "--abort")
		return fmt.Errorf("git commit merge: %s", strings.TrimSpace(out))
	}

	summary.printIfNonempty()
	return nil
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
