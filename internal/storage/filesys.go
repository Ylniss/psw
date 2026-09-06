package storage

import (
	"fmt"
	"os"
	"strings"
)

func ensureDirExists(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return fmt.Errorf("create directory %s: %w", path, err)
	}
	return nil
}

func expandPathWithHomePrefix(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand ~ in path: %w", err)
		}
		path = strings.Replace(path, "~", home, 1)
	}

	return path, nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, fmt.Errorf("stat %s: %w", path, err)
}

func copyFile(source, destination string) error {
	input, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	if err := writeFileAtomic(destination, input); err != nil {
		return fmt.Errorf("failed to write file to destination: %w", err)
	}

	return nil
}
