package tests

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const defaultMainPass = "testpass"

type pswResult struct {
	stdout string
	stderr string
	code   int
}

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// Fresh PSW_HOME with seeded pswcfg.toml; pre-runs psw to swallow the first-run banner.
func newVault(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join("testdata", "pswcfg.toml")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read testdata pswcfg.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pswcfg.toml"), data, 0644); err != nil {
		t.Fatalf("write vault pswcfg.toml: %v", err)
	}
	runPsw(t, dir)
	return dir
}

func runPsw(t *testing.T, vault string, args ...string) pswResult {
	return runPswEnv(t, vault, nil, args...)
}

// extraEnv keys override the helper defaults.
// Allow-list (not os.Environ()) prevents stray PSW_* leaking into subprocesses.
func runPswEnv(t *testing.T, vault string, extraEnv map[string]string, args ...string) pswResult {
	t.Helper()
	env := map[string]string{
		"PSW_HOME":          vault,
		"PSW_MAIN_PASSWORD": defaultMainPass,
		"PSW_GIT":           "0",
		"PATH":              os.Getenv("PATH"),
		"HOME":              os.Getenv("HOME"),
	}
	for k, v := range extraEnv {
		env[k] = v
	}
	flat := make([]string, 0, len(env))
	for k, v := range env {
		flat = append(flat, k+"="+v)
	}

	cmd := exec.Command(pswBin, args...)
	cmd.Env = flat
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			t.Fatalf("running psw %v: %v", args, err)
		}
	}
	return pswResult{
		stdout: stripANSI(stdout.String()),
		stderr: stripANSI(stderr.String()),
		code:   code,
	}
}

func mustExit(t *testing.T, r pswResult, code int) {
	t.Helper()
	if r.code != code {
		t.Fatalf("exit code = %d, want %d\nstdout: %s\nstderr: %s", r.code, code, r.stdout, r.stderr)
	}
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected to contain %q\ngot: %s", needle, haystack)
	}
}

func mustEqual(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func trimmed(r pswResult) string {
	return strings.TrimRight(r.stdout, "\n\r \t")
}
