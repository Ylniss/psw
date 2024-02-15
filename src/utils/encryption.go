package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"os"
)

func generateKey(password string) []byte {
	hasher := sha256.New()
	hasher.Write([]byte(password))
	return hasher.Sum(nil)
}

// encryptStringToFile encrypts a plain text string and writes the encrypted data as base64 encoded string to a file.
func EncryptStringToFile(filename, plainText, password string) error {
	key := generateKey(password)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	encryptedData := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	// Encode to base64
	encodedData := base64.StdEncoding.EncodeToString(encryptedData)
	return os.WriteFile(filename, []byte(encodedData), 0644)
}

// decryptStringFromFile reads encrypted data from a file, decrypts it, and returns the plain text string.
func DecryptStringFromFile(filename, password string) (string, error) {
	key := generateKey(password)
	encodedData, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}

	encryptedData, err := base64.StdEncoding.DecodeString(string(encodedData))
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return "", err
	}

	nonce, ciphertext := encryptedData[:nonceSize], encryptedData[nonceSize:]
	decryptedData, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(decryptedData), nil
}
