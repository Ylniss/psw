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
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
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

// TestEncryptBytesToFile_RoundTrip exercises the renameio write path: file
// exists at mode 0600 and decrypts back to the original plaintext.
func TestEncryptBytesToFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage.psw")
	want := []byte("hello world")
	password := []byte("p")

	if err := encryptBytesToFile(path, bytes.Clone(want), password); err != nil {
		t.Fatalf("encryptBytesToFile: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("file mode = %o, want 0600", got)
	}

	got, err := decryptBytesFromFile(path, password)
	if err != nil {
		t.Fatalf("decryptBytesFromFile: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("round-trip mismatch: got %q, want %q", got, want)
	}
}

// TestEncryptBytesToFile_NoLeftoverTemp verifies renameio doesn't leave its
// tmp file behind on success.
func TestEncryptBytesToFile_NoLeftoverTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage.psw")
	if err := encryptBytesToFile(path, []byte("x"), []byte("p")); err != nil {
		t.Fatalf("encryptBytesToFile: %v", err)
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

// TestEncryptBytesToFile_WipesInput verifies the input slice is wiped after
// the call (matches memguard.NewEnclave convention so callers don't need to
// wipe themselves).
func TestEncryptBytesToFile_WipesInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage.psw")
	plain := []byte("sensitive")
	if err := encryptBytesToFile(path, plain, []byte("p")); err != nil {
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
		if err := encryptBytesToFile(path, bytes.Clone(want), []byte("p")); err != nil {
			t.Fatalf("size=%d encrypt: %v", n, err)
		}
		got, err := decryptBytesFromFile(path, []byte("p"))
		if err != nil {
			t.Fatalf("size=%d decrypt: %v", n, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("size=%d round-trip mismatch", n)
		}
	}
}

// TestPSW2_PaddingIsBlockAligned verifies that the sealed payload, minus magic
// header and salt, has a length that rounds the plaintext to padBlockSize plus
// the GCM auth tag + nonce overhead.
func TestPSW2_PaddingIsBlockAligned(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage.psw")
	plain := bytes.Repeat([]byte("a"), 17) // far below one block
	if err := encryptBytesToFile(path, plain, []byte("p")); err != nil {
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
	sealed := payload[len(magicHeaderV2)+saltLength:]
	// GCMWithRandomNonce: 12-byte nonce prepended + 16-byte tag.
	const gcmOverhead = 12 + 16
	paddedLen := len(sealed) - gcmOverhead
	if paddedLen%padBlockSize != 0 {
		t.Fatalf("padded plaintext length %d is not a multiple of %d", paddedLen, padBlockSize)
	}
}

// TestPSW2_SaltTamperingFailsDecrypt flips a byte in the salt. The salt is not
// AAD-bound, but it feeds the Argon2id KDF, so a different salt yields a
// different AES key and gcm.Open fails the tag check.
func TestPSW2_SaltTamperingFailsDecrypt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage.psw")
	if err := encryptBytesToFile(path, []byte("hello"), []byte("p")); err != nil {
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
	payload[len(magicHeaderV2)] ^= 0xFF
	tampered := base64.StdEncoding.EncodeToString(payload)
	if err := os.WriteFile(path, []byte(tampered), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := decryptBytesFromFile(path, []byte("p")); err == nil {
		t.Fatalf("expected decrypt to fail after salt tampering, got nil error")
	}
}

// TestPSW2_UnknownMagicRejected swaps the on-disk magic to an unrecognized
// value. The format check rejects before decryption is attempted. The magic-
// as-AAD binding would refuse a hypothetical PSW3 reading a PSW2 ciphertext,
// but here the format check fires first.
func TestPSW2_UnknownMagicRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage.psw")
	if err := encryptBytesToFile(path, []byte("hello"), []byte("p")); err != nil {
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
	payload[3] = '3' // PSW2 → PSW3 (unrecognized)
	tampered := base64.StdEncoding.EncodeToString(payload)
	if err := os.WriteFile(path, []byte(tampered), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err = decryptBytesFromFile(path, []byte("p"))
	if err == nil {
		t.Fatalf("expected decrypt to fail after magic swap")
	}
	if !strings.Contains(err.Error(), "unrecognized storage format") {
		t.Fatalf("expected 'unrecognized storage format', got: %v", err)
	}
}

// TestDecrypt_PSW1ReturnsErrPSW1Unsupported synthesizes a PSW1 blob (the
// pre-phase-3 format) and confirms the new decrypt path refuses with the
// sentinel error.
func TestDecrypt_PSW1ReturnsErrPSW1Unsupported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage.psw")
	if err := writePSW1Vault(path, "hello", []byte("p")); err != nil {
		t.Fatalf("synthesize PSW1: %v", err)
	}
	_, err := decryptBytesFromFile(path, []byte("p"))
	if !errors.Is(err, ErrPSW1Unsupported) {
		t.Fatalf("expected ErrPSW1Unsupported, got %v", err)
	}
}

// writePSW1Vault mirrors the pre-phase-3 encrypt path: PSW1 magic, no AAD, no
// padding. Used to validate the rejection path.
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

// TestPSW2_FileBeginsWithPSW2 inspects the on-disk header bytes.
func TestPSW2_FileBeginsWithPSW2(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage.psw")
	if err := encryptBytesToFile(path, []byte("x"), []byte("p")); err != nil {
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
	if !bytes.Equal(payload[:4], []byte("PSW2")) {
		t.Fatalf("expected PSW2 magic, got %q", string(payload[:4]))
	}
}

// TestUnpadPlaintext_MalformedLengthPrefix rejects an oversized length prefix.
func TestUnpadPlaintext_MalformedLengthPrefix(t *testing.T) {
	padded := make([]byte, padBlockSize)
	// Length prefix says plaintext is 10 KiB, but the buffer is only 4 KiB.
	padded[0] = 0x00
	padded[1] = 0x00
	padded[2] = 0x27
	padded[3] = 0x10 // 0x2710 = 10000
	if _, err := unpadPlaintext(padded); err == nil {
		t.Fatalf("expected malformed-length error, got nil")
	}
}
