package strg

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
)

func EncryptStringToStorage(plainText, password string) error {
	return encryptStringToFile(Cfg.storageFilePath, plainText, password)
}

func DecryptStringFromStorage(password string) (string, error) {
	return decryptStringFromFile(Cfg.storageFilePath, password)
}

func generateSha256Key(password string) []byte {
	hasher := sha256.New()
	hasher.Write([]byte(password))
	return hasher.Sum(nil)
}

// encrypts a plain text string and writes the encrypted data as base64 encoded string to a file.
func encryptStringToFile(filePath, plainText, password string) error {
	key := generateSha256Key(password)
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("Error when acquiring new cipher block:\n%w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("Error when acquiring new GCM:\n%w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("Error when acquiring nonce size:\n%w", err)
	}

	encryptedData := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	encodedData := base64.StdEncoding.EncodeToString(encryptedData)

	err = os.WriteFile(filePath, []byte(encodedData), 0644)
	if err != nil {
		return fmt.Errorf("Error when writing encypted file:\n%w", err)
	}

	return nil
}

// reads encrypted data from a file, decrypts it, and returns the plain text string.
func decryptStringFromFile(filePath, password string) (string, error) {
	encodedData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("Error when reading file to decrypt:\n%w", err)
	}

	encryptedData, err := base64.StdEncoding.DecodeString(string(encodedData))
	if err != nil {
		return "", fmt.Errorf("Error when decoding string to data:\n%w", err)
	}

	key := generateSha256Key(password)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("Error when acquiring new cipher block:\n%w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("Error when acquiring new GCM:\n%w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return "", errors.New("Encrypted data is too short compared to the nonce size")
	}

	nonce, ciphertext := encryptedData[:nonceSize], encryptedData[nonceSize:]
	decryptedData, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("Wrong password.")
	}

	return string(decryptedData), nil
}
