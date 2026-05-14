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

const defaultMainPassword = "testpass"

type pswResult struct {
	stdout string
	stderr string
	code   int
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// Fresh PSW_HOME with seeded pswcfg.toml.
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
	return dir
}

// assertVaultHasRecords fails if any name does not resolve via `psw get --exact`.
func assertVaultHasRecords(t *testing.T, vault string, env map[string]string, names ...string) {
	t.Helper()
	for _, name := range names {
		r := runPswEnv(t, vault, env, "get", name, "--exact", "--stdout")
		if r.code != 0 {
			t.Fatalf("expected record %q present; exit=%d\nstdout: %s\nstderr: %s", name, r.code, r.stdout, r.stderr)
		}
	}
}

// assertVaultLacksRecords fails if any name resolves via `psw get --exact`.
func assertVaultLacksRecords(t *testing.T, vault string, env map[string]string, names ...string) {
	t.Helper()
	for _, name := range names {
		r := runPswEnv(t, vault, env, "get", name, "--exact", "--stdout")
		if r.code != 1 {
			t.Fatalf("expected record %q absent; exit=%d\nstdout: %s\nstderr: %s", name, r.code, r.stdout, r.stderr)
		}
	}
}

func runPsw(t *testing.T, vault string, args ...string) pswResult {
	return runPswEnv(t, vault, nil, args...)
}

// extraEnv keys override the helper defaults.
// Allow-list (not os.Environ()) prevents stray PSW_* leaking into subprocesses.
func runPswEnv(t *testing.T, vault string, extraEnv map[string]string, args ...string) pswResult {
	t.Helper()
	cmd := exec.Command(pswBinary, args...)
	cmd.Env = flattenEnv(t, vault, extraEnv)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
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

// flattenEnv builds the env slice for a spawned psw subprocess.
// HOME/USERPROFILE/XDG_CONFIG_HOME point at an empty t.TempDir() so go-git's
// global-config lookup (which honors none of GIT_CONFIG_GLOBAL/SYSTEM/NOSYSTEM)
// and shell-git's global config both come up empty; GIT_CONFIG_NOSYSTEM=1
// disables shell-git's system config (go-git's system scope is hard-coded to
// /etc/gitconfig). extra keys override defaults.
func flattenEnv(t *testing.T, vault string, extra map[string]string) []string {
	t.Helper()
	iso := t.TempDir()
	env := map[string]string{
		"PSW_HOME":            vault,
		"PSW_MAIN_PASSWORD":   defaultMainPassword,
		"PSW_GIT":             "0",
		"PSW_FAST_ARGON":      "1",
		"PATH":                os.Getenv("PATH"),
		"HOME":                iso,
		"USERPROFILE":         iso,
		"XDG_CONFIG_HOME":     iso,
		"GIT_CONFIG_NOSYSTEM": "1",
	}
	for k, v := range extra {
		env[k] = v
	}
	flat := make([]string, 0, len(env))
	for k, v := range env {
		flat = append(flat, k+"="+v)
	}
	return flat
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

func mustNotContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("expected to NOT contain %q\ngot: %s", needle, haystack)
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
