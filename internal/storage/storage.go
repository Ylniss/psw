package storage

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/awnumar/memguard"
)

// Reserved keywords for the main-password rotation flow. They are not record
// names — they act as sentinels in the `change` command and in the picker.
const (
	MainPasswordKeywordShort = "main"
	MainPasswordKeywordLong  = "main-password"
)

// Record stores either a username/password pair or a single value (never both).
// Password and Value are []byte to allow manual zeroization on overwrite/remove;
// encoding/json serializes them as base64 strings inside the encrypted JSON.
// Discriminator is len(Value) != 0.
type Record struct {
	Name     string `json:"name"`
	Username string `json:"user"`
	Password []byte `json:"pass,omitempty"`
	Value    []byte `json:"value,omitempty"`
	MTime    int64  `json:"mtime,omitempty"`
}

type Storage struct {
	// MainPassword is sealed in a memguard Enclave. Encrypt/decrypt paths Open
	// it locally, defer Destroy on the LockedBuffer.
	MainPassword *memguard.Enclave
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

// AddRecord clones the byte fields so callers can zeroize their input without
// poisoning the stored record.
func (s *Storage) AddRecord(r *Record) {
	r.MTime = time.Now().UnixMilli()
	cp := *r
	cp.Password = bytes.Clone(r.Password)
	cp.Value = bytes.Clone(r.Value)
	s.insertSorted(cp)
}

func (s *Storage) GetRecord(name string) (Record, bool) {
	for _, r := range s.Records {
		if strings.EqualFold(r.Name, name) {
			return r, true
		}
	}
	return Record{}, false
}

// UpdateRecord zeroizes the prior record's secret bytes before overwriting,
// and clones the incoming bytes (same ownership rule as AddRecord).
func (s *Storage) UpdateRecord(name string, updatedRecord Record) {
	for i, r := range s.Records {
		if !strings.EqualFold(r.Name, name) {
			continue
		}
		updatedRecord.MTime = time.Now().UnixMilli()
		cp := updatedRecord
		cp.Password = bytes.Clone(updatedRecord.Password)
		cp.Value = bytes.Clone(updatedRecord.Value)
		zeroize(r.Password)
		zeroize(r.Value)
		// No rename: replace in place.
		if cp.Name == r.Name {
			s.Records[i] = cp
			return
		}
		// Rename: delete + re-insert sorted.
		s.Records = slices.Delete(s.Records, i, i+1)
		s.insertSorted(cp)
		return
	}
}

// RemoveRecord wipes the secret byte fields before dropping the record.
func (s *Storage) RemoveRecord(name string) {
	s.Records = slices.DeleteFunc(s.Records, func(r Record) bool {
		if strings.EqualFold(r.Name, name) {
			zeroize(r.Password)
			zeroize(r.Value)
			return true
		}
		return false
	})
}

func zeroize(b []byte) {
	for i := range b {
		b[i] = 0
	}
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
