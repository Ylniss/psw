package tests

import "testing"

// Each call is its own process; only the on-disk vault carries state.
func TestPersistence_AcrossInvocations(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "a", "-u", "u", "--password=p1")
	runPsw(t, vault, "add", "b", "-s", "--value=v2")
	runPsw(t, vault, "change", "a", "--password=p1new", "--exact")
	// Act
	result := runPsw(t, vault, "get", "a", "--exact", "--stdout")
	result2 := runPsw(t, vault, "get", "b", "--exact", "--stdout")
	// Assert
	mustExit(t, result, 0)
	mustEqual(t, trimmed(result), "p1new")
	mustExit(t, result2, 0)
	mustEqual(t, trimmed(result2), "v2")
}
