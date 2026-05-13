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

// Sentinel keywords for the main-password rotation flow; not valid record names.
const (
	MainPasswordAlias = "main"
	MainPasswordName  = "main-password"
)

// Record holds a user+password pair or a single value, never both.
// Discriminator: len(Value) != 0. Secret fields are []byte to allow wiping.
type Record struct {
	Name     string `json:"name"`
	Username string `json:"user"`
	Password []byte `json:"pass,omitempty"`
	Value    []byte `json:"value,omitempty"`
	MTime    int64  `json:"mtime,omitempty"`
}

// cloneSecrets returns r with Password/Value deep-copied.
func cloneSecrets(r Record) Record {
	r.Password = bytes.Clone(r.Password)
	r.Value = bytes.Clone(r.Value)
	return r
}

type Storage struct {
	// MainPassword is sealed; opened on demand at encrypt/decrypt sites.
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

// AddRecord deep-copies r.Password/r.Value; callers may wipe their input.
func (s *Storage) AddRecord(r *Record) {
	r.MTime = time.Now().UnixMilli()
	s.insertSorted(cloneSecrets(*r))
}

func (s *Storage) GetRecord(name string) (Record, bool) {
	for _, r := range s.Records {
		if strings.EqualFold(r.Name, name) {
			return r, true
		}
	}
	return Record{}, false
}

// UpdateRecord wipes the prior secret bytes and deep-copies the incoming bytes.
func (s *Storage) UpdateRecord(name string, updatedRecord Record) {
	for i, r := range s.Records {
		if !strings.EqualFold(r.Name, name) {
			continue
		}
		updatedRecord.MTime = time.Now().UnixMilli()
		cp := cloneSecrets(updatedRecord)
		memguard.WipeBytes(r.Password)
		memguard.WipeBytes(r.Value)
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
			memguard.WipeBytes(r.Password)
			memguard.WipeBytes(r.Value)
			return true
		}
		return false
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

// MarshalRecords serializes records to indented JSON bytes. Caller wipes.
func (s *Storage) MarshalRecords() ([]byte, error) {
	return json.MarshalIndent(s.Records, "", "  ")
}

// Save writes the records encrypted under MainPassword.
func (s *Storage) Save() error {
	plain, err := s.MarshalRecords()
	if err != nil {
		return err
	}
	slog.Debug("saved", "bytes", len(plain), "records", len(s.Records))
	return EncryptToStorage(plain, s.MainPassword)
}
