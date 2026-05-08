package storage

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	color "github.com/TwiN/go-color"
)

type LogEntry struct {
	ShortSHA string
	Time     time.Time
	Message  string
}

func IsGitRepo() (bool, error) {
	return pathExists(filepath.Join(Paths.storagePath, ".git"))
}

func GitLog() ([]LogEntry, error) {
	entries, err := gitLogEntries()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	slog.Debug("git log read", "count", len(entries))
	return entries, nil
}

func initGitRepoIfNotExists() error {
	// PSW_GIT=0 opts out of git init/commit (intended for tests/scripting)
	if os.Getenv("PSW_GIT") == "0" {
		return nil
	}

	gitRepoExists, err := pathExists(filepath.Join(Paths.storagePath, ".git"))
	if err != nil {
		return err
	}
	if gitRepoExists {
		Paths.gitRepoExists = true
		return nil
	}

	fmt.Println(color.InGreen(fmt.Sprintf("Initializing git repository in %s", Paths.storagePath)))
	if err := initRepo(); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	Paths.gitRepoExists = true

	fmt.Println(color.InGreen("Making initial commit with main password set for storage"))
	fmt.Println(color.InGreen("Every add/change/remove action will also commit to repository"))
	return GitCommit("initial main password set")
}

func GitCommit(message string) error {
	if os.Getenv("PSW_GIT") == "0" {
		return nil
	}
	if !Paths.gitRepoExists {
		return nil
	}

	if err := gitAddPaths("storage.psw", "pswcfg.toml"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	err := gitCommit(message)
	if errors.Is(err, ErrSigningRequired) {
		if !gitOnPath() {
			// Data is on disk; only the commit step is missing. Warn and continue, like push failures.
			printWarn("commit signing requires git on PATH (commit.gpgsign=true is set); record saved but not committed")
			return nil
		}
		if out, runErr := runGit("commit", "--message="+message); runErr != nil {
			return fmt.Errorf("git commit: %s", strings.TrimSpace(out))
		}
	} else if err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	GitPush()
	return nil
}
