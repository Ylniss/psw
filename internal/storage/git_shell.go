package storage

import (
	"context"
	"errors"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing/transport"
)

// gitNetworkTimeout caps fetch/push to prevent indefinite hangs.
const gitNetworkTimeout = 5 * time.Second

// runGit and runGitNetwork are the shell-git fallback path for auth/signing
// cases go-git can't handle.

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

// shouldFallbackToShell is true for auth/signing failures we can recover from
// via shell git (our pre-call sentinel, go-git transport sentinels, or the SSH
// "no supported methods" handshake error).
func shouldFallbackToShell(err error) bool {
	if errors.Is(err, ErrShellGitNeeded) {
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
