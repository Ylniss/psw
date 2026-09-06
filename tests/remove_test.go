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
	mustContain(t, result.stdout, "Removed foo")
	result2 := runPsw(t, vault, "get", "foo", "--exact", "--stdout")
	mustExit(t, result2, 1)
	mustContain(t, result2.stdout, "Record foo was not found")
}

func TestRemove_MissingRecord(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	// Act
	result := runPsw(t, vault, "remove", "nope", "--exact")
	// Assert
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "Records not found: nope")
}

func TestRemove_MultiExact(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "alice", "-u", "u", "--password=p")
	runPsw(t, vault, "add", "bob", "-u", "u", "--password=p")
	runPsw(t, vault, "add", "carol", "-u", "u", "--password=p")

	// Act
	result := runPsw(t, vault, "remove", "alice", "carol", "--exact")
	// Assert
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "Removed alice")
	mustContain(t, result.stdout, "Removed carol")

	gotBob := runPsw(t, vault, "get", "bob", "--exact", "--stdout")
	mustExit(t, gotBob, 0)
	mustEqual(t, trimmed(gotBob), "p")
	assertVaultLacksRecords(t, vault, nil, "alice", "carol")
}

func TestRemove_MultiExactOneMissing(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "alice", "-u", "u", "--password=p")

	// Act
	result := runPsw(t, vault, "remove", "alice", "missing", "--exact")
	// Assert
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "Records not found: missing")

	got := runPsw(t, vault, "get", "alice", "--exact", "--stdout")
	mustExit(t, got, 0)
	mustEqual(t, trimmed(got), "p")
}

func TestRemove_MultiExactSeveralMissing(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "alice", "-u", "u", "--password=p")

	// Act
	result := runPsw(t, vault, "remove", "alice", "ghost", "phantom", "--exact")
	// Assert
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "Records not found: ghost, phantom")

	got := runPsw(t, vault, "get", "alice", "--exact", "--stdout")
	mustExit(t, got, 0)
	mustEqual(t, trimmed(got), "p")
}

func TestRemove_MultiArgsRequireExact(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "alice", "-u", "u", "--password=p")
	runPsw(t, vault, "add", "bob", "-u", "u", "--password=p")

	// Act
	result := runPsw(t, vault, "remove", "alice", "bob")
	// Assert
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "passing multiple names requires --exact")

	gotA := runPsw(t, vault, "get", "alice", "--exact", "--stdout")
	mustExit(t, gotA, 0)
	mustEqual(t, trimmed(gotA), "p")
	gotB := runPsw(t, vault, "get", "bob", "--exact", "--stdout")
	mustExit(t, gotB, 0)
	mustEqual(t, trimmed(gotB), "p")
}
