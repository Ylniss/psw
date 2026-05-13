package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRemoteURL(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty allowed", "", false},
		{"ssh allowed", "ssh://git@example.com/user/repo.git", false},
		{"git scp-like allowed", "git@example.com:user/repo.git", false},
		{"file allowed", "file:///srv/repo.git", false},
		{"https clean allowed", "https://example.com/user/repo.git", false},
		{"https with userinfo rejected", "https://u:p@example.com/user/repo.git", true},
		{"http with userinfo rejected", "http://u:p@example.com/user/repo.git", true},
		{"https with user-only rejected", "https://token@example.com/user/repo.git", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRemoteURL(tc.input)
			if tc.wantErr && err == nil {
				t.Fatalf("validateRemoteURL(%q) = nil, want error", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateRemoteURL(%q) = %v, want nil", tc.input, err)
			}
		})
	}
}

func TestReadConfigFile_WarnsAndClearsUserinfoRemote(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "pswcfg.toml")
	body := "clipboard_timeout = 20\nremote = \"https://user:secret@example.com/repo.git\"\n"
	if err := os.WriteFile(cfg, []byte(body), 0600); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	prevPath := Paths.configFilePath
	prevSink := WarnSink
	prevApp := AppConfig
	t.Cleanup(func() {
		Paths.configFilePath = prevPath
		WarnSink = prevSink
		AppConfig = prevApp
	})

	var captured []string
	WarnSink = func(s string) { captured = append(captured, s) }
	Paths.configFilePath = cfg

	if err := readConfigFile(); err != nil {
		t.Fatalf("readConfigFile: %v", err)
	}
	if AppConfig.Remote != "" {
		t.Fatalf("remote not cleared: %q", AppConfig.Remote)
	}
	if len(captured) == 0 {
		t.Fatalf("expected a warn, got none")
	}
	joined := strings.Join(captured, "\n")
	if !strings.Contains(joined, "ignoring remote") {
		t.Fatalf("warn missing 'ignoring remote':\n%s", joined)
	}
	if strings.Contains(joined, "secret") {
		t.Fatalf("warn leaked password:\n%s", joined)
	}
}

func TestReadConfigFile_KeepsCleanRemote(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "pswcfg.toml")
	body := "remote = \"git@example.com:user/repo.git\"\n"
	if err := os.WriteFile(cfg, []byte(body), 0600); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	prevPath := Paths.configFilePath
	prevApp := AppConfig
	t.Cleanup(func() {
		Paths.configFilePath = prevPath
		AppConfig = prevApp
	})
	Paths.configFilePath = cfg

	if err := readConfigFile(); err != nil {
		t.Fatalf("readConfigFile: %v", err)
	}
	if AppConfig.Remote != "git@example.com:user/repo.git" {
		t.Fatalf("clean remote was altered: %q", AppConfig.Remote)
	}
}
