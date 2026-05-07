package strg

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

func ensureDirExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, 0700)
		if err != nil {
			return fmt.Errorf("Error when trying to create directory:\n%w", err)
		}
		slog.Debug(fmt.Sprintf("Directory created: %s", path))
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

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, fmt.Errorf("Error when checking if file %s exists:\n%w", path, err)
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	if err := os.WriteFile(dst, input, 0600); err != nil {
		return fmt.Errorf("failed to write file to destination: %w", err)
	}
	if err := os.Chmod(dst, 0600); err != nil {
		return fmt.Errorf("failed to chmod destination file: %w", err)
	}

	return nil
}
