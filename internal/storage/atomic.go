package storage

import (
	"errors"
	"os"
	"path/filepath"
)

// writeFileAtomic atomically replaces the file at filePath: write to temp
// file, fsync, rename, then fsync parent dir on POSIX.
func writeFileAtomic(filePath string, data []byte) error {
	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)
	f, err := os.CreateTemp(dir, "."+base+".tmp.*")
	if err != nil {
		return err
	}
	tempPath := f.Name()
	var renamed bool
	defer func() {
		if !renamed {
			os.Remove(tempPath)
		}
	}()
	if _, err := f.Write(data); err != nil {
		return errors.Join(err, f.Close())
	}
	if err := f.Sync(); err != nil {
		return errors.Join(err, f.Close())
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, filePath); err != nil {
		return err
	}
	renamed = true
	return syncDir(dir)
}
