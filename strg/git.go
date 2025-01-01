package strg

import (
	"fmt"
	"os/exec"
	"path/filepath"

	color "github.com/TwiN/go-color"
)

func initGitRepoIfNotExists() error {
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

	fmt.Printf(color.InGreen("Initilizing git repository in %s\n"), Cfg.storagePath)

	cmd := exec.Command("git", "init")
	cmd.Dir = Cfg.storagePath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Failed to run git init:\n%w", err)
	}

	Cfg.gitRepoExists = true

	fmt.Println(color.InGreen("Making initial commit with main password set for storage"))
	fmt.Println(color.InGreen("Every add/change/remove action will also commit to repository"))
	return GitCommit("initial main password set")
}

func GitCommit(message string) error {
	if !Cfg.gitRepoExists {
		return nil
	}

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = Cfg.storagePath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Failed to run git add:\n%w", err)
	}

	cmd = exec.Command("git", "commit", "--message="+message)
	cmd.Dir = Cfg.storagePath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Failed to run git commit:\n%w", err)
	}

	return nil
}
