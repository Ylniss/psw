package tests

import "testing"

func TestChange_Password(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "foo", "-u", "u", "--password=old")
	// Act
	result := runPsw(t, vault, "change", "foo", "--password=new", "--exact")
	// Assert
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "was updated successfully")
	result2 := runPsw(t, vault, "get", "foo", "--exact", "--stdout")
	mustExit(t, result2, 0)
	mustEqual(t, trimmed(result2), "new")
}

func TestChange_Username(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "foo", "-u", "olduser", "--password=p")
	// Act
	result := runPsw(t, vault, "change", "foo", "--username=newuser", "--exact")
	// Assert
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "was updated successfully")
	result2 := runPsw(t, vault)
	mustExit(t, result2, 0)
	mustContain(t, result2.stdout, "(newuser)")
}

func TestChange_Rename(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "foo", "-u", "u", "--password=p")
	// Act
	result := runPsw(t, vault, "change", "foo", "--rename=bar", "--exact")
	// Assert
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "was updated successfully")
	result2 := runPsw(t, vault, "get", "bar", "--exact", "--stdout")
	mustExit(t, result2, 0)
	mustEqual(t, trimmed(result2), "p")
	result3 := runPsw(t, vault, "get", "foo", "--exact", "--stdout")
	mustExit(t, result3, 1)
	mustContain(t, result3.stdout, "Record foo was not found")
}

func TestChange_RenameCollisionRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "foo", "-u", "u", "--password=foopass")
	runPsw(t, vault, "add", "bar", "-u", "u", "--password=barpass")
	// Act
	result := runPsw(t, vault, "change", "foo", "--rename=bar", "--exact")
	// Assert
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "Record with name bar already exists")
	result2 := runPsw(t, vault, "get", "bar", "--exact", "--stdout")
	mustExit(t, result2, 0)
	mustEqual(t, trimmed(result2), "barpass")
}

func TestChange_Value(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "foo", "-s", "--value=v1")
	// Act
	result := runPsw(t, vault, "change", "foo", "--value=v2", "--exact")
	// Assert
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "was updated successfully")
	result2 := runPsw(t, vault, "get", "foo", "--exact", "--stdout")
	mustExit(t, result2, 0)
	mustEqual(t, trimmed(result2), "v2")
}

func TestChange_UserFlagOnValueRecordRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "foo", "-s", "--value=v")
	// Act
	result := runPsw(t, vault, "change", "foo", "--username=x", "--exact")
	// Assert
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "is value-only; --username/--password not applicable")
}

func TestChange_ValueFlagOnUserPassRecordRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "foo", "-u", "u", "--password=p")
	// Act
	result := runPsw(t, vault, "change", "foo", "--value=x", "--exact")
	// Assert
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "is user/pass; --value not applicable")
}

func TestChange_Main(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "foo", "-u", "u", "--password=p")
	// Act
	result := runPswEnv(t, vault, map[string]string{"PSW_NEW_MAIN_PASSWORD": "newpass"},
		"change", "main")
	// Assert
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "Main password changed")
	result2 := runPswEnv(t, vault, map[string]string{"PSW_MAIN_PASSWORD": "newpass"},
		"get", "foo", "--exact", "--stdout")
	mustExit(t, result2, 0)
	mustEqual(t, trimmed(result2), "p")
	result3 := runPsw(t, vault, "get", "foo", "--exact", "--stdout")
	mustExit(t, result3, 0)
	mustContain(t, result3.stdout, "Wrong password.")
}

func TestChange_MainPasswordAlias(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	runPsw(t, vault, "add", "foo", "-u", "u", "--password=p")
	// Act
	result := runPswEnv(t, vault, map[string]string{"PSW_NEW_MAIN_PASSWORD": "newpass"},
		"change", "main-password")
	// Assert
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "Main password changed")
	result2 := runPswEnv(t, vault, map[string]string{"PSW_MAIN_PASSWORD": "newpass"},
		"get", "foo", "--exact", "--stdout")
	mustExit(t, result2, 0)
	mustEqual(t, trimmed(result2), "p")
}

func TestChange_MainWithRecordFlagRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	// Act
	result := runPsw(t, vault, "change", "main", "--rename=x")
	// Assert
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "Record-level flags")
	mustContain(t, result.stdout, "are not valid with 'change main'")
}

func TestChange_MissingRecord(t *testing.T) {
	t.Parallel()
	// Arrange
	vault := newVault(t)
	// Act
	result := runPsw(t, vault, "change", "nope", "--password=x", "--exact")
	// Assert
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "Record nope was not found")
}
