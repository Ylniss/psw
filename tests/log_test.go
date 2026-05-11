package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func newGitVault(t *testing.T) (string, map[string]string) {
	t.Helper()
	env := map[string]string{
		"PSW_GIT":             "",
		"PSW_GIT_REMOTE":      "0",
		"GIT_AUTHOR_NAME":     "psw-tests",
		"GIT_AUTHOR_EMAIL":    "psw@tests.local",
		"GIT_COMMITTER_NAME":  "psw-tests",
		"GIT_COMMITTER_EMAIL": "psw@tests.local",
	}
	dir := t.TempDir()
	src := filepath.Join("testdata", "pswcfg.toml")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read testdata pswcfg.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pswcfg.toml"), data, 0644); err != nil {
		t.Fatalf("write vault pswcfg.toml: %v", err)
	}
	runPswEnv(t, dir, env)
	return dir, env
}

func TestLog_ShowsCommits(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	vault, env := newGitVault(t)

	mustExit(t, runPswEnv(t, vault, env, "add", "foo", "-u", "u", "--password=p"), 0)
	mustExit(t, runPswEnv(t, vault, env, "add", "bar", "-u", "u", "--password=p"), 0)
	mustExit(t, runPswEnv(t, vault, env, "remove", "bar", "-e"), 0)
	mustExit(t, runPswEnv(t, vault, env, "change", "foo", "--password=newp", "-e"), 0)

	result := runPswEnv(t, vault, env, "log")
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "added new record")
	mustContain(t, result.stdout, "record removed")

	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(result.stdout), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) != 5 {
		t.Fatalf("expected 5 log lines, got %d:\n%s", len(lines), result.stdout)
	}

	lineRegex := regexp.MustCompile(`^[0-9a-f]{7,}\s+\d{4}-\d{2}-\d{2} \d{2}:\d{2}\s+\S`)
	for i, l := range lines {
		if !lineRegex.MatchString(l) {
			t.Fatalf("line %d doesn't match expected format: %q", i, l)
		}
	}

	// --reverse: oldest first.
	if !strings.Contains(lines[0], "initial main password set") {
		t.Fatalf("expected oldest commit on first line, got: %q", lines[0])
	}
	if !strings.Contains(lines[len(lines)-1], "record updated") {
		t.Fatalf("expected newest commit on last line, got: %q", lines[len(lines)-1])
	}
}

func TestLog_NoGitRepo(t *testing.T) {
	t.Parallel()
	vault := newVault(t) // PSW_GIT=0, no .git/
	result := runPsw(t, vault, "log")
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "Storage is not a git repository")
}
