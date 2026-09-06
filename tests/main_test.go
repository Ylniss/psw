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

	binDir, err := os.MkdirTemp("", "psw-tests-*")
	if err != nil {
		return 0, fmt.Errorf("mkdtemp: %w", err)
	}
	defer os.RemoveAll(binDir)

	exeSuffix := ""
	if runtime.GOOS == "windows" {
		exeSuffix = ".exe"
	}
	pswBinaryPath = filepath.Join(binDir, "psw"+exeSuffix)
	fakeGpgBinaryPath = filepath.Join(binDir, "fakegpg"+exeSuffix)

	for _, b := range []struct{ out, pkg string }{
		{pswBinaryPath, "./cmd/psw"},
		{fakeGpgBinaryPath, "./tests/cmd/fakegpg"},
	} {
		buildCmd := exec.Command("go", "build", "-o", b.out, b.pkg)
		buildCmd.Dir = root
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			return 0, fmt.Errorf("build %s: %w", b.pkg, err)
		}
	}

	// Mirror `make build`: copy pswcfg-template.toml → <bin dir>/pswcfg.toml
	// so ensureUserConfig and config reset find it.
	tmpl, err := os.ReadFile(filepath.Join(root, "pswcfg-template.toml"))
	if err != nil {
		return 0, fmt.Errorf("read template: %w", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "pswcfg.toml"), tmpl, 0644); err != nil {
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
