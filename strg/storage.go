package strg

import (
	"fmt"
	"sort"
	"strings"

	color "github.com/TwiN/go-color"
	"github.com/samber/lo"
	"github.com/ylniss/psw/utils"
)

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
	names := lo.Map(s.GetNames(), func(n string, _ int) string { return strings.ToLower(n) })
	return lo.Contains(names, strings.ToLower(name))
}

func (s *Storage) String() string {
	storageStr := ""
	for _, r := range s.Records {
		if r.Value == "" { // record is user/pass
			storageStr += cfg.recordMarker + r.Name + cfg.valueEndMarker + r.User + cfg.valueEndMarker + r.Pass + cfg.valueEndMarker
		} else { // record is value only
			storageStr += cfg.recordMarker + r.Name + cfg.valueEndMarker + r.Value + cfg.valueEndMarker
		}
	}

	return storageStr
}

func GetOrCreateIfNotExists() (*Storage, error) {
	mainPass, created, err := createEncryptedStorageIfNotExists()
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

	storageStr, err := DecryptStringFromStorage(mainPass)
	if err != nil {
		return nil, err
	}

	records := getRecords(storageStr)
	storage := Storage{Records: records, MainPass: mainPass}

	return &storage, nil
}

func getRecords(storageStr string) []Record {
	recordsStr := strings.Split(storageStr, cfg.recordMarker)
	recordsStr = recordsStr[1:] // trim from first empty string
	return lo.Map(recordsStr, func(rStr string, _ int) Record {
		values := strings.Split(rStr, cfg.valueEndMarker)
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
func createEncryptedStorageIfNotExists() (string, bool, error) {
	storageFileExists, err := fileExists(cfg.storageFilePath)
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

	err = EncryptStringToStorage("", mainPass)
	fmt.Println(color.InGreen("Main password set successfully"))

	if err != nil {
		return "", false, err
	}

	return mainPass, true, nil
}
