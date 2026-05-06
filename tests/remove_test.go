package tests

import "testing"

func TestRemove_Success(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "foo", "-u", "u", "--password=p")
	// Act
	r := runPsw(t, v, "remove", "foo", "--exact")
	// Assert
	mustExit(t, r, 0)
	mustContain(t, r.stdout, "Record foo successfully removed")
	r2 := runPsw(t, v)
	mustExit(t, r2, 0)
	mustContain(t, r2.stdout, "No secrets found")
}

func TestRemove_MissingRecord(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	// Act
	r := runPsw(t, v, "remove", "nope", "--exact")
	// Assert
	mustExit(t, r, 1)
	mustContain(t, r.stdout, "Record nope was not found")
}
