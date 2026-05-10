package storage

import (
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"time"
)

// Reserved keywords for the main-password rotation flow. They are not record
// names — they act as sentinels in the `change` command and in the picker.
const (
	MainPasswordKeywordShort = "main"
	MainPasswordKeywordLong  = "main-password"
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

func (s *Storage) ToJSON() (string, error) {
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
	storageJSON, err := s.ToJSON()
	if err != nil {
		return err
	}
	slog.Debug("saved storage content", "json", storageJSON)
	return EncryptStringToStorage(storageJSON, s.MainPassword)
}
