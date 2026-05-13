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

func recordNameCmp(a, b Record) int {
	return strings.Compare(a.Name, b.Name)
}

// insertSorted inserts r at its sorted-by-Name position. Slice is assumed
// already sorted under recordNameCmp.
func (s *Storage) insertSorted(r Record) {
	i, _ := slices.BinarySearchFunc(s.Records, r, recordNameCmp)
	s.Records = slices.Insert(s.Records, i, r)
}

func (s *Storage) AddRecord(r *Record) {
	r.MTime = time.Now().UnixMilli()
	s.insertSorted(*r)
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
		if !strings.EqualFold(r.Name, name) {
			continue
		}
		updatedRecord.MTime = time.Now().UnixMilli()
		// No rename: replace in place.
		if updatedRecord.Name == r.Name {
			s.Records[i] = updatedRecord
			return
		}
		// Rename: delete + re-insert sorted.
		s.Records = slices.Delete(s.Records, i, i+1)
		s.insertSorted(updatedRecord)
		return
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
	slog.Debug("saved", "bytes", len(storageJSON), "records", len(s.Records))
	return EncryptStringToStorage(storageJSON, s.MainPassword)
}
