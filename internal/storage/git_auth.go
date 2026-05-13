package storage

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// ErrShellGitNeeded signals the caller to fall back to shell git: credential
// helper needed, key passphrase, or unsupported scheme.
var ErrShellGitNeeded = errors.New("auth method requires shell git or credential helper")

// ErrSigningRequired signals that commit signing can't be done by go-git.
// Caller falls back to shell git if available.
var ErrSigningRequired = errors.New("commit signing requires shell git")

// ErrRemoteCredentialsInURL signals a remote URL with embedded userinfo
// (https://user:pass@host/). Hard refusal — not retried via shell git.
var ErrRemoteCredentialsInURL = errors.New("credentials embedded in remote URL are not allowed; use ssh, ssh-agent, or git's credential.helper")

type remoteKind int

const (
	remoteSSH remoteKind = iota
	remoteHTTPS
	remoteFile
	remoteUnknown
)

func classifyRemote(remoteURL string) remoteKind {
	if strings.HasPrefix(remoteURL, "git@") || strings.HasPrefix(remoteURL, "ssh://") {
		return remoteSSH
	}
	if strings.HasPrefix(remoteURL, "https://") || strings.HasPrefix(remoteURL, "http://") {
		return remoteHTTPS
	}
	if strings.HasPrefix(remoteURL, "file://") || filepath.IsAbs(remoteURL) {
		return remoteFile
	}
	return remoteUnknown
}

// gitAuth picks the AuthMethod for remoteURL. Returns ErrShellGitNeeded
// when only shell git can handle the auth.
func gitAuth(remoteURL string) (transport.AuthMethod, error) {
	switch classifyRemote(remoteURL) {
	case remoteFile:
		return nil, nil
	case remoteSSH:
		return sshAuth()
	case remoteHTTPS:
		return httpsAuth(remoteURL)
	case remoteUnknown:
		return nil, fmt.Errorf("%w: unsupported remote scheme: %s", ErrShellGitNeeded, redactURL(remoteURL))
	}
	panic("unreachable: remoteKind exhausted")
}

func sshAuth() (transport.AuthMethod, error) {
	hostKeyCB, err := hostKeyCallback()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrShellGitNeeded, err)
	}
	// ssh-agent only when it has keys; empty agent fails handshake before keyfile fallback.
	if os.Getenv("SSH_AUTH_SOCK") != "" {
		if agentAuth, err := ssh.NewSSHAgentAuth("git"); err == nil {
			if signers, err := agentAuth.Callback(); err == nil && len(signers) > 0 {
				agentAuth.HostKeyCallback = hostKeyCB
				return agentAuth, nil
			}
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrShellGitNeeded, err)
	}
	for _, name := range []string{"id_ed25519", "id_rsa"} {
		path := filepath.Join(home, ".ssh", name)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		keyAuth, err := ssh.NewPublicKeysFromFile("git", path, "")
		if err != nil {
			continue
		}
		keyAuth.HostKeyCallback = hostKeyCB
		return keyAuth, nil
	}
	return nil, fmt.Errorf("%w: no usable SSH key for go-git", ErrShellGitNeeded)
}

func httpsAuth(remoteURL string) (transport.AuthMethod, error) {
	u, err := url.Parse(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("%w: parse url: %v", ErrShellGitNeeded, err)
	}
	if u.User != nil {
		return nil, fmt.Errorf("%w: %s", ErrRemoteCredentialsInURL, redactURL(remoteURL))
	}
	if hasCredentialHelper() {
		return nil, fmt.Errorf("%w: credential.helper configured", ErrShellGitNeeded)
	}
	return nil, nil
}

func hasCredentialHelper() bool {
	if !gitOnPath() {
		return false
	}
	out, err := exec.Command("git", "config", "--get-all", "credential.helper").CombinedOutput()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}
