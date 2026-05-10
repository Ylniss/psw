package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"

	"github.com/ylniss/psw/internal/ui"
)

// shouldUseRemote reports whether fetch/push are allowed.
// False when: PSW_GIT=0, PSW_GIT_REMOTE=0, or no remote in config.
func shouldUseRemote() bool {
	if os.Getenv("PSW_GIT") == "0" {
		return false
	}
	if os.Getenv("PSW_GIT_REMOTE") == "0" {
		return false
	}
	if !Paths.gitRepoExists {
		return false
	}
	return AppConfig.Remote != ""
}

// GitFetch fetches origin/<branch>. Tries go-git first; falls back to shell
// `git fetch` when auth requires a helper and `git` is on PATH.
func GitFetch() error {
	if !shouldUseRemote() {
		return nil
	}
	if err := ensureGitRemote(); err != nil {
		return fmt.Errorf("ensure remote: %w", err)
	}
	branch, err := detectBranch()
	if err != nil {
		return err
	}
	err = ui.WithSpinner("Pulling from remote", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), gitNetworkTimeout)
		defer cancel()
		goGitErr := gitFetchPureGo(ctx, branch)
		if !shouldFallbackToShell(goGitErr) {
			return goGitErr
		}
		if !gitOnPath() {
			return goGitErr
		}
		slog.Debug("falling back to shell git fetch", "go_git_err", goGitErr)
		out, shellErr := runGitNetwork("fetch", "origin", branch)
		if shellErr != nil {
			return fmt.Errorf("%s", strings.TrimSpace(out))
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("git fetch %s: %w", redactURL(AppConfig.Remote), err)
	}
	slog.Debug("git fetch ok", "remote", redactURL(AppConfig.Remote), "branch", branch)
	return nil
}

func gitFetchPureGo(ctx context.Context, branch string) error {
	repo, err := openRepo()
	if err != nil {
		return err
	}
	auth, err := gitAuth(AppConfig.Remote)
	if err != nil {
		return err
	}
	refspec := config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/remotes/origin/%s", branch, branch))
	err = repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{refspec},
		Auth:       auth,
	})
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

// GitPush pushes the current branch to origin. Errors print yellow to stderr;
// the function never returns an error.
func GitPush() {
	if !shouldUseRemote() {
		return
	}
	if err := ensureGitRemote(); err != nil {
		Warn("git remote setup failed: %v", err)
		return
	}
	branch, err := detectBranch()
	if err != nil {
		Warn("git push: %v", err)
		return
	}
	err = ui.WithSpinner("Pushing to remote", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), gitNetworkTimeout)
		defer cancel()
		goGitErr := gitPushPureGo(ctx, branch)
		if !shouldFallbackToShell(goGitErr) {
			return goGitErr
		}
		if !gitOnPath() {
			return goGitErr
		}
		slog.Debug("falling back to shell git push", "go_git_err", goGitErr)
		out, shellErr := runGitNetwork("push", "origin", branch)
		if shellErr != nil {
			return fmt.Errorf("%s", strings.TrimSpace(out))
		}
		return nil
	})
	if err != nil {
		slog.Debug("git push failed", "remote", redactURL(AppConfig.Remote), "branch", branch, "err", err)
		Warn("git push to %s failed: %v", redactURL(AppConfig.Remote), err)
		return
	}
	slog.Debug("git push ok", "remote", redactURL(AppConfig.Remote), "branch", branch)
}

func gitPushPureGo(ctx context.Context, branch string) error {
	repo, err := openRepo()
	if err != nil {
		return err
	}
	auth, err := gitAuth(AppConfig.Remote)
	if err != nil {
		return err
	}
	refspec := config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch))
	err = repo.PushContext(ctx, &git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{refspec},
		Auth:       auth,
	})
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}
