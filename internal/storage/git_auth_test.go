package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	httpauth "github.com/go-git/go-git/v5/plumbing/transport/http"
)

func TestClassifyRemote(t *testing.T) {
	cases := []struct {
		url  string
		want remoteKind
	}{
		{"git@github.com:user/repo.git", remoteSSH},
		{"ssh://git@host/repo.git", remoteSSH},
		{"https://github.com/user/repo.git", remoteHTTPS},
		{"https://user:token@host/repo.git", remoteHTTPS},
		{"http://localhost:8080/repo", remoteHTTPS},
		{"file:///tmp/bare", remoteFile},
		{"/tmp/bare", remoteFile},
		{"ftp://x", remoteUnknown},
		{"", remoteUnknown},
	}
	for _, c := range cases {
		if got := classifyRemote(c.url); got != c.want {
			t.Errorf("classifyRemote(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestHTTPSAuth(t *testing.T) {
	t.Run("token in URL → BasicAuth", func(t *testing.T) {
		auth, err := httpsAuth("https://alice:s3cret@host/repo.git")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		basic, ok := auth.(*httpauth.BasicAuth)
		if !ok {
			t.Fatalf("expected *BasicAuth, got %T", auth)
		}
		if basic.Username != "alice" || basic.Password != "s3cret" {
			t.Errorf("got user=%q pass=%q, want alice/s3cret", basic.Username, basic.Password)
		}
	})

	t.Run("no token + no helper → anonymous nil auth", func(t *testing.T) {
		t.Setenv("PATH", "")
		auth, err := httpsAuth("https://host/repo.git")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if auth != nil {
			t.Errorf("expected nil auth (anonymous), got %T", auth)
		}
	})

	t.Run("no token + credential.helper configured → ErrAuthRequiresHelper", func(t *testing.T) {
		// Point HOME at a fresh dir with a .gitconfig that sets a helper.
		// We need git on PATH for the check itself; gate the test on that.
		homeDir := t.TempDir()
		gitconfig := filepath.Join(homeDir, ".gitconfig")
		if err := os.WriteFile(gitconfig, []byte("[credential]\n\thelper = store\n"), 0644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", homeDir)
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("GIT_CONFIG_GLOBAL", gitconfig)

		if !gitOnPath() {
			t.Skip("git not on PATH; credential.helper detection requires it")
		}

		_, err := httpsAuth("https://host/repo.git")
		if !errors.Is(err, ErrAuthRequiresHelper) {
			t.Errorf("expected ErrAuthRequiresHelper, got %v", err)
		}
	})
}

func TestGitAuth(t *testing.T) {
	cases := []struct {
		name   string
		url    string
		wantNil bool
		wantErr error
	}{
		{"file path returns nil auth", "/tmp/bare", true, nil},
		{"file:// returns nil auth", "file:///tmp/bare", true, nil},
		{"unknown scheme returns ErrAuthRequiresHelper", "ftp://x/y", false, ErrAuthRequiresHelper},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			auth, err := gitAuth(c.url)
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Errorf("expected %v, got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.wantNil && auth != nil {
				t.Errorf("expected nil auth, got %T", auth)
			}
		})
	}
}
