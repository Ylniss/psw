package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// assertConfigContains fails if pswcfg.toml in vault doesn't contain needle.
func assertConfigContains(t *testing.T, vault, needle string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(vault, "pswcfg.toml"))
	if err != nil {
		t.Fatalf("read pswcfg: %v", err)
	}
	if !strings.Contains(string(data), needle) {
		t.Fatalf("expected pswcfg.toml to contain %q\ngot:\n%s", needle, string(data))
	}
}

func TestConfig_HelpListsKeysWithDescriptions(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	r := runPsw(t, vault, "config", "-h")
	mustExit(t, r, 0)
	mustContain(t, r.stdout, "Configurable keys:")
	mustContain(t, r.stdout, "clipboard_timeout (int)")
	mustContain(t, r.stdout, "allow_repeat (bool)")
	mustContain(t, r.stdout, "Total length of auto-generated passwords")
}

func TestConfig_BarePrintsPath(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	r := runPsw(t, vault, "config")
	mustExit(t, r, 0)
	expected := filepath.Join(vault, "pswcfg.toml")
	mustContain(t, r.stdout, expected)
}

func TestConfig_SetIntKey(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	mustExit(t, runPsw(t, vault, "config", "set", "min_digits", "8"), 0)
	assertConfigContains(t, vault, "min_digits = 8")
}

func TestConfig_SetBoolKey(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	mustExit(t, runPsw(t, vault, "config", "set", "allow_repeat", "false"), 0)
	assertConfigContains(t, vault, "allow_repeat = false")
}

func TestConfig_SetStringKey(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	mustExit(t, runPsw(t, vault, "config", "set", "remote", "git@example.com:foo.git"), 0)
	assertConfigContains(t, vault, `remote = 'git@example.com:foo.git'`)
}

func TestConfig_SetClipboardTimeout(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	mustExit(t, runPsw(t, vault, "config", "set", "clipboard_timeout", "45"), 0)
	assertConfigContains(t, vault, "clipboard_timeout = 45")
}

func TestConfig_SetUnknownKeyExitsOne(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	r := runPsw(t, vault, "config", "set", "no_such_key", "x")
	mustExit(t, r, 1)
	mustContain(t, r.stdout, "unknown config key: no_such_key")
	mustContain(t, r.stdout, "min_digits")
}

func TestConfig_SetTypeMismatchExitsOne(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	r := runPsw(t, vault, "config", "set", "min_digits", "abc")
	mustExit(t, r, 1)
	mustContain(t, r.stdout, `invalid int for "min_digits"`)
}

func TestConfig_SetNegativeIntExitsOne(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	// `--` so cobra treats "-3" as a positional value, not a short flag.
	r := runPsw(t, vault, "config", "set", "--", "min_digits", "-3")
	mustExit(t, r, 1)
	mustContain(t, r.stdout, `value for "min_digits" must be >= 0`)
}

func TestConfig_SetBoolTypeMismatchExitsOne(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	r := runPsw(t, vault, "config", "set", "allow_repeat", "maybe")
	mustExit(t, r, 1)
	mustContain(t, r.stdout, `invalid bool for "allow_repeat"`)
}

func TestConfig_SetWrongArityExitsOne(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	r := runPsw(t, vault, "config", "set", "min_digits")
	mustExit(t, r, 1)
}

func TestConfig_ResetRestoresTemplate(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	mustExit(t, runPsw(t, vault, "config", "set", "min_digits", "99"), 0)
	assertConfigContains(t, vault, "min_digits = 99")

	mustExit(t, runPsw(t, vault, "config", "reset"), 0)
	data, err := os.ReadFile(filepath.Join(vault, "pswcfg.toml"))
	if err != nil {
		t.Fatalf("read pswcfg after reset: %v", err)
	}
	if strings.Contains(string(data), "min_digits = 99") {
		t.Fatalf("reset did not clear min_digits=99:\n%s", string(data))
	}
}

func TestConfig_SetCommitsWhenGitOn(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	vault, env := newGitVault(t)
	// init git repo via an add first (config alone doesn't init the repo).
	mustExit(t, runPswEnv(t, vault, env, "add", "alpha", "-u", "u", "--password=p"), 0)
	mustExit(t, runPswEnv(t, vault, env, "config", "set", "min_digits", "10"), 0)

	logRes := runPswEnv(t, vault, env, "log")
	mustExit(t, logRes, 0)
	mustContain(t, logRes.stdout, "config updated")
}

func TestConfig_ResetCommitsWhenGitOn(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	vault, env := newGitVault(t)
	mustExit(t, runPswEnv(t, vault, env, "add", "alpha", "-u", "u", "--password=p"), 0)
	mustExit(t, runPswEnv(t, vault, env, "config", "reset"), 0)

	logRes := runPswEnv(t, vault, env, "log")
	mustExit(t, logRes, 0)
	mustContain(t, logRes.stdout, "config reset")
}

func TestConfig_PSWGitZeroSkipsCommit(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	vault, env := newGitVault(t)
	mustExit(t, runPswEnv(t, vault, env, "add", "alpha", "-u", "u", "--password=p"), 0)
	before := runPswEnv(t, vault, env, "log")
	mustExit(t, before, 0)
	beforeCount := strings.Count(before.stdout, "\n")

	env2 := withEnv(env, "PSW_GIT", "0")
	mustExit(t, runPswEnv(t, vault, env2, "config", "set", "min_digits", "7"), 0)

	after := runPswEnv(t, vault, env, "log")
	mustExit(t, after, 0)
	afterCount := strings.Count(after.stdout, "\n")
	if afterCount != beforeCount {
		t.Fatalf("expected no new commits with PSW_GIT=0; before=%d after=%d\n%s", beforeCount, afterCount, after.stdout)
	}
}
