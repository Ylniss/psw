package storage

import (
	"encoding/json"
	"fmt"

	color "github.com/TwiN/go-color"

	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/ui"
)

// GetOrCreateForRead loads storage without network access.
func GetOrCreateForRead() (*Storage, error) { return getOrCreate(false) }

// GetOrCreateForMutate pulls from the configured remote, merges, then loads.
func GetOrCreateForMutate() (*Storage, error) { return getOrCreate(true) }

func getOrCreate(pull bool) (*Storage, error) {
	mainPassword, created, err := createVaultIfMissing()
	if err != nil {
		return nil, err
	}
	if err := initGitRepoIfNotExists(); err != nil {
		return nil, err
	}
	// Password resolved before pull; merge needs it to decrypt fork/remote blobs.
	if !created && mainPassword == "" {
		mainPassword, err = prompt.PromptForMainPassword(false)
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
func LoadOrCreate(password string, pull bool) (*Storage, error) {
	if err := createEmptyVaultIfMissing(password); err != nil {
		return nil, err
	}
	if err := initGitRepoIfNotExists(); err != nil {
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

// createEmptyVaultIfMissing writes an empty encrypted vault under password
// when storage.psw doesn't exist. No-op when it does.
func createEmptyVaultIfMissing(password string) error {
	exists, err := pathExists(Paths.storageFilePath)
	if err != nil || exists {
		return err
	}
	return EncryptStringToStorage("[]", password)
}

// Decrypt reads storage.psw and returns the *Storage decrypted under
// mainPassword. Errors on wrong password or corrupt file.
func Decrypt(mainPassword string) (*Storage, error) {
	storageJSON, err := DecryptStringFromStorage(mainPassword)
	if err != nil {
		return nil, err
	}

	records, err := getRecords(storageJSON)
	if err != nil {
		return nil, err
	}

	return &Storage{Records: records, MainPassword: mainPassword}, nil
}

func getRecords(storageJSON string) ([]Record, error) {
	var records []Record
	err := json.Unmarshal([]byte(storageJSON), &records)
	if err != nil {
		return nil, fmt.Errorf("error decoding JSON: %w", err)
	}
	return records, nil
}

// createVaultIfMissing prompts for a new main password and seeds an empty
// vault when storage.psw doesn't exist. Returns (password, true, nil) on
// creation, ("", false, nil) when the vault was already there.
func createVaultIfMissing() (string, bool, error) {
	exists, err := pathExists(Paths.storageFilePath)
	if err != nil {
		return "", false, err
	}
	if exists {
		return "", false, nil
	}

	fmt.Println("No encrypted storage found. Set your main password that will be used to decrypt your secrets.")

	mainPassword, err := prompt.PromptForMainPassword(true)
	if err != nil {
		return "", false, err
	}

	if err := EncryptStringToStorage("[]", mainPassword); err != nil {
		return "", false, err
	}

	fmt.Println(
		color.InGreen("Main password set successfully, you can change it with"),
		color.InCyan("change main"),
		color.InGreen("command"))

	return mainPassword, true, nil
}
