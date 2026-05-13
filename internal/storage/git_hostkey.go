package storage

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/skeema/knownhosts"
	cryptossh "golang.org/x/crypto/ssh"
)

// ErrHostKeyChanged signals a known_hosts mismatch. Hard refusal — do NOT
// match in shouldFallbackToShell, otherwise shell git would silently re-accept
// (or re-reject) using the same file we just consulted.
var ErrHostKeyChanged = errors.New("ssh host key changed")

// knownHostsPath returns ~/.ssh/known_hosts, creating ~/.ssh (0700) and the
// file (0600) if missing. knownhosts.New errors on a missing file.
func knownHostsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return "", fmt.Errorf("create ~/.ssh: %w", err)
	}
	path := filepath.Join(sshDir, "known_hosts")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return "", fmt.Errorf("create known_hosts: %w", err)
		}
		f.Close()
	} else if err != nil {
		return "", fmt.Errorf("stat known_hosts: %w", err)
	}
	return path, nil
}

var (
	hostKeyCallbackOnce sync.Once
	hostKeyCallback     cryptossh.HostKeyCallback
	hostKeyCallbackErr  error
)

// acceptNewHostKeyCallback returns an ssh.HostKeyCallback that pins unknown
// hosts to ~/.ssh/known_hosts on first connect (OpenSSH StrictHostKeyChecking=
// accept-new) and rejects key changes loudly. Cached for the process lifetime.
func acceptNewHostKeyCallback() (cryptossh.HostKeyCallback, error) {
	hostKeyCallbackOnce.Do(func() {
		path, err := knownHostsPath()
		if err != nil {
			hostKeyCallbackErr = err
			return
		}
		hostKeyCallback, hostKeyCallbackErr = buildAcceptNewCallback(path)
	})
	return hostKeyCallback, hostKeyCallbackErr
}

// buildAcceptNewCallback is the testable core. Caller ensures the file exists.
func buildAcceptNewCallback(path string) (cryptossh.HostKeyCallback, error) {
	cb, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("parse known_hosts %s: %w", path, err)
	}
	return func(hostname string, remote net.Addr, key cryptossh.PublicKey) error {
		err := cryptossh.HostKeyCallback(cb)(hostname, remote, key)
		if err == nil {
			return nil
		}
		if knownhosts.IsHostKeyChanged(err) {
			return fmt.Errorf("%w for %s: inspect %s and remove the stale entry if you trust the new key", ErrHostKeyChanged, hostname, path)
		}
		if knownhosts.IsHostUnknown(err) {
			f, oerr := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
			if oerr != nil {
				return fmt.Errorf("append to %s: %w", path, oerr)
			}
			defer f.Close()
			if werr := knownhosts.WriteKnownHost(f, hostname, remote, key); werr != nil {
				return fmt.Errorf("write known_hosts entry: %w", werr)
			}
			slog.Debug("pinned new ssh host key", "host", hostname, "type", key.Type())
			return nil
		}
		return err
	}, nil
}
