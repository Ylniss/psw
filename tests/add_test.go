package tests

import "testing"

func TestAdd_UserPassSuccess(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	// Act
	result := runPsw(t, vault, "add", "foo", "-u", "u", "--password=p")
	// Assert
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "Username/password set successfully in foo record")
	result2 := runPsw(t, vault, "get", "foo", "--exact", "--stdout")
	mustExit(t, result2, 0)
	mustEqual(t, trimmed(result2), "p")
}

func TestAdd_ValueSuccess(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	// Act
	result := runPsw(t, vault, "add", "foo", "-s", "--value=v")
	// Assert
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "Value set successfully in foo record")
	result2 := runPsw(t, vault, "get", "foo", "--exact", "--stdout")
	mustExit(t, result2, 0)
	mustEqual(t, trimmed(result2), "v")
}

func TestAdd_GenerateLength(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	// Act
	result := runPsw(t, vault, "add", "foo", "-u", "u", "-g")
	// Assert
	mustExit(t, result, 0)
	result2 := runPsw(t, vault, "get", "foo", "--exact", "--stdout")
	mustExit(t, result2, 0)
	if got := trimmed(result2); len(got) != 16 {
		t.Fatalf("generated password length = %d, want 16 (got %q)", len(got), got)
	}
}

func TestAdd_ReservedMainRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	// Act
	result := runPsw(t, vault, "add", "main", "-u", "u", "--password=p")
	// Assert: reserved-name path uses bare return, so exit 0.
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "Name main is reserved")
}

func TestAdd_ReservedMainPasswordRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	// Act
	result := runPsw(t, vault, "add", "main-password", "-u", "u", "--password=p")
	// Assert
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "Name main-password is reserved")
}

func TestAdd_DuplicateRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "foo", "-u", "u", "--password=p")
	// Act
	result := runPsw(t, vault, "add", "foo", "-u", "u", "--password=p")
	// Assert: duplicate-name path uses bare return, so exit 0.
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "Record with name foo already exists")
}

func TestAdd_SingleAndGenerateRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	// Act
	result := runPsw(t, vault, "add", "foo", "-s", "-g", "--value=v")
	// Assert
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "Flags --single and --generate cannot be used together")
}

func TestAdd_SingleWithUsernameRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	// Act
	result := runPsw(t, vault, "add", "foo", "-s", "-u", "u", "--value=v")
	// Assert
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "cannot be combined with --single")
}

func TestAdd_ValueWithoutSingleRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	// Act
	result := runPsw(t, vault, "add", "foo", "--value=v")
	// Assert
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "Flag --value requires --single")
}

func TestAdd_PasswordWithGenerateRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	// Act
	result := runPsw(t, vault, "add", "foo", "-u", "u", "--password=p", "-g")
	// Assert
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "Flags --password and --generate cannot be used together")
}
