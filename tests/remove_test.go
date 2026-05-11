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

	list := runPsw(t, vault)
	mustExit(t, list, 0)
	mustContain(t, list.stdout, "bob")
	mustNotContain(t, list.stdout, "alice")
	mustNotContain(t, list.stdout, "carol")
}

func TestRemove_MultiExactOneMissing(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	runPsw(t, vault, "add", "alice", "-u", "u", "--password=p")

	result := runPsw(t, vault, "remove", "alice", "missing", "--exact")
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "Records not found: missing")

	list := runPsw(t, vault)
	mustExit(t, list, 0)
	mustContain(t, list.stdout, "alice")
}

func TestRemove_MultiExactSeveralMissing(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	runPsw(t, vault, "add", "alice", "-u", "u", "--password=p")

	result := runPsw(t, vault, "remove", "alice", "ghost", "phantom", "--exact")
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "Records not found: ghost, phantom")

	list := runPsw(t, vault)
	mustExit(t, list, 0)
	mustContain(t, list.stdout, "alice")
}

func TestRemove_MultiArgsRequireExact(t *testing.T) {
	t.Parallel()
	vault := newVault(t)
	runPsw(t, vault, "add", "alice", "-u", "u", "--password=p")
	runPsw(t, vault, "add", "bob", "-u", "u", "--password=p")

	result := runPsw(t, vault, "remove", "alice", "bob")
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "multiple names require --exact")

	list := runPsw(t, vault)
	mustExit(t, list, 0)
	mustContain(t, list.stdout, "alice")
	mustContain(t, list.stdout, "bob")
}
