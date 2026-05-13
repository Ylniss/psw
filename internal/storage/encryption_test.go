package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// TestArgonIterations_Default pins the OWASP "balanced for desktop" t=3
// setting. PSW_FAST_ARGON=1 lowers it to t=1 for the integration tests; assert
// whichever branch matches the test environment.
func TestArgonIterations_Default(t *testing.T) {
	if os.Getenv("PSW_FAST_ARGON") == "1" {
		if argonIterations != 1 {
			t.Fatalf("PSW_FAST_ARGON=1: argonIterations = %d, want 1", argonIterations)
		}
		return
	}
	if argonIterations != 3 {
		t.Fatalf("argonIterations = %d, want 3", argonIterations)
	}
}

// TestEncryptStringToFile_RoundTrip exercises the renameio write path: file
// exists at mode 0600 and decrypts back to the original plaintext.
func TestEncryptStringToFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage.psw")
	plain := "hello world"
	password := "p"

	if err := encryptStringToFile(path, plain, password); err != nil {
		t.Fatalf("encryptStringToFile: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("file mode = %o, want 0600", got)
	}

	got, err := decryptStringFromFile(path, password)
	if err != nil {
		t.Fatalf("decryptStringFromFile: %v", err)
	}
	if got != plain {
		t.Fatalf("round-trip mismatch: got %q, want %q", got, plain)
	}
}

// TestEncryptStringToFile_NoLeftoverTemp verifies renameio doesn't leave its
// tmp file behind on success.
func TestEncryptStringToFile_NoLeftoverTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage.psw")
	if err := encryptStringToFile(path, "x", "p"); err != nil {
		t.Fatalf("encryptStringToFile: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "storage.psw" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Fatalf("expected only storage.psw, got %v", names)
	}
}
