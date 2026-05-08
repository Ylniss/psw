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
	httpauth "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	cryptossh "golang.org/x/crypto/ssh"
)

// ErrAuthRequiresHelper signals the caller to fall back to shell git: credential helper needed, key passphrase, or unsupported scheme.
var ErrAuthRequiresHelper = errors.New("auth method requires shell git or credential helper")

// ErrSigningRequired signals commit signing the go-git layer can't perform. Caller falls back to git commit if available.
var ErrSigningRequired = errors.New("commit signing requires shell git")

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

// gitAuth resolves the right transport.AuthMethod for the remote URL or
// returns ErrAuthRequiresHelper if go-git can't satisfy auth pure-Go.
func gitAuth(remoteURL string) (transport.AuthMethod, error) {
	switch classifyRemote(remoteURL) {
	case remoteFile:
		return nil, nil
	case remoteSSH:
		return sshAuth()
	case remoteHTTPS:
		return httpsAuth(remoteURL)
	default:
		return nil, fmt.Errorf("%w: unsupported remote scheme: %s", ErrAuthRequiresHelper, redactURL(remoteURL))
	}
}

func sshAuth() (transport.AuthMethod, error) {
	// Use ssh-agent only when it has keys; an empty agent makes go-git's handshake fail before we'd reach keyfile fallback.
	if os.Getenv("SSH_AUTH_SOCK") != "" {
		if agentAuth, err := ssh.NewSSHAgentAuth("git"); err == nil {
			if signers, err := agentAuth.Callback(); err == nil && len(signers) > 0 {
				agentAuth.HostKeyCallback = cryptossh.InsecureIgnoreHostKey()
				return agentAuth, nil
			}
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAuthRequiresHelper, err)
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
		keyAuth.HostKeyCallback = cryptossh.InsecureIgnoreHostKey()
		return keyAuth, nil
	}
	return nil, fmt.Errorf("%w: no usable SSH key for go-git", ErrAuthRequiresHelper)
}

func httpsAuth(remoteURL string) (transport.AuthMethod, error) {
	u, err := url.Parse(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("%w: parse url: %v", ErrAuthRequiresHelper, err)
	}
	if u.User != nil {
		password, _ := u.User.Password()
		return &httpauth.BasicAuth{Username: u.User.Username(), Password: password}, nil
	}
	if hasCredentialHelper() {
		return nil, fmt.Errorf("%w: credential.helper configured", ErrAuthRequiresHelper)
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
