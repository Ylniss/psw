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
