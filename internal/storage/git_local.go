package storage

import (
	"cmp"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Repo handle, lazy-initialized on first successful PlainOpen.
var (
	cachedRepoMu sync.Mutex
	cachedRepo   *git.Repository
)

func openRepo() (*git.Repository, error) {
	cachedRepoMu.Lock()
	defer cachedRepoMu.Unlock()
	if cachedRepo != nil {
		return cachedRepo, nil
	}
	repo, err := git.PlainOpen(Paths.storagePath)
	if err != nil {
		return nil, err
	}
	cachedRepo = repo
	return repo, nil
}

// initRepo creates a fresh repo with `main` as the default branch.
func initRepo() error {
	_, err := git.PlainInitWithOptions(Paths.storagePath, &git.PlainInitOptions{
		InitOptions: git.InitOptions{
			DefaultBranch: plumbing.NewBranchReferenceName("main"),
		},
		Bare: false,
	})
	return err
}

// gitAddPaths stages files relative to Paths.storagePath.
func gitAddPaths(paths ...string) error {
	repo, err := openRepo()
	if err != nil {
		return err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	for _, p := range paths {
		if _, err := wt.Add(p); err != nil {
			return fmt.Errorf("git add %s: %w", p, err)
		}
	}
	return nil
}

// ErrSigningRequired signals that commit signing can't be done by go-git.
// Caller falls back to shell git if available.
var ErrSigningRequired = errors.New("commit signing requires shell git")

// gitCommit creates a commit. Returns ErrSigningRequired when commit.gpgsign is true.
func gitCommit(msg string) error {
	repo, err := openRepo()
	if err != nil {
		return err
	}
	cfg, err := repo.ConfigScoped(config.SystemScope)
	if err != nil {
		return err
	}
	if strings.EqualFold(cfg.Raw.Section("commit").Option("gpgsign"), "true") {
		return ErrSigningRequired
	}
	sig := authorSignature(cfg)
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	hash, err := wt.Commit(msg, &git.CommitOptions{Author: sig, Committer: sig})
	if err != nil {
		return err
	}
	slog.Debug("git commit ok", "hash", hash.String()[:7], "msg", msg)
	return nil
}

func authorSignature(cfg *config.Config) *object.Signature {
	name := envOrConfig("GIT_AUTHOR_NAME", cfg, "user", "name", "psw")
	email := envOrConfig("GIT_AUTHOR_EMAIL", cfg, "user", "email", "psw@localhost")
	return &object.Signature{Name: name, Email: email, When: time.Now()}
}

func envOrConfig(envKey string, cfg *config.Config, section, key, fallback string) string {
	return cmp.Or(os.Getenv(envKey), cfg.Raw.Section(section).Option(key), fallback)
}

func detectBranch() (string, error) {
	repo, err := openRepo()
	if err != nil {
		return "", err
	}
	head, err := repo.Head()
	if err != nil {
		return "", err
	}
	if !head.Name().IsBranch() {
		return "", fmt.Errorf("HEAD is not on a branch (detached?): %s", head.Name())
	}
	return head.Name().Short(), nil
}

func revParse(ref string) (string, error) {
	repo, err := openRepo()
	if err != nil {
		return "", err
	}
	h, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return "", fmt.Errorf("rev-parse %s: %w", ref, err)
	}
	return h.String(), nil
}

// isAncestor reports whether ancestor is an ancestor of descendant.
func isAncestor(ancestor, descendant string) bool {
	repo, err := openRepo()
	if err != nil {
		slog.Debug("isAncestor open repo failed", "err", err)
		return false
	}
	ancestorCommit, err := commitFromRev(repo, ancestor)
	if err != nil {
		slog.Debug("isAncestor resolve failed", "ref", ancestor, "err", err)
		return false
	}
	descendantCommit, err := commitFromRev(repo, descendant)
	if err != nil {
		slog.Debug("isAncestor resolve failed", "ref", descendant, "err", err)
		return false
	}
	yes, err := ancestorCommit.IsAncestor(descendantCommit)
	if err != nil {
		slog.Debug("isAncestor walk failed", "ancestor", ancestor, "descendant", descendant, "err", err)
		return false
	}
	return yes
}

func gitMergeBase(refA, refB string) (string, error) {
	repo, err := openRepo()
	if err != nil {
		return "", err
	}
	commitA, err := commitFromRev(repo, refA)
	if err != nil {
		return "", err
	}
	commitB, err := commitFromRev(repo, refB)
	if err != nil {
		return "", err
	}
	bases, err := commitA.MergeBase(commitB)
	if err != nil {
		return "", err
	}
	if len(bases) == 0 {
		return "", fmt.Errorf("merge-base: no common ancestor for %s and %s", refA, refB)
	}
	return bases[0].Hash.String(), nil
}

func commitFromRev(repo *git.Repository, ref string) (*object.Commit, error) {
	h, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", ref, err)
	}
	return repo.CommitObject(*h)
}

