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

// Env-password warning is TTY-gated. go test stdin is not a TTY, so the
// warning stays silent even when PSW_MAIN_PASSWORD is set in the env.
func TestRoot_EnvPasswordWarning_SuppressedNonTTY(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	result := runPsw(t, vault, "add", "foo", "-s", "--value=v")
	mustExit(t, result, 0)
	mustNotContain(t, result.stderr, "PSW_MAIN_PASSWORD")
	mustNotContain(t, result.stdout, "PSW_MAIN_PASSWORD")
}
