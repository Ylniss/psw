package tests

import "testing"

func TestGet_UserPassStdout(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "foo", "-u", "u", "--password=secret")
	// Act
	result := runPsw(t, vault, "get", "foo", "--exact", "--stdout")
	// Assert
	mustExit(t, result, 0)
	mustEqual(t, trimmed(result), "secret")
}

func TestGet_ValueStdout(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "foo", "-s", "--value=v")
	// Act
	result := runPsw(t, vault, "get", "foo", "--exact", "--stdout")
	// Assert
	mustExit(t, result, 0)
	mustEqual(t, trimmed(result), "v")
}

func TestGet_MissingRecord(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	// Act
	result := runPsw(t, vault, "get", "nope", "--exact", "--stdout")
	// Assert
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "Record nope was not found")
}

func TestGet_ExactRequiresName(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	// Act
	result := runPsw(t, vault, "get", "--exact")
	// Assert
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "--exact needs a record name")
}

func TestGet_WrongMainPassword(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "foo", "-u", "u", "--password=p")
	// Act
	result := runPswEnv(t, vault, map[string]string{"PSW_MAIN_PASSWORD": "wrongpass"},
		"get", "foo", "--exact", "--stdout")
	// Assert
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "Wrong password")
}

func TestGet_CaseInsensitiveLookup(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "Foo", "-u", "u", "--password=secret")
	// Act
	result := runPsw(t, vault, "get", "FOO", "--exact", "--stdout")
	// Assert
	mustExit(t, result, 0)
	mustEqual(t, trimmed(result), "secret")
}
