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
	vault := newVault(t)
	runPsw(t, vault, "add", "alice", "-u", "u", "--password=p")
	runPsw(t, vault, "add", "bob", "-u", "u", "--password=p")
	runPsw(t, vault, "add", "carol", "-u", "u", "--password=p")

	result := runPsw(t, vault, "remove", "alice", "carol", "--exact")
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "Record alice successfully removed")
	mustContain(t, result.stdout, "Record carol successfully removed")

	gotBob := runPsw(t, vault, "get", "bob", "--exact", "--stdout")
	mustExit(t, gotBob, 0)
	mustEqual(t, trimmed(gotBob), "p")
	gotAlice := runPsw(t, vault, "get", "alice", "--exact", "--stdout")
	mustExit(t, gotAlice, 1)
	mustContain(t, gotAlice.stdout, "Record alice was not found")
	gotCarol := runPsw(t, vault, "get", "carol", "--exact", "--stdout")
	mustExit(t, gotCarol, 1)
	mustContain(t, gotCarol.stdout, "Record carol was not found")
}

func TestRemove_MultiExactOneMissing(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	runPsw(t, vault, "add", "alice", "-u", "u", "--password=p")

	result := runPsw(t, vault, "remove", "alice", "missing", "--exact")
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "Records not found: missing")

	got := runPsw(t, vault, "get", "alice", "--exact", "--stdout")
	mustExit(t, got, 0)
	mustEqual(t, trimmed(got), "p")
}

func TestRemove_MultiExactSeveralMissing(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	runPsw(t, vault, "add", "alice", "-u", "u", "--password=p")

	result := runPsw(t, vault, "remove", "alice", "ghost", "phantom", "--exact")
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "Records not found: ghost, phantom")

	got := runPsw(t, vault, "get", "alice", "--exact", "--stdout")
	mustExit(t, got, 0)
	mustEqual(t, trimmed(got), "p")
}

func TestRemove_MultiArgsRequireExact(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	runPsw(t, vault, "add", "alice", "-u", "u", "--password=p")
	runPsw(t, vault, "add", "bob", "-u", "u", "--password=p")

	result := runPsw(t, vault, "remove", "alice", "bob")
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "multiple names require --exact")

	gotA := runPsw(t, vault, "get", "alice", "--exact", "--stdout")
	mustExit(t, gotA, 0)
	mustEqual(t, trimmed(gotA), "p")
	gotB := runPsw(t, vault, "get", "bob", "--exact", "--stdout")
	mustExit(t, gotB, 0)
	mustEqual(t, trimmed(gotB), "p")
}
