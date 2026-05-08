package tests

import (
	"strings"
	"testing"
)

// Without PSW_MAIN_PASSWORD and without a TTY (exec.Cmd stdin = /dev/null),
// the prompt's non-TTY pre-check must short-circuit and produce a clear
// error rather than hanging.
func TestPrompt_NonTTYBailsOut(t *testing.T) {
	vault := newVault(t)
	result := runPswEnv(t, vault, map[string]string{"PSW_MAIN_PASSWORD": ""}, "add", "foo")
	mustExit(t, result, 0)
	combined := result.stdout + result.stderr
	if !strings.Contains(combined, "interactive prompt required") {
		t.Fatalf("expected output to contain 'interactive prompt required'\nstdout: %s\nstderr: %s", result.stdout, result.stderr)
	}
}
