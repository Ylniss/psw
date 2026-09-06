package storage

import (
	"encoding/json"
	"fmt"

	color "github.com/TwiN/go-color"
	"github.com/awnumar/memguard"

	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/ui"
)

// GetOrCreateForRead loads storage without network access.
func GetOrCreateForRead() (*Storage, error) { return getOrCreate(false) }

// GetOrCreateForMutate pulls from the configured remote, merges, then loads.
func GetOrCreateForMutate() (*Storage, error) { return getOrCreate(true) }

func getOrCreate(pull bool) (*Storage, error) {
	mainPassword, created, err := promptAndCreateVaultIfMissing()
	if err != nil {
		return nil, err
	}
	if err := ensureGitRepo(); err != nil {
		return nil, err
	}
	// Password resolved before pull; merge needs it to decrypt fork/remote blobs.
	if !created && mainPassword == nil {
		mainPassword, err = prompt.PromptMainPassword(false)
		if err != nil {
			return nil, err
		}
	}
	if pull {
		mergedStore, err := GitPullAndMerge(mainPassword)
		if err != nil {
			return nil, err
		}
		if mergedStore != nil {
			// Merge already decrypted; skip Decrypt.
			return mergedStore, nil
		}
	}
	var s *Storage
	err = ui.WithSpinner("Decrypting", func() error {
		var gerr error
		s, gerr = Decrypt(mainPassword)
		return gerr
	})
	return s, err
}

// LoadOrCreate decrypts the vault, creating an empty one under password if
// the storage file is missing. Initializes the git repo if missing.
// pull=true also fetches and merges from remote first. No prompts, no spinner.
func LoadOrCreate(password *memguard.Enclave, pull bool) (*Storage, error) {
	if err := createEmptyVaultIfMissing(password); err != nil {
		return nil, err
	}
	if err := ensureGitRepo(); err != nil {
		return nil, err
	}
	if pull {
		mergedStore, err := GitPullAndMerge(password)
		if err != nil {
			return nil, err
		}
		if mergedStore != nil {
			return mergedStore, nil
		}
	}
	return Decrypt(password)
}

// createEmptyVaultIfMissing writes an empty vault if storage.psw is absent.
func createEmptyVaultIfMissing(password *memguard.Enclave) error {
	exists, err := pathExists(Paths.storageFilePath)
	if err != nil || exists {
		return err
	}
	return EncryptToStorage([]byte("[]"), password)
}

// Decrypt loads and decrypts storage.psw under mainPassword.
func Decrypt(mainPassword *memguard.Enclave) (*Storage, error) {
	plain, err := DecryptFromStorage(mainPassword)
	if err != nil {
		return nil, err
	}
	defer memguard.WipeBytes(plain)

	records, err := decodeRecords(plain)
	if err != nil {
		return nil, err
	}

	return &Storage{Records: records, MainPassword: mainPassword}, nil
}

func decodeRecords(plain []byte) ([]Record, error) {
	var records []Record
	if err := json.Unmarshal(plain, &records); err != nil {
		return nil, fmt.Errorf("error decoding JSON: %w", err)
	}
	return records, nil
}

// promptAndCreateVaultIfMissing prompts for a new main password and writes an
// empty vault if storage.psw is absent. Returns the created enclave + true;
// (nil, false) when the vault already exists.
func promptAndCreateVaultIfMissing() (*memguard.Enclave, bool, error) {
	exists, err := pathExists(Paths.storageFilePath)
	if err != nil {
		return nil, false, err
	}
	if exists {
		return nil, false, nil
	}

	fmt.Println("No vault found. Choose a main password to protect your secrets.")

	mainPassword, err := prompt.PromptMainPassword(true)
	if err != nil {
		return nil, false, err
	}

	if err := EncryptToStorage([]byte("[]"), mainPassword); err != nil {
		return nil, false, err
	}

	fmt.Println(color.InGreen("Main password set. You can change it anytime with ") +
		color.InCyan("change main") +
		color.InGreen("."))

	return mainPassword, true, nil
}
