package strg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
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

type NameAndUser struct {
	Name string
	User string
}

func (s *Storage) GetNamesAndUsers() []NameAndUser {
	return lo.Map(s.Records, func(r Record, _ int) NameAndUser { return NameAndUser{Name: r.Name, User: r.User} })
}

func (s *Storage) GetNamesWithPart(namePart string) []string {
	names := s.GetNames()
	return lo.FilterMap(names, func(name string, _ int) (string, bool) {
		return name, strings.Contains(name, namePart)
	})
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

func (s *Storage) UpdateRecord(name string, updatedRecord Record) {
	records := lo.Map(s.Records, func(r Record, _ int) Record {
		if r.Name != name {
			return r
		} else {
			return updatedRecord
		}
	})

	s.Records = records
}

func (s *Storage) RemoveRecord(name string) {
	records := lo.Filter(s.Records, func(r Record, _ int) bool {
		return r.Name != name
	})

	s.Records = records
}

func (s *Storage) Exists(name string) bool {
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

	err = initGitRepoIfNotExists()
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

	return Get(mainPass)
}

func Get(mainPass string) (*Storage, error) {
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

func GetRecordNameWithFzf(names []string) (string, error) {
	if len(names) == 1 {
		return names[0], nil
	}

	// Check if fzf is installed
	if _, err := exec.LookPath("fzf"); err != nil {
		return "", fmt.Errorf("fzf is not installed. Please install fzf to use this feature")
	}

	cmd := exec.Command("fzf")

	var input bytes.Buffer
	input.WriteString(strings.Join(names, "\n"))
	cmd.Stdin = &input

	var output bytes.Buffer
	cmd.Stdout = &output

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("Failed to run fzf:\n%w", err)
	}

	return strings.TrimSpace(output.String()), nil
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
	storageFileExists, err := pathExists(Cfg.storageFilePath)
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
