package tests

import "testing"

func TestChange_Password(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "foo", "-u", "u", "--password=old")
	// Act
	r := runPsw(t, v, "change", "foo", "--password=new", "--exact")
	// Assert
	mustExit(t, r, 0)
	mustContain(t, r.stdout, "Record updated")
	r2 := runPsw(t, v, "get", "foo", "--exact", "--stdout")
	mustExit(t, r2, 0)
	mustEqual(t, trimmed(r2), "new")
}

func TestChange_Username(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "foo", "-u", "olduser", "--password=p")
	// Act
	r := runPsw(t, v, "change", "foo", "--username=newuser", "--exact")
	// Assert
	mustExit(t, r, 0)
	mustContain(t, r.stdout, "Record updated")
	r2 := runPsw(t, v)
	mustExit(t, r2, 0)
	mustContain(t, r2.stdout, "(newuser)")
}

func TestChange_Rename(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "foo", "-u", "u", "--password=p")
	// Act
	r := runPsw(t, v, "change", "foo", "--rename=bar", "--exact")
	// Assert
	mustExit(t, r, 0)
	mustContain(t, r.stdout, "Record updated")
	r2 := runPsw(t, v, "get", "bar", "--exact", "--stdout")
	mustExit(t, r2, 0)
	mustEqual(t, trimmed(r2), "p")
	r3 := runPsw(t, v, "get", "foo", "--exact", "--stdout")
	mustExit(t, r3, 1)
	mustContain(t, r3.stdout, "Record foo was not found")
}

func TestChange_RenameCollisionRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "foo", "-u", "u", "--password=foopass")
	runPsw(t, v, "add", "bar", "-u", "u", "--password=barpass")
	// Act
	r := runPsw(t, v, "change", "foo", "--rename=bar", "--exact")
	// Assert
	mustExit(t, r, 0)
	mustContain(t, r.stdout, "Record with name bar already exists")
	r2 := runPsw(t, v, "get", "bar", "--exact", "--stdout")
	mustExit(t, r2, 0)
	mustEqual(t, trimmed(r2), "barpass")
}

func TestChange_Value(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "foo", "-s", "--value=v1")
	// Act
	r := runPsw(t, v, "change", "foo", "--value=v2", "--exact")
	// Assert
	mustExit(t, r, 0)
	mustContain(t, r.stdout, "Record updated")
	r2 := runPsw(t, v, "get", "foo", "--exact", "--stdout")
	mustExit(t, r2, 0)
	mustEqual(t, trimmed(r2), "v2")
}

func TestChange_UserFlagOnValueRecordRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "foo", "-s", "--value=v")
	// Act
	r := runPsw(t, v, "change", "foo", "--username=x", "--exact")
	// Assert
	mustExit(t, r, 1)
	mustContain(t, r.stdout, "is value-only; --username/--password not applicable")
}

func TestChange_ValueFlagOnUserPassRecordRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "foo", "-u", "u", "--password=p")
	// Act
	r := runPsw(t, v, "change", "foo", "--value=x", "--exact")
	// Assert
	mustExit(t, r, 1)
	mustContain(t, r.stdout, "is user/pass; --value not applicable")
}

func TestChange_Main(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	runPsw(t, v, "add", "foo", "-u", "u", "--password=p")
	// Act
	r := runPswEnv(t, v, map[string]string{"PSW_NEW_MAIN_PASSWORD": "newpass"},
		"change", "main")
	// Assert
	mustExit(t, r, 0)
	mustContain(t, r.stdout, "Main password changed")
	r2 := runPswEnv(t, v, map[string]string{"PSW_MAIN_PASSWORD": "newpass"},
		"get", "foo", "--exact", "--stdout")
	mustExit(t, r2, 0)
	mustEqual(t, trimmed(r2), "p")
	r3 := runPsw(t, v, "get", "foo", "--exact", "--stdout")
	mustExit(t, r3, 0)
	mustContain(t, r3.stdout, "Wrong password.")
}

func TestChange_MainWithRecordFlagRejected(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	// Act
	r := runPsw(t, v, "change", "main", "--rename=x")
	// Assert
	mustExit(t, r, 1)
	mustContain(t, r.stdout, "Record-level flags")
	mustContain(t, r.stdout, "are not valid with 'change main'")
}

func TestChange_MissingRecord(t *testing.T) {
	t.Parallel()
	// Arrange
	v := newVault(t)
	// Act
	r := runPsw(t, v, "change", "nope", "--password=x", "--exact")
	// Assert
	mustExit(t, r, 1)
	mustContain(t, r.stdout, "Record nope was not found")
}
