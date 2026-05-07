package tests

import "testing"

func TestGet_UserPassStdout(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "foo", "-u", "u", "--password=secret")
	// Act
	r := runPsw(t, v, "get", "foo", "--exact", "--stdout")
	// Assert
	mustExit(t, r, 0)
	mustEqual(t, trimmed(r), "secret")
}

func TestGet_ValueStdout(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "foo", "-s", "--value=v")
	// Act
	r := runPsw(t, v, "get", "foo", "--exact", "--stdout")
	// Assert
	mustExit(t, r, 0)
	mustEqual(t, trimmed(r), "v")
}

func TestGet_MissingRecord(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	// Act
	r := runPsw(t, v, "get", "nope", "--exact", "--stdout")
	// Assert
	mustExit(t, r, 1)
	mustContain(t, r.stdout, "Record nope was not found")
}

func TestGet_ExactRequiresName(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	// Act
	r := runPsw(t, v, "get", "--exact")
	// Assert
	mustExit(t, r, 1)
	mustContain(t, r.stdout, "--exact requires a record name argument")
}

func TestGet_WrongMainPassword(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "foo", "-u", "u", "--password=p")
	// Act
	r := runPswEnv(t, v, map[string]string{"PSW_MAIN_PASSWORD": "wrongpass"},
		"get", "foo", "--exact", "--stdout")
	// Assert
	mustExit(t, r, 0)
	mustContain(t, r.stdout, "Wrong password.")
}

func TestGet_CaseInsensitiveLookup(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "Foo", "-u", "u", "--password=secret")
	// Act
	r := runPsw(t, v, "get", "FOO", "--exact", "--stdout")
	// Assert
	mustExit(t, r, 0)
	mustEqual(t, trimmed(r), "secret")
}
