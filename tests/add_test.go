package tests

import "testing"

func TestAdd_UserPassSuccess(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	// Act
	r := runPsw(t, v, "add", "foo", "-u", "u", "--password=p")
	// Assert
	mustExit(t, r, 0)
	mustContain(t, r.stdout, "Username/password set successfully in foo record")
	r2 := runPsw(t, v, "get", "foo", "--exact", "--stdout")
	mustExit(t, r2, 0)
	mustEqual(t, trimmed(r2), "p")
}

func TestAdd_ValueSuccess(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	// Act
	r := runPsw(t, v, "add", "foo", "-s", "--value=v")
	// Assert
	mustExit(t, r, 0)
	mustContain(t, r.stdout, "Value set successfully in foo record")
	r2 := runPsw(t, v, "get", "foo", "--exact", "--stdout")
	mustExit(t, r2, 0)
	mustEqual(t, trimmed(r2), "v")
}

func TestAdd_GenerateLength(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	// Act
	r := runPsw(t, v, "add", "foo", "-u", "u", "-g")
	// Assert
	mustExit(t, r, 0)
	r2 := runPsw(t, v, "get", "foo", "--exact", "--stdout")
	mustExit(t, r2, 0)
	if got := trimmed(r2); len(got) != 16 {
		t.Fatalf("generated password length = %d, want 16 (got %q)", len(got), got)
	}
}

func TestAdd_ReservedMainRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	// Act
	r := runPsw(t, v, "add", "main", "-u", "u", "--password=p")
	// Assert: reserved-name path uses bare return, so exit 0.
	mustExit(t, r, 0)
	mustContain(t, r.stdout, "Name main is reserved")
}

func TestAdd_DuplicateRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "foo", "-u", "u", "--password=p")
	// Act
	r := runPsw(t, v, "add", "foo", "-u", "u", "--password=p")
	// Assert: duplicate-name path uses bare return, so exit 0.
	mustExit(t, r, 0)
	mustContain(t, r.stdout, "Record with name foo already exists")
}

func TestAdd_SingleAndGenerateRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	// Act
	r := runPsw(t, v, "add", "foo", "-s", "-g", "--value=v")
	// Assert
	mustExit(t, r, 1)
	mustContain(t, r.stdout, "Flags --single and --generate cannot be used together")
}

func TestAdd_SingleWithUsernameRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	// Act
	r := runPsw(t, v, "add", "foo", "-s", "-u", "u", "--value=v")
	// Assert
	mustExit(t, r, 1)
	mustContain(t, r.stdout, "cannot be combined with --single")
}

func TestAdd_ValueWithoutSingleRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	// Act
	r := runPsw(t, v, "add", "foo", "--value=v")
	// Assert
	mustExit(t, r, 1)
	mustContain(t, r.stdout, "Flag --value requires --single")
}

func TestAdd_PasswordWithGenerateRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	// Act
	r := runPsw(t, v, "add", "foo", "-u", "u", "--password=p", "-g")
	// Assert
	mustExit(t, r, 1)
	mustContain(t, r.stdout, "Flags --password and --generate cannot be used together")
}
