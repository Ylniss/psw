package tests

import (
	"strings"
	"testing"
)

func TestList_EmptyVault(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	// Act
	result := runPsw(t, vault)
	// Assert
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "No secrets found. Use add command first.")
}

func TestList_ShowsUserPassRecord(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "foo", "-u", "u", "--password=p")
	// Act
	result := runPsw(t, vault)
	// Assert
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "foo")
	mustContain(t, result.stdout, "(u)")
}

func TestList_ShowsValueOnlyRecord(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "bar", "-s", "--value=v")
	// Act
	result := runPsw(t, vault)
	// Assert
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "bar")
	mustContain(t, result.stdout, "<value only>")
}

func TestList_AlphabeticalOrder(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "b", "-u", "u", "--password=p")
	runPsw(t, vault, "add", "a", "-u", "u", "--password=p")
	runPsw(t, vault, "add", "c", "-u", "u", "--password=p")
	// Act
	result := runPsw(t, vault)
	// Assert
	mustExit(t, result, 0)
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(result.stdout), "\n") {
		if line == "" {
			continue
		}
		names = append(names, strings.SplitN(line, ".", 2)[0])
	}
	if len(names) != 3 || names[0] != "a" || names[1] != "b" || names[2] != "c" {
		t.Fatalf("expected listing order [a b c], got %v\nstdout: %s", names, result.stdout)
	}
}

func TestList_OrderAfterRename(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "alpha", "-u", "u", "--password=p")
	runPsw(t, vault, "add", "beta", "-u", "u", "--password=p")
	// Act
	runPsw(t, vault, "change", "alpha", "--rename=zeta", "--exact")
	result := runPsw(t, vault)
	// Assert
	mustExit(t, result, 0)
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(result.stdout), "\n") {
		if line == "" {
			continue
		}
		names = append(names, strings.SplitN(line, ".", 2)[0])
	}
	if len(names) != 2 || names[0] != "beta" || names[1] != "zeta" {
		t.Fatalf("expected [beta zeta] after rename, got %v\nstdout: %s", names, result.stdout)
	}
}
