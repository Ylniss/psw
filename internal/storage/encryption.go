package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/awnumar/memguard"
	"github.com/google/renameio/v2"
	"golang.org/x/crypto/argon2"
)

const (
	// magicHeaderV1 is the legacy on-disk magic; refused at decrypt with a
	// pointer to the one-off upgrade tool.
	magicHeaderV1 = "PSW1"
	// magicHeaderV2 is the current on-disk magic. The plaintext is length-
	// prefixed (4-byte BE uint32) and zero-padded to padBlockSize before
	// encryption. The magic header bytes are bound as AAD to gcm.Seal/Open.
	magicHeaderV2 = "PSW2"
	saltLength    = 16
	keyLength     = 32
	// padBlockSize hides record count by rounding plaintext to the next 4 KiB.
	padBlockSize = 4096
	// lengthPrefixSize is the 4-byte uint32 at the head of the padded buffer
	// describing the actual plaintext length.
	lengthPrefixSize = 4
)

// ErrPSW1Unsupported is returned when a legacy PSW1 vault is encountered.
// The migration tool at _oneshot/upgrade.go (one-off, never committed) rewrites
// PSW1 storage as PSW2; run it once, then delete the directory.
var ErrPSW1Unsupported = errors.New("vault is in legacy PSW1 format; run the one-off upgrade tool documented in plans/security-hardening-storage-psw-phase3.md before launching psw again")

// Argon2id parameters. Vars so PSW_FAST_ARGON can lower them for tests.
var (
	argonIterations  uint32 = 3
	argonMemoryKiB   uint32 = 64 * 1024
	argonParallelism uint8  = 4
)

func init() {
	if os.Getenv("PSW_FAST_ARGON") == "1" {
		argonIterations = 1
		argonMemoryKiB = 64
		argonParallelism = 1
	}
}

// EncryptToStorage seals plain under the password enclave and writes the
// resulting PSW2 payload to storage.psw atomically. plain is wiped on return
// (matches memguard.NewEnclave convention).
func EncryptToStorage(plain []byte, password *memguard.Enclave) error {
	pwBuf, err := password.Open()
	if err != nil {
		return fmt.Errorf("open password enclave: %w", err)
	}
	defer pwBuf.Destroy()
	return encryptBytesToFile(Paths.storageFilePath, plain, pwBuf.Bytes())
}

// DecryptFromStorage opens storage.psw and returns the decrypted plaintext.
// Caller must wipe the returned slice when done.
func DecryptFromStorage(password *memguard.Enclave) ([]byte, error) {
	pwBuf, err := password.Open()
	if err != nil {
		return nil, fmt.Errorf("open password enclave: %w", err)
	}
	defer pwBuf.Destroy()
	return decryptBytesFromFile(Paths.storageFilePath, pwBuf.Bytes())
}

func deriveKey(password, salt []byte) []byte {
	return argon2.IDKey(password, salt, argonIterations, argonMemoryKiB, argonParallelism, keyLength)
}

func encryptBytesToFile(filePath string, plain []byte, password []byte) error {
	defer memguard.WipeBytes(plain)

	salt := make([]byte, saltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	key := deriveKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	padded := padPlaintext(plain)
	sealed := gcm.Seal(nil, nil, padded, []byte(magicHeaderV2))
	memguard.WipeBytes(padded)

	payload := make([]byte, 0, len(magicHeaderV2)+saltLength+len(sealed))
	payload = append(payload, magicHeaderV2...)
	payload = append(payload, salt...)
	payload = append(payload, sealed...)

	encoded := base64.StdEncoding.EncodeToString(payload)
	if err := renameio.WriteFile(filePath, []byte(encoded), 0600); err != nil {
		return fmt.Errorf("failed to write encrypted file: %w", err)
	}
	return nil
}

func decryptBytesFromFile(filePath string, password []byte) ([]byte, error) {
	encoded, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read storage file: %w", err)
	}
	return decryptBytes(encoded, password)
}

// decryptBytes decrypts an in-memory base64 PSW2 payload. PSW1 payloads are
// refused with ErrPSW1Unsupported. Caller wipes the returned slice.
func decryptBytes(encoded []byte, password []byte) ([]byte, error) {
	payload, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return nil, fmt.Errorf("failed to decode storage: %w", err)
	}

	if len(payload) < len(magicHeaderV2)+saltLength {
		return nil, errors.New("storage file is corrupted or unrecognized")
	}
	magic := string(payload[:len(magicHeaderV2)])
	if magic == magicHeaderV1 {
		return nil, ErrPSW1Unsupported
	}
	if magic != magicHeaderV2 {
		return nil, fmt.Errorf("unrecognized storage format; expected %s, got %q", magicHeaderV2, magic)
	}

	salt := payload[len(magicHeaderV2) : len(magicHeaderV2)+saltLength]
	sealed := payload[len(magicHeaderV2)+saltLength:]

	key := deriveKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	padded, err := gcm.Open(nil, nil, sealed, []byte(magicHeaderV2))
	if err != nil {
		return nil, errors.New("Wrong password.")
	}
	plain, err := unpadPlaintext(padded)
	if err != nil {
		memguard.WipeBytes(padded)
		return nil, err
	}
	// Defensive copy so callers don't pin the padded buffer's tail.
	out := make([]byte, len(plain))
	copy(out, plain)
	memguard.WipeBytes(padded)
	return out, nil
}

// padPlaintext prepends a 4-byte big-endian length prefix and zero-pads to the
// next padBlockSize boundary. The padded buffer is what gets sealed.
func padPlaintext(plain []byte) []byte {
	total := lengthPrefixSize + len(plain)
	paddedLen := ((total + padBlockSize - 1) / padBlockSize) * padBlockSize
	buf := make([]byte, paddedLen)
	binary.BigEndian.PutUint32(buf[:lengthPrefixSize], uint32(len(plain)))
	copy(buf[lengthPrefixSize:], plain)
	return buf
}

// unpadPlaintext strips the length prefix and returns the actual plaintext.
// Returns a sub-slice of padded; caller must copy out if it intends to wipe
// the source.
func unpadPlaintext(padded []byte) ([]byte, error) {
	if len(padded) < lengthPrefixSize {
		return nil, errors.New("malformed plaintext: missing length prefix")
	}
	n := binary.BigEndian.Uint32(padded[:lengthPrefixSize])
	// Guard against 32-bit int wrap before the bound check below.
	if uint64(n) > math.MaxInt {
		return nil, errors.New("malformed plaintext: length prefix exceeds platform int")
	}
	if int(n) > len(padded)-lengthPrefixSize {
		return nil, errors.New("malformed plaintext: length prefix exceeds buffer")
	}
	return padded[lengthPrefixSize : lengthPrefixSize+int(n)], nil
}
