package tests

import (
	"testing"
)

func TestRoot_NonTTYErrors(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	result := runPsw(t, vault)
	mustExit(t, result, 1)
	mustContain(t, result.stdout+result.stderr, "requires an interactive terminal")
}

// L8: the env-password warning is gated on an interactive TTY. The test
// harness inherits stdin from `go test`, which is not a TTY, so the warning
// should never fire here even though PSW_MAIN_PASSWORD is set in the env.
func TestRoot_EnvPasswordWarning_SuppressedNonTTY(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	result := runPsw(t, vault, "add", "foo", "-s", "--value=v")
	mustExit(t, result, 0)
	mustNotContain(t, result.stderr, "PSW_MAIN_PASSWORD")
	mustNotContain(t, result.stdout, "PSW_MAIN_PASSWORD")
}
