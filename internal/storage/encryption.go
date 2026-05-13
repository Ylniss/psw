package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/argon2"
)

const (
	magicHeaderV1 = "PSW1"
	saltLength    = 16
	keyLength     = 32
)

// Argon2id parameters. Vars so PSW_FAST_ARGON can lower them for tests.
var (
	argonIterations  uint32 = 2
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

func EncryptStringToStorage(plainText, password string) error {
	return encryptStringToFile(Paths.storageFilePath, plainText, password)
}

func DecryptStringFromStorage(password string) (string, error) {
	return decryptStringFromFile(Paths.storageFilePath, password)
}

func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, argonIterations, argonMemoryKiB, argonParallelism, keyLength)
}

func encryptStringToFile(filePath, plainText, password string) error {
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

	sealed := gcm.Seal(nil, nil, []byte(plainText), nil)

	payload := make([]byte, 0, len(magicHeaderV1)+saltLength+len(sealed))
	payload = append(payload, magicHeaderV1...)
	payload = append(payload, salt...)
	payload = append(payload, sealed...)

	encoded := base64.StdEncoding.EncodeToString(payload)
	if err := os.WriteFile(filePath, []byte(encoded), 0600); err != nil {
		return fmt.Errorf("failed to write encrypted file: %w", err)
	}
	if err := os.Chmod(filePath, 0600); err != nil {
		return fmt.Errorf("failed to chmod encrypted file: %w", err)
	}
	return nil
}

func decryptStringFromFile(filePath, password string) (string, error) {
	encoded, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read storage file: %w", err)
	}
	return decryptBytes(encoded, password)
}

// decryptBytes decrypts an in-memory base64 PSW1 payload.
func decryptBytes(encoded []byte, password string) (string, error) {
	payload, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return "", fmt.Errorf("failed to decode storage: %w", err)
	}

	if len(payload) < len(magicHeaderV1)+saltLength {
		return "", errors.New("storage file is corrupted or unrecognized")
	}
	if string(payload[:len(magicHeaderV1)]) != magicHeaderV1 {
		return "", errors.New("unrecognized storage format; expected PSW1")
	}

	salt := payload[len(magicHeaderV1) : len(magicHeaderV1)+saltLength]
	sealed := payload[len(magicHeaderV1)+saltLength:]

	key := deriveKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}
	plain, err := gcm.Open(nil, nil, sealed, nil)
	if err != nil {
		return "", errors.New("Wrong password.")
	}
	return string(plain), nil
}
