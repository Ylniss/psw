package strg

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

func ensureDirExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			return fmt.Errorf("Error when trying to create directory:\n%w", err)
		}
		log.Debugf("Directory created: %s\n", path)
	} else if err != nil {
		return fmt.Errorf("Error when trying to check directory:\n%w", err)
	}

	return nil
}

func expandPathWithHomePrefix(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("Error while trying to epand ~ in directory as home:\n%w", err)
		}
		path = strings.Replace(path, "~", home, 1)
	}

	return path, nil
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

func moveFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	if err := os.WriteFile(dst, input, 0644); err != nil {
		return fmt.Errorf("failed to write file to destination: %w", err)
	}

	if err := os.Remove(src); err != nil {
		return fmt.Errorf("failed to remove source file: %w", err)
	}

	return nil
}
