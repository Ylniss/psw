package tests

import (
	"strings"
	"testing"
)

func TestList_EmptyVault(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	// Act
	r := runPsw(t, v)
	// Assert
	mustExit(t, r, 0)
	mustContain(t, r.stdout, "No secrets found. Use add command first.")
}

func TestList_ShowsUserPassRecord(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "foo", "-u", "u", "--password=p")
	// Act
	r := runPsw(t, v)
	// Assert
	mustExit(t, r, 0)
	mustContain(t, r.stdout, "foo")
	mustContain(t, r.stdout, "(u)")
}

func TestList_ShowsValueOnlyRecord(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "bar", "-s", "--value=v")
	// Act
	r := runPsw(t, v)
	// Assert
	mustExit(t, r, 0)
	mustContain(t, r.stdout, "bar")
	mustContain(t, r.stdout, "<value only>")
}

func TestList_AlphabeticalOrder(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "b", "-u", "u", "--password=p")
	runPsw(t, v, "add", "a", "-u", "u", "--password=p")
	runPsw(t, v, "add", "c", "-u", "u", "--password=p")
	// Act
	r := runPsw(t, v)
	// Assert
	mustExit(t, r, 0)
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(r.stdout), "\n") {
		if line == "" {
			continue
		}
		names = append(names, strings.SplitN(line, ".", 2)[0])
	}
	if len(names) != 3 || names[0] != "a" || names[1] != "b" || names[2] != "c" {
		t.Fatalf("expected listing order [a b c], got %v\nstdout: %s", names, r.stdout)
	}
}

func TestList_OrderAfterRename(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "alpha", "-u", "u", "--password=p")
	runPsw(t, v, "add", "beta", "-u", "u", "--password=p")
	// Act
	runPsw(t, v, "change", "alpha", "--rename=zeta", "--exact")
	r := runPsw(t, v)
	// Assert
	mustExit(t, r, 0)
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(r.stdout), "\n") {
		if line == "" {
			continue
		}
		names = append(names, strings.SplitN(line, ".", 2)[0])
	}
	if len(names) != 2 || names[0] != "beta" || names[1] != "zeta" {
		t.Fatalf("expected [beta zeta] after rename, got %v\nstdout: %s", names, r.stdout)
	}
}
