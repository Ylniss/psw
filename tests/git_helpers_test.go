package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// gitTestEnv returns env vars enabling git commits + remote sync.
func gitTestEnv() map[string]string {
	return map[string]string{
		"PSW_GIT":             "",
		"GIT_AUTHOR_NAME":     "psw-tests",
		"GIT_AUTHOR_EMAIL":    "psw@tests.local",
		"GIT_COMMITTER_NAME":  "psw-tests",
		"GIT_COMMITTER_EMAIL": "psw@tests.local",
	}
}

// newBareRemote creates a bare git repo in a temp dir and returns its path.
// Uses --initial-branch=main to match the vault's init.
func newBareRemote(t *testing.T) string {
	t.Helper()
	bare := t.TempDir()
	if out, err := exec.Command("git", "init", "--bare", "--initial-branch=main", bare).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	return bare
}

// newGitVaultWithRemote creates a vault wired to a fresh bare repo and runs
// psw once to trigger first-time init. Returns vault, bare path, and env.
func newGitVaultWithRemote(t *testing.T) (vault, bare string, env map[string]string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	bare = newBareRemote(t)
	vault = t.TempDir()
	cfg := fmt.Sprintf("clipboard_timeout = 20\nremote = %q\n", bare)
	if err := os.WriteFile(filepath.Join(vault, "pswcfg.toml"), []byte(cfg), 0644); err != nil {
		t.Fatalf("write pswcfg: %v", err)
	}
	env = gitTestEnv()
	// Trigger first-time init via a known-missing record (exit 1 ignored).
	runPswEnv(t, vault, env, "get", "__init__", "--exact")
	return vault, bare, env
}

// addPeer clones bare into a new vault. Shares history so merges find a
// common ancestor.
func addPeer(t *testing.T, bare string) (vault string, env map[string]string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	vault = t.TempDir()
	if out, err := exec.Command("git", "clone", bare, vault).CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}
	env = gitTestEnv()
	return vault, env
}

// runGitInBare runs git -C bare <args> and returns trimmed output; failures fatal.
func runGitInBare(t *testing.T, bare string, args ...string) string {
	t.Helper()
	gitArgs := append([]string{"-C", bare}, args...)
	out, err := exec.Command("git", gitArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// bareCommitCount returns the number of commits reachable from main in bare.
func bareCommitCount(t *testing.T, bare string) int {
	t.Helper()
	out, err := exec.Command("git", "-C", bare, "rev-list", "--count", "main").CombinedOutput()
	if err != nil {
		// no main branch yet
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n
}
