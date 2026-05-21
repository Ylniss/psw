package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// pathWithoutGit returns PATH with directories containing `git`/`git.exe` removed.
// Simulates "git not installed".
func pathWithoutGit(t *testing.T) string {
	t.Helper()
	sep := string(os.PathListSeparator)
	var kept []string
	for dir := range strings.SplitSeq(os.Getenv("PATH"), sep) {
		if dir == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "git")); err == nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "git.exe")); err == nil {
			continue
		}
		kept = append(kept, dir)
	}
	return strings.Join(kept, sep)
}

// With git stripped from PATH and no remote configured, init/add/commit are
// pure go-git. (file:// transport still needs git binary; HTTPS/SSH paths
// are pure-Go and verified by manual smoke.)
func TestSync_GitNotOnPath_LocalOnly(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pswcfg.toml"), []byte("clipboard_timeout = 20\n"), 0644); err != nil {
		t.Fatal(err)
	}
	env := gitTestEnv()
	env["PATH"] = pathWithoutGit(t)

	// First mutating command triggers init via go-git; git binary not needed.
	mustExit(t, runPswEnv(t, dir, env, "add", "foo", "-u", "u", "--password=p"), 0)

	if _, err := os.Stat(filepath.Join(dir, ".git", "HEAD")); err != nil {
		t.Fatalf("expected .git/HEAD via go-git init: %v", err)
	}
	res := runPswEnv(t, dir, env, "log")
	mustExit(t, res, 0)
	mustContain(t, res.stdout, "added new record")
}

// TestSync_GPGSignFallback_GitNotOnPath: commit.gpgsign=true with git not on
// PATH triggers the typed "signing requires git" warning; data is still saved.
func TestSync_GPGSignFallback_GitNotOnPath(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	vault, _, env := newGitVaultWithRemote(t)
	addToGitConfig(t, vault, "[commit]\n\tgpgsign = true\n")

	noGitEnv := withEnv(env, "PATH", pathWithoutGit(t))
	res := runPswEnv(t, vault, noGitEnv, "add", "foo", "-u", "u", "--password=p")
	mustExit(t, res, 0)
	mustContain(t, res.stderr, "commit signing requires git on PATH")

	// Record is still saved (data path independent of commit path).
	assertVaultHasRecords(t, vault, noGitEnv, "foo")
}

// TestSync_GPGSignFallback_GitOnPath: commit.gpgsign=true with git on PATH
// dispatches to shell `git commit`. We point gpg.program at a fake script
// that emits a signature plus the [GNUPG:] SIG_CREATED status line git
// requires; git accepts it and the commit/push proceed.
func TestSync_GPGSignFallback_GitOnPath(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	vault, bare, env := newGitVaultWithRemote(t)

	addToGitConfig(t, vault, "[commit]\n\tgpgsign = true\n[gpg]\n\tprogram = "+filepath.ToSlash(fakeGpgBinaryPath)+"\n[user]\n\tsigningkey = test\n")

	beforeCount := bareCommitCount(t, bare)
	res := runPswEnv(t, vault, env, "add", "foo", "-u", "u", "--password=p")
	mustExit(t, res, 0)
	if got := bareCommitCount(t, bare); got <= beforeCount {
		t.Fatalf("expected commit count to advance via shell-out fallback; before=%d after=%d\nstderr: %s",
			beforeCount, got, res.stderr)
	}
}

// addToGitConfig appends raw INI text to the vault's .git/config.
func addToGitConfig(t *testing.T, vault, addition string) {
	t.Helper()
	cfgPath := filepath.Join(vault, ".git", "config")
	cfgBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read .git/config: %v", err)
	}
	if err := os.WriteFile(cfgPath, append(cfgBytes, []byte(addition)...), 0644); err != nil {
		t.Fatalf("write .git/config: %v", err)
	}
}
