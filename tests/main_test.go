package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

var (
	pswBinaryPath     string
	fakeGpgBinaryPath string
)

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

	pswBinaryFilename := "psw"
	if runtime.GOOS == "windows" {
		pswBinaryFilename = "psw.exe"
	}
	pswBinaryPath = filepath.Join(dir, pswBinaryFilename)
	pswBuildCmd := exec.Command("go", "build", "-o", pswBinaryPath, "./cmd/psw")
	pswBuildCmd.Dir = root
	pswBuildCmd.Stdout = os.Stdout
	pswBuildCmd.Stderr = os.Stderr
	if err := pswBuildCmd.Run(); err != nil {
		return 0, fmt.Errorf("build psw: %w", err)
	}

	fakeGpgFilename := "fakegpg"
	if runtime.GOOS == "windows" {
		fakeGpgFilename = "fakegpg.exe"
	}
	fakeGpgBinaryPath = filepath.Join(dir, fakeGpgFilename)
	fakeGpgBuildCmd := exec.Command("go", "build", "-o", fakeGpgBinaryPath, "./tests/cmd/fakegpg")
	fakeGpgBuildCmd.Dir = root
	fakeGpgBuildCmd.Stdout = os.Stdout
	fakeGpgBuildCmd.Stderr = os.Stderr
	if err := fakeGpgBuildCmd.Run(); err != nil {
		return 0, fmt.Errorf("build fakegpg: %w", err)
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
