package strg

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	color "github.com/TwiN/go-color"
	"github.com/samber/lo"
	"github.com/ylniss/psw/prmpt"
)

type Record struct {
	Name  string `json:"name"`
	User  string `json:"user"`
	Pass  string `json:"pass"`
	Value string `json:"value"`
}

type Storage struct {
	MainPass string
	Records  []Record
}

func (s *Storage) GetNames() []string {
	return lo.Map(s.Records, func(r Record, _ int) string { return r.Name })
}

func (s *Storage) AddRecord(r *Record) {
	s.Records = append(s.Records, *r)

	// Sort Records alphabetically by Name
	sort.Slice(s.Records, func(i, j int) bool {
		return s.Records[i].Name < s.Records[j].Name
	})
}

func (s *Storage) GetRecord(name string) (Record, bool) {
	return lo.Find(s.Records, func(r Record) bool { return r.Name == name })
}

func (s *Storage) IsDuplicate(name string) bool {
	names := lo.Map(s.GetNames(), func(n string, _ int) string { return strings.ToLower(n) })
	return lo.Contains(names, strings.ToLower(name))
}

func (s *Storage) ToJson() (string, error) {
	jsonData, err := json.MarshalIndent(s.Records, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

func GetOrCreateIfNotExists() (*Storage, error) {
	mainPass, created, err := createEncryptedStorageIfNotExists()
	if err != nil {
		return nil, err
	}

	// when storage already exists, prompt for password to access
	if !created && mainPass == "" {
		mainPass, err = prmpt.PromptForMainPass(false)
		if err != nil {
			return nil, err
		}
	}

	storageJson, err := DecryptStringFromStorage(mainPass)
	if err != nil {
		return nil, err
	}

	records, err := getRecords(storageJson)
	if err != nil {
		return nil, err
	}

	storage := Storage{Records: records, MainPass: mainPass}

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
	storageFileExists, err := fileExists(cfg.storageFilePath)
	if err != nil {
		return "", false, err
	}

	if storageFileExists {
		return "", false, nil
	}

	fmt.Println("No encrypted storage found. Set your main password that will be used to decrypt your secrets.")

	mainPass, err := prmpt.PromptForMainPass(true)
	if err != nil {
		return "", false, err
	}

	err = EncryptStringToStorage("[]", mainPass)
	if err != nil {
		return "", false, err
	}

	fmt.Println(
		color.InGreen("Main password set successfully, you can change it with"),
		color.InCyan("change main"),
		color.InGreen("command"))

	return mainPass, true, nil
}
