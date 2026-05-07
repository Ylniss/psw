package strg

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	return pathExists(filepath.Join(Cfg.storagePath, ".git"))
}

func GitLog() ([]LogEntry, error) {
	out, err := exec.Command("git", "-C", Cfg.storagePath, "log", "--reverse", "--format=%h %ct %s").Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	var entries []LogEntry
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 3 {
			continue
		}
		secs, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}
		entries = append(entries, LogEntry{
			ShortSHA: parts[0],
			Time:     time.Unix(secs, 0),
			Message:  parts[2],
		})
	}
	slog.Debug("git log read", "count", len(entries))
	return entries, nil
}

func initGitRepoIfNotExists() error {
	// PSW_GIT=0 opts out of git init/commit (intended for tests/scripting)
	if os.Getenv("PSW_GIT") == "0" {
		return nil
	}

	// Check if git is installed
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Println(color.InRed("git is not installed. It is not recommended to use psw with storage outside of git repository"))
		return nil
	}

	gitRepoExists, err := pathExists(filepath.Join(Cfg.storagePath, ".git"))
	if err != nil {
		return err
	}

	if gitRepoExists {
		Cfg.gitRepoExists = true
		return nil
	}

	msg := fmt.Sprintf("Initializing git repository in %s", Cfg.storagePath)
	fmt.Println(color.InGreen(msg))

	cmd := exec.Command("git", "init")
	cmd.Dir = Cfg.storagePath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git init: %w", err)
	}

	Cfg.gitRepoExists = true

	fmt.Println(color.InGreen("Making initial commit with main password set for storage"))
	fmt.Println(color.InGreen("Every add/change/remove action will also commit to repository"))
	return GitCommit("initial main password set")
}

func GitCommit(message string) error {
	if os.Getenv("PSW_GIT") == "0" {
		return nil
	}

	if !Cfg.gitRepoExists {
		return nil
	}

	cmd := exec.Command("git", "add", "storage.psw", "pswcfg.toml")
	cmd.Dir = Cfg.storagePath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	cmd = exec.Command("git", "commit", "--message="+message)
	cmd.Dir = Cfg.storagePath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}
