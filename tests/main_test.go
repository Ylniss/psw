package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

var pswBinary string

func TestMain(m *testing.M) {
	code, err := buildAndRun(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(code)
}

func buildAndRun(m *testing.M) (int, error) {
	root, err := repoRoot()
	if err != nil {
		return 0, err
	}

	dir, err := os.MkdirTemp("", "psw-tests-*")
	if err != nil {
		return 0, fmt.Errorf("mkdtemp: %w", err)
	}
	defer os.RemoveAll(dir)

	binName := "psw"
	if runtime.GOOS == "windows" {
		binName = "psw.exe"
	}
	pswBinary = filepath.Join(dir, binName)
	cmd := exec.Command("go", "build", "-o", pswBinary, "./cmd/psw")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("build psw: %w", err)
	}

	// Mirror `make build`: copy pswcfg-template.toml → <bin dir>/pswcfg.toml
	// so ensureUserConfig and config reset find it.
	tmpl, err := os.ReadFile(filepath.Join(root, "pswcfg-template.toml"))
	if err != nil {
		return 0, fmt.Errorf("read template: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pswcfg.toml"), tmpl, 0644); err != nil {
		return 0, fmt.Errorf("write template next to binary: %w", err)
	}

	return m.Run(), nil
}

func repoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", cwd)
		}
		dir = parent
	}
}
