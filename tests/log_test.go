package tests

import (
	"regexp"
	"strings"
	"testing"
)

func TestLog_ShowsCommits(t *testing.T) {
	t.Parallel()
	vault, env := newGitVault(t)

	mustExit(t, runPswEnv(t, vault, env, "add", "foo", "-u", "u", "--password=p"), 0)
	mustExit(t, runPswEnv(t, vault, env, "add", "bar", "-u", "u", "--password=p"), 0)
	mustExit(t, runPswEnv(t, vault, env, "remove", "bar", "-e"), 0)
	mustExit(t, runPswEnv(t, vault, env, "change", "foo", "--password=newp", "-e"), 0)

	result := runPswEnv(t, vault, env, "log")
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "added new record")
	mustContain(t, result.stdout, "record removed")

	lines := strings.Split(strings.TrimSpace(result.stdout), "\n")
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
	mustContain(t, result.stdout, "isn't tracked by git")
}
