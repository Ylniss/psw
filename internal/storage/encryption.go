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
	// magicHeaderV1 is the pre-V2 on-disk magic. Decrypt rejects with ErrPSW1Unsupported.
	magicHeaderV1 = "PSW1"
	// magicHeaderV2 is the current on-disk magic.
	magicHeaderV2 = "PSW2"
	saltLength    = 16
	keyLength     = 32
	// padBlockSize rounds plaintext up to hide record count.
	padBlockSize = 4096
	// lengthPrefixSize is the BE uint32 at the head of the padded buffer.
	lengthPrefixSize = 4
)

// ErrPSW1Unsupported is returned for PSW1 vaults; see error text for the upgrade path.
var ErrPSW1Unsupported = errors.New("vault is in legacy PSW1 format; run the one-off upgrade tool documented in plans/security-hardening-storage-psw-phase3.md before launching psw again")

// Argon2id parameters; tests override via PSW_FAST_ARGON.
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

// EncryptToStorage seals plain and writes storage.psw atomically. plain is
// wiped on return.
func EncryptToStorage(plain []byte, password *memguard.Enclave) error {
	pwBuf, err := password.Open()
	if err != nil {
		return fmt.Errorf("open password enclave: %w", err)
	}
	defer pwBuf.Destroy()
	return encryptToFile(Paths.storageFilePath, plain, pwBuf.Bytes())
}

// DecryptFromStorage loads and decrypts storage.psw. Caller wipes the result.
func DecryptFromStorage(password *memguard.Enclave) ([]byte, error) {
	pwBuf, err := password.Open()
	if err != nil {
		return nil, fmt.Errorf("open password enclave: %w", err)
	}
	defer pwBuf.Destroy()
	return decryptFromFile(Paths.storageFilePath, pwBuf.Bytes())
}

func deriveKey(password, salt []byte) []byte {
	return argon2.IDKey(password, salt, argonIterations, argonMemoryKiB, argonParallelism, keyLength)
}

func encryptToFile(filePath string, plain []byte, password []byte) error {
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

func decryptFromFile(filePath string, password []byte) ([]byte, error) {
	encoded, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read storage file: %w", err)
	}
	return decryptBlob(encoded, password)
}

// decryptBlob decodes and unseals a base64 payload. Caller wipes the result.
func decryptBlob(encoded []byte, password []byte) ([]byte, error) {
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
	// Copy out — padded is wiped below.
	out := make([]byte, len(plain))
	copy(out, plain)
	memguard.WipeBytes(padded)
	return out, nil
}

// padPlaintext prepends a 4-byte BE length prefix and zero-pads to padBlockSize.
func padPlaintext(plain []byte) []byte {
	total := lengthPrefixSize + len(plain)
	paddedLen := ((total + padBlockSize - 1) / padBlockSize) * padBlockSize
	buf := make([]byte, paddedLen)
	binary.BigEndian.PutUint32(buf[:lengthPrefixSize], uint32(len(plain)))
	copy(buf[lengthPrefixSize:], plain)
	return buf
}

// unpadPlaintext returns the plaintext as a sub-slice of padded. Caller copies
// out before wiping the source.
func unpadPlaintext(padded []byte) ([]byte, error) {
	if len(padded) < lengthPrefixSize {
		return nil, errors.New("malformed plaintext: missing length prefix")
	}
	n := binary.BigEndian.Uint32(padded[:lengthPrefixSize])
	// Reject lengths that wrap a platform int.
	if uint64(n) > math.MaxInt {
		return nil, errors.New("malformed plaintext: length prefix exceeds platform int")
	}
	if int(n) > len(padded)-lengthPrefixSize {
		return nil, errors.New("malformed plaintext: length prefix exceeds buffer")
	}
	return padded[lengthPrefixSize : lengthPrefixSize+int(n)], nil
}
