package storage

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

// TestArgonIterations_Default pins t=3 in normal builds; PSW_FAST_ARGON=1 drops to t=1.
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

// TestEncryptToFile_RoundTrip: round-trip plus file-mode-0600 check.
func TestEncryptToFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage.psw")
	want := []byte("hello world")
	password := []byte("p")

	if err := encryptToFile(path, bytes.Clone(want), password); err != nil {
		t.Fatalf("encryptToFile: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if runtime.GOOS != "windows" {
		if got := info.Mode().Perm(); got != 0600 {
			t.Fatalf("file mode = %o, want 0600", got)
		}
	}

	got, err := decryptFromFile(path, password)
	if err != nil {
		t.Fatalf("decryptFromFile: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("round-trip mismatch: got %q, want %q", got, want)
	}
}

func TestEncryptToFile_NoLeftoverTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage.psw")
	if err := encryptToFile(path, []byte("x"), []byte("p")); err != nil {
		t.Fatalf("encryptToFile: %v", err)
	}
	assertOnlyFile(t, dir, "storage.psw")
}

func TestEncryptToFile_WipesInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage.psw")
	plain := []byte("sensitive")
	if err := encryptToFile(path, plain, []byte("p")); err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	for i, b := range plain {
		if b != 0 {
			t.Fatalf("plain[%d] = %d, want 0 (input not wiped)", i, b)
		}
	}
}

// TestPSW2_RoundTrip_AllSizes covers boundary plaintext sizes around 4 KiB.
func TestPSW2_RoundTrip_AllSizes(t *testing.T) {
	cases := []int{
		1,
		padBlockSize - lengthPrefixSize - 1,
		padBlockSize - lengthPrefixSize, // exactly fills one block
		padBlockSize - lengthPrefixSize + 1,
		padBlockSize,
		2 * padBlockSize,
		3*padBlockSize + 17,
	}
	for _, n := range cases {
		want := bytes.Repeat([]byte("a"), n)
		dir := t.TempDir()
		path := filepath.Join(dir, "storage.psw")
		if err := encryptToFile(path, bytes.Clone(want), []byte("p")); err != nil {
			t.Fatalf("size=%d encrypt: %v", n, err)
		}
		got, err := decryptFromFile(path, []byte("p"))
		if err != nil {
			t.Fatalf("size=%d decrypt: %v", n, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("size=%d round-trip mismatch", n)
		}
	}
}

// encryptedPayload writes plain to a fresh vault file and returns the path plus
// the decoded on-disk payload (magic || salt || sealed).
func encryptedPayload(t *testing.T, plain []byte) (string, []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "storage.psw")
	if err := encryptToFile(path, plain, []byte("p")); err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	payload, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	return path, payload
}

// writePayload re-encodes payload over the vault file at path.
func writePayload(t *testing.T, path string, payload []byte) {
	t.Helper()
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(payload)), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestPSW2_PaddingIsBlockAligned(t *testing.T) {
	_, payload := encryptedPayload(t, bytes.Repeat([]byte("a"), 17)) // far below one block
	sealed := payload[len(magicHeaderV2)+saltLength:]
	// GCMWithRandomNonce: 12-byte nonce prepended + 16-byte tag.
	const gcmOverhead = 12 + 16
	paddedLen := len(sealed) - gcmOverhead
	if paddedLen%padBlockSize != 0 {
		t.Fatalf("padded plaintext length %d is not a multiple of %d", paddedLen, padBlockSize)
	}
}

// TestPSW2_SaltTamperingFailsDecrypt: flipping a salt byte yields a different
// derived key; gcm.Open fails the tag check.
func TestPSW2_SaltTamperingFailsDecrypt(t *testing.T) {
	path, payload := encryptedPayload(t, []byte("hello"))
	payload[len(magicHeaderV2)] ^= 0xFF
	writePayload(t, path, payload)
	if _, err := decryptFromFile(path, []byte("p")); err == nil {
		t.Fatalf("expected decrypt to fail after salt tampering, got nil error")
	}
}

// TestPSW2_UnknownMagicRejected: format check rejects unknown magic before
// decryption.
func TestPSW2_UnknownMagicRejected(t *testing.T) {
	path, payload := encryptedPayload(t, []byte("hello"))
	payload[3] = '3' // PSW2 → PSW3 (unrecognized)
	writePayload(t, path, payload)
	_, err := decryptFromFile(path, []byte("p"))
	if err == nil {
		t.Fatalf("expected decrypt to fail after magic swap")
	}
	if !strings.Contains(err.Error(), "unrecognized storage format") {
		t.Fatalf("expected 'unrecognized storage format', got: %v", err)
	}
}

func TestDecrypt_PSW1ReturnsErrPSW1Unsupported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage.psw")
	if err := writePSW1Vault(path, "hello", []byte("p")); err != nil {
		t.Fatalf("synthesize PSW1: %v", err)
	}
	_, err := decryptFromFile(path, []byte("p"))
	if !errors.Is(err, ErrPSW1Unsupported) {
		t.Fatalf("expected ErrPSW1Unsupported, got %v", err)
	}
}

func TestPSW2_FileBeginsWithPSW2(t *testing.T) {
	_, payload := encryptedPayload(t, []byte("x"))
	if !bytes.Equal(payload[:4], []byte("PSW2")) {
		t.Fatalf("expected PSW2 magic, got %q", string(payload[:4]))
	}
}

func TestUnpadPlaintext_MalformedLengthPrefix(t *testing.T) {
	padded := make([]byte, padBlockSize)
	// Length prefix says 10000 bytes, but the buffer is one 4 KiB block.
	padded[0] = 0x00
	padded[1] = 0x00
	padded[2] = 0x27
	padded[3] = 0x10 // 0x2710 = 10000
	if _, err := unpadPlaintext(padded); err == nil {
		t.Fatalf("expected malformed-length error, got nil")
	}
}

// writePSW1Vault writes a PSW1-format vault (no AAD, no padding).
func writePSW1Vault(filePath, plainText string, password []byte) error {
	salt := make([]byte, saltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return err
	}
	key := argon2.IDKey(password, salt, argonIterations, argonMemoryKiB, argonParallelism, keyLength)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return err
	}
	sealed := gcm.Seal(nil, nil, []byte(plainText), nil)
	payload := append([]byte(magicHeaderV1), salt...)
	payload = append(payload, sealed...)
	encoded := base64.StdEncoding.EncodeToString(payload)
	return os.WriteFile(filePath, []byte(encoded), 0600)
}
