package strg

import (
	"fmt"
	"os"
	"sort"
	"strings"

	color "github.com/TwiN/go-color"
	"github.com/samber/lo"
	"github.com/ylniss/psw/utils"
)

type StorageCfg struct {
	RecordMarker   string
	ValueEndMarker string
}

var Cfg = StorageCfg{
	RecordMarker:   "!===##$$##$$##$$##$$===!\n",
	ValueEndMarker: "(;+!_+_!+;)\n",
}

type Record struct {
	Name  string
	User  string
	Pass  string
	Value string
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
	names := s.GetNames()
	return lo.Contains(names, name)
}

func (s *Storage) String() string {
	storageStr := ""
	for _, r := range s.Records {
		if r.Value == "" { // record is user/pass
			storageStr += Cfg.RecordMarker + r.Name + Cfg.ValueEndMarker + r.User + Cfg.ValueEndMarker + r.Pass + Cfg.ValueEndMarker
		} else { // record is value only
			storageStr += Cfg.RecordMarker + r.Name + Cfg.ValueEndMarker + r.Value + Cfg.ValueEndMarker
		}
	}

	return storageStr
}

func GetOrCreateIfNotExists(storageFilePath string) (*Storage, error) {
	mainPass, created, err := createEncryptedStorageIfNotExists(storageFilePath)
	if err != nil {
		return nil, err
	}

	// when storage already exists, prompt for password to access
	if !created && mainPass == "" {
		mainPass, err = utils.PromptForMainPass(false)
		if err != nil {
			return nil, err
		}
	}

	storageStr, err := DecryptStringFromFile(storageFilePath, mainPass)
	if err != nil {
		return nil, err
	}

	records := getRecords(storageStr)
	storage := Storage{Records: records, MainPass: mainPass}

	return &storage, nil
}

func getRecords(storageStr string) []Record {
	recordsStr := strings.Split(storageStr, Cfg.RecordMarker)
	recordsStr = recordsStr[1:] // trim from first empty string
	return lo.Map(recordsStr, func(rStr string, _ int) Record {
		values := strings.Split(rStr, Cfg.ValueEndMarker)
		if len(values) == 1 { // empty
			return Record{}
		}

		if len(values) == 2 { // single value instead of user/pass
			return Record{Name: values[0], Value: values[1]}
		}

		return Record{Name: values[0], User: values[1], Pass: values[2]}
	})
}

// returns true and password used to create storage if created storage
// or false with empty string when error occured or storage already existed
func createEncryptedStorageIfNotExists(storageFilePath string) (string, bool, error) {
	storageFileExists, err := fileExists(storageFilePath)
	if err != nil {
		return "", false, err
	}

	if storageFileExists {
		return "", false, nil
	}

	fmt.Println("No encrypted storage found. Set your main password that will be used to decrypt your secrets.")

	mainPass, err := utils.PromptForMainPass(true)
	if err != nil {
		return "", false, err
	}

	err = EncryptStringToFile(storageFilePath, "", mainPass)
	fmt.Println(color.InGreen("Main password set successfully"))

	if err != nil {
		return "", false, err
	}

	return mainPass, true, nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, fmt.Errorf("Error when checking if file %s exists:\n%w", path, err)
}
