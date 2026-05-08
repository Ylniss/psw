package tests

import "testing"

func TestRemove_Success(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "foo", "-u", "u", "--password=p")
	// Act
	result := runPsw(t, vault, "remove", "foo", "--exact")
	// Assert
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "Record foo successfully removed")
	result2 := runPsw(t, vault)
	mustExit(t, result2, 0)
	mustContain(t, result2.stdout, "No secrets found")
}

func TestRemove_MissingRecord(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	// Act
	result := runPsw(t, vault, "remove", "nope", "--exact")
	// Assert
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "Record nope was not found")
}
