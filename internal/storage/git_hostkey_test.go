package storage

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	cryptossh "golang.org/x/crypto/ssh"
)

func newSSHKey(t *testing.T) cryptossh.PublicKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	signer, err := cryptossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	return signer.PublicKey()
}

func newKnownHostsFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatalf("create known_hosts: %v", err)
	}
	return path
}

func tcpAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 22}
}

func TestAcceptNew_UnknownHostAppendsAndAccepts(t *testing.T) {
	path := newKnownHostsFile(t)
	cb, err := buildHostKeyCallback(path)
	if err != nil {
		t.Fatalf("build callback: %v", err)
	}

	key := newSSHKey(t)
	if err := cb("example.com:22", tcpAddr(), key); err != nil {
		t.Fatalf("first connect should accept-new, got %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read known_hosts: %v", err)
	}
	if !strings.Contains(string(data), "example.com") {
		t.Fatalf("known_hosts missing entry:\n%s", data)
	}
}

func TestAcceptNew_KnownHostSameKeyAccepts(t *testing.T) {
	path := newKnownHostsFile(t)
	key := newSSHKey(t)

	// Build twice: first call pins, second uses a fresh callback that reads
	// the now-populated file. Same key → silent accept.
	cb1, err := buildHostKeyCallback(path)
	if err != nil {
		t.Fatalf("build cb1: %v", err)
	}
	if err := cb1("host.test:22", tcpAddr(), key); err != nil {
		t.Fatalf("pin: %v", err)
	}

	cb2, err := buildHostKeyCallback(path)
	if err != nil {
		t.Fatalf("build cb2: %v", err)
	}
	if err := cb2("host.test:22", tcpAddr(), key); err != nil {
		t.Fatalf("second connect with same key rejected: %v", err)
	}
}

func TestAcceptNew_KnownHostDifferentKeyReturnsErrHostKeyChanged(t *testing.T) {
	path := newKnownHostsFile(t)
	key1 := newSSHKey(t)
	key2 := newSSHKey(t)

	cb1, err := buildHostKeyCallback(path)
	if err != nil {
		t.Fatalf("build cb1: %v", err)
	}
	if err := cb1("host.test:22", tcpAddr(), key1); err != nil {
		t.Fatalf("pin: %v", err)
	}

	cb2, err := buildHostKeyCallback(path)
	if err != nil {
		t.Fatalf("build cb2: %v", err)
	}
	err = cb2("host.test:22", tcpAddr(), key2)
	if err == nil {
		t.Fatal("expected ErrHostKeyChanged, got nil")
	}
	if !errors.Is(err, ErrHostKeyChanged) {
		t.Fatalf("expected ErrHostKeyChanged, got %v", err)
	}
	if !strings.Contains(err.Error(), "host.test") {
		t.Fatalf("error missing host name: %v", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error missing known_hosts path: %v", err)
	}
}

func TestKnownHostsPath_CreatesFileAndDirIfMissing(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	path, err := knownHostsPath()
	if err != nil {
		t.Fatalf("knownHostsPath: %v", err)
	}
	wantPath := filepath.Join(tmpHome, ".ssh", "known_hosts")
	if path != wantPath {
		t.Fatalf("path = %q, want %q", path, wantPath)
	}

	assertPerm(t, filepath.Join(tmpHome, ".ssh"), 0700)
	assertPerm(t, path, 0600)
}

// assertPerm checks that path exists and, off Windows (which carries no Unix
// mode), that its permission bits match want.
func assertPerm(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s perm = %o, want %o", path, got, want)
	}
}
