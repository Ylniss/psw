package strg

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"

	color "github.com/TwiN/go-color"
)

var GitAuthType = struct {
	Unknown string
	Https   string
	Ssh     string
}{
	Unknown: "UNKNOWN",
	Https:   "HTTPS",
	Ssh:     "SSH",
}

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
	return gitCommit("initial main password set")
}

func gitCommit(message string) error {
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

func GitSync(message string) error {
	err := gitCommit(message)
	if err != nil {
		return err
	}

	if !AppConfig.Git.Sync {
		return nil
	}

	if Cfg.gitSyncAuthType == GitAuthType.Unknown {
		return errors.New("Git url provided in the config is not supported, only HTTPS and SSH auth type urls are allowed")
	}

	if Cfg.gitSyncAuthType == GitAuthType.Ssh {
		cmd := exec.Command("git", "pull")
		cmd.Dir = Cfg.storagePath
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("Failed to run git pull:\n%w", err)
		}

		cmd = exec.Command("git", "push")
		cmd.Dir = Cfg.storagePath
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("Failed to run git push:\n%w", err)
		}
	}

	if Cfg.gitSyncAuthType == GitAuthType.Https {
		// todo: handle writing auth token with new cmd psw config gitauth <auth_token>
		//      add condition that if auth token not configured error msg with info that it needs to be configured and package main

		// cmd := exec.Command("git", "pull")
		// cmd.Dir = Cfg.storagePath
		// if err := cmd.Run(); err != nil {
		// 	return fmt.Errorf("Failed to run git pull:\n%w", err)
		// }
		//
		// cmd = exec.Command("git", "push")
		// cmd.Dir = Cfg.storagePath
		// if err := cmd.Run(); err != nil {
		// 	return fmt.Errorf("Failed to run git push:\n%w", err)
		// }
	}

	return nil
}
