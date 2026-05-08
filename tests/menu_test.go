package tests

import (
	"testing"
)

func TestMenu_NonTTYErrors(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	result := runPsw(t, vault, "menu")
	mustExit(t, result, 1)
	mustContain(t, result.stdout+result.stderr, "requires an interactive terminal")
}

func TestMenu_HelpListsMenu(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	result := runPsw(t, vault, "--help")
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "menu")
}
