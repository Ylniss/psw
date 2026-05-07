package strg

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
	magicV1      = "PSW1"
	saltLen      = 16
	keyLen       = 32
	argonTime    = 2
	argonMemory  = 64 * 1024
	argonThreads = 4
)

func EncryptStringToStorage(plainText, password string) error {
	return encryptStringToFile(Cfg.storageFilePath, plainText, password)
}

func DecryptStringFromStorage(password string) (string, error) {
	return decryptStringFromFile(Cfg.storageFilePath, password)
}

func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, keyLen)
}

func encryptStringToFile(filePath, plainText, password string) error {
	salt := make([]byte, saltLen)
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

	payload := make([]byte, 0, len(magicV1)+saltLen+len(sealed))
	payload = append(payload, magicV1...)
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
	payload, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return "", fmt.Errorf("failed to decode storage: %w", err)
	}

	if len(payload) < len(magicV1)+saltLen {
		return "", errors.New("storage file is corrupted or unrecognized")
	}
	if string(payload[:len(magicV1)]) != magicV1 {
		return "", errors.New("unrecognized storage format; expected PSW1")
	}

	salt := payload[len(magicV1) : len(magicV1)+saltLen]
	sealed := payload[len(magicV1)+saltLen:]

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