// gitShowBlob returns the bytes of <path> at git revision <ref>.
func gitShowBlob(ref, path string) ([]byte, error) {
	repo, err := openRepo()
	if err != nil {
		return nil, err
	}
	commit, err := commitFromRev(repo, ref)
	if err != nil {
		return nil, fmt.Errorf("show %s: %w", ref, err)
	}
	file, err := commit.File(path)
	if err != nil {
		return nil, err
	}
	reader, err := file.Reader()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

// gitLogEntries returns commits oldest-first.
func gitLogEntries() ([]LogEntry, error) {
	repo, err := openRepo()
	if err != nil {
		return nil, err
	}
	iter, err := repo.Log(&git.LogOptions{Order: git.LogOrderCommitterTime})
	if err != nil {
		return nil, err
	}
	var entries []LogEntry
	if err := iter.ForEach(func(c *object.Commit) error {
		firstLine, _, _ := strings.Cut(c.Message, "\n")
		entries = append(entries, LogEntry{
			ShortSHA: c.Hash.String()[:7],
			Time:     c.Committer.When,
			Message:  firstLine,
		})
		return nil
	}); err != nil {
		return nil, err
	}
	slices.Reverse(entries)
	return entries, nil
}

// parseSHA1Hash validates a 40-hex SHA-1 string. plumbing.NewHash silently
// returns ZeroHash on malformed input; this catches that.
func parseSHA1Hash(sha string) (plumbing.Hash, error) {
	if len(sha) != 40 {
		return plumbing.ZeroHash, fmt.Errorf("invalid sha length %d (want 40): %q", len(sha), sha)
	}
	if _, err := hex.DecodeString(sha); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("invalid sha hex: %q", sha)
	}
	return plumbing.NewHash(sha), nil
}

// gitStorageMergeCommit stages storage.psw and creates a two-parent merge commit on the current branch.
func gitStorageMergeCommit(localSHA, remoteSHA, msg string) error {
	localHash, err := parseSHA1Hash(localSHA)
	if err != nil {
		return err
	}
	remoteHash, err := parseSHA1Hash(remoteSHA)
	if err != nil {
		return err
	}
	repo, err := openRepo()
	if err != nil {
		return err
	}
	cfg, err := repo.ConfigScoped(config.SystemScope)
	if err != nil {
		return err
	}
	sig := authorSignature(cfg)

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	if _, err := wt.Add(Paths.storageFileName); err != nil {
		return fmt.Errorf("git add merged: %w", err)
	}
	_, err = wt.Commit(msg, &git.CommitOptions{
		Author:    sig,
		Committer: sig,
		Parents:   []plumbing.Hash{localHash, remoteHash},
	})
	if err != nil {
		return fmt.Errorf("git commit merge: %w", err)
	}
	return nil
}

// gitFastForward updates the current branch ref to remoteSHA and resets
// the worktree to match.
func gitFastForward(remoteSHA string) error {
	remoteHash, err := parseSHA1Hash(remoteSHA)
	if err != nil {
		return err
	}
	repo, err := openRepo()
	if err != nil {
		return err
	}
	head, err := repo.Head()
	if err != nil {
		return err
	}
	if !head.Name().IsBranch() {
		return fmt.Errorf("HEAD is not on a branch (detached?): %s", head.Name())
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	if err := wt.Reset(&git.ResetOptions{Mode: git.HardReset, Commit: remoteHash}); err != nil {
		return fmt.Errorf("fast-forward reset: %w", err)
	}
	return nil
}

// ensureGitRemote idempotently sets the `origin` remote to AppConfig.Remote.
func ensureGitRemote() error {
	repo, err := openRepo()
	if err != nil {
		return err
	}
	if existing, err := repo.Remote("origin"); err == nil {
		urls := existing.Config().URLs
		if len(urls) > 0 && urls[0] == AppConfig.Remote {
			return nil
		}
		if err := repo.DeleteRemote("origin"); err != nil {
			return fmt.Errorf("delete remote: %w", err)
		}
	} else if !errors.Is(err, git.ErrRemoteNotFound) {
		return fmt.Errorf("get remote: %w", err)
	}
	if _, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{AppConfig.Remote},
	}); err != nil {
		return fmt.Errorf("create remote: %w", err)
	}
	return nil
}
