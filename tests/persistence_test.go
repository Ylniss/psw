package tests

import "testing"

// Each call is its own process; only the on-disk vault carries state.
func TestPersistence_AcrossInvocations(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "a", "-u", "u", "--password=p1")
	runPsw(t, v, "add", "b", "-s", "--value=v2")
	runPsw(t, v, "change", "a", "--password=p1new", "--exact")
	// Act
	r := runPsw(t, v, "get", "a", "--exact", "--stdout")
	r2 := runPsw(t, v, "get", "b", "--exact", "--stdout")
	// Assert
	mustExit(t, r, 0)
	mustEqual(t, trimmed(r), "p1new")
	mustExit(t, r2, 0)
	mustEqual(t, trimmed(r2), "v2")
}
