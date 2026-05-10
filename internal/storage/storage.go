package storage

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	color "github.com/TwiN/go-color"
	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/ui"
)

type Record struct {
	Name     string `json:"name"`
	Username string `json:"user"`
	Password string `json:"pass"`
	Value    string `json:"value"`
	MTime    int64  `json:"mtime,omitempty"`
}

type Storage struct {
	MainPassword string
	Records      []Record
}

func (s *Storage) GetNames() []string {
	names := make([]string, len(s.Records))
	for i, r := range s.Records {
		names[i] = r.Name
	}
	return names
}

type NameAndUser struct {
	Name     string
	Username string
}

func (s *Storage) GetNamesAndUsers() []NameAndUser {
	nameAndUsers := make([]NameAndUser, len(s.Records))
	for i, r := range s.Records {
		nameAndUsers[i] = NameAndUser{Name: r.Name, Username: r.Username}
	}
	return nameAndUsers
}

func (s *Storage) GetNamesWithPart(namePart string) []string {
	lowercaseNamePart := strings.ToLower(namePart)
	var matched []string
	for _, r := range s.Records {
		if strings.Contains(strings.ToLower(r.Name), lowercaseNamePart) {
			matched = append(matched, r.Name)
		}
	}
	return matched
}

func (s *Storage) sortRecords() {
	slices.SortFunc(s.Records, func(a, b Record) int {
		return strings.Compare(a.Name, b.Name)
	})
}

func (s *Storage) AddRecord(r *Record) {
	r.MTime = time.Now().UnixMilli()
	s.Records = append(s.Records, *r)
	s.sortRecords()
}

func (s *Storage) GetRecord(name string) (Record, bool) {
	for _, r := range s.Records {
		if strings.EqualFold(r.Name, name) {
			return r, true
		}
	}
	return Record{}, false
}

func (s *Storage) UpdateRecord(name string, updatedRecord Record) {
	for i, r := range s.Records {
		if strings.EqualFold(r.Name, name) {
			updatedRecord.MTime = time.Now().UnixMilli()
			s.Records[i] = updatedRecord
			s.sortRecords()
			return
		}
	}
}

func (s *Storage) RemoveRecord(name string) {
	s.Records = slices.DeleteFunc(s.Records, func(r Record) bool {
		return strings.EqualFold(r.Name, name)
	})
}

func (s *Storage) Exists(name string) bool {
	for _, r := range s.Records {
		if strings.EqualFold(r.Name, name) {
			return true
		}
	}
	return false
}

func (s *Storage) ToJson() (string, error) {
	jsonData, err := json.MarshalIndent(s.Records, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// Save serializes records to JSON and writes them encrypted under the
// storage's current MainPassword. To change the main password, mutate
// storage.MainPassword before calling Save.
func (s *Storage) Save() error {
	storageJson, err := s.ToJson()
	if err != nil {
		return err
	}
	slog.Debug("saved storage content", "json", storageJson)
	return EncryptStringToStorage(storageJson, s.MainPassword)
}

// GetOrCreateForRead loads storage without network access.
// Used by `psw`, `psw get`, `psw log`.
func GetOrCreateForRead() (*Storage, error) { return getOrCreate(false) }

// GetOrCreateForMutate pulls from the configured remote, merges, then loads.
// Used by `psw add`, `psw change`, `psw remove`.
func GetOrCreateForMutate() (*Storage, error) { return getOrCreate(true) }

func getOrCreate(pull bool) (*Storage, error) {
	mainPassword, created, err := createEncryptedStorageIfNotExists()
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
		if err := GitPullAndMerge(mainPassword); err != nil {
			return nil, err
		}
	}
	var s *Storage
	err = ui.WithSpinner("Decrypting", func() error {
		var gerr error
		s, gerr = Get(mainPassword)
		return gerr
	})
	return s, err
}

// LoadOrCreate decrypts the vault, creating an empty one under password if
// the storage file is missing. Initializes the git repo if missing.
// pull=true also fetches and merges from remote first. No prompts, no spinner.
func LoadOrCreate(password string, pull bool) (*Storage, error) {
	if err := createIfMissing(password); err != nil {
		return nil, err
	}
	if err := initGitRepoIfNotExists(); err != nil {
		return nil, err
	}
	if pull {
		if err := GitPullAndMerge(password); err != nil {
			return nil, err
		}
	}
	return Get(password)
}

func createIfMissing(password string) error {
	exists, err := pathExists(Paths.storageFilePath)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return EncryptStringToStorage("[]", password)
}

func Get(mainPassword string) (*Storage, error) {
	storageJson, err := DecryptStringFromStorage(mainPassword)
	if err != nil {
		return nil, err
	}

	records, err := getRecords(storageJson)
	if err != nil {
		return nil, err
	}

	storage := Storage{Records: records, MainPassword: mainPassword}

	return &storage, nil
}

func getRecords(storageJson string) ([]Record, error) {
	var records []Record
	err := json.Unmarshal([]byte(storageJson), &records)
	if err != nil {
		// Return an empty slice and the error
		return nil, fmt.Errorf("error decoding JSON: %w", err)
	}
	return records, nil
}

// returns true and password used to create storage if created storage
// or false with empty string when error occured or storage already existed
func createEncryptedStorageIfNotExists() (string, bool, error) {
	storageFileExists, err := pathExists(Paths.storageFilePath)
	if err != nil {
		return "", false, err
	}

	if storageFileExists {
		return "", false, nil
	}

	fmt.Println("No encrypted storage found. Set your main password that will be used to decrypt your secrets.")

	mainPassword, err := prompt.PromptForMainPassword(true)
	if err != nil {
		return "", false, err
	}

	err = EncryptStringToStorage("[]", mainPassword)
	if err != nil {
		return "", false, err
	}

	fmt.Println(
		color.InGreen("Main password set successfully, you can change it with"),
		color.InCyan("change main"),
		color.InGreen("command"))

	return mainPassword, true, nil
}
