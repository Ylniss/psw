package storage

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"github.com/skeema/knownhosts"
	cryptossh "golang.org/x/crypto/ssh"
)

// ErrHostKeyChanged signals a known_hosts mismatch. Hard refusal — not in
// shouldFallbackToShell, since shell git would re-read the same file.
var ErrHostKeyChanged = errors.New("ssh host key changed")

// knownHostsPath returns ~/.ssh/known_hosts, creating ~/.ssh (0700) and the
// file (0600) if absent.
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
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0600)
	if err != nil {
		return "", fmt.Errorf("create known_hosts: %w", err)
	}
	f.Close()
	return path, nil
}

// hostKeyCallback returns an OpenSSH accept-new callback. Each call reads a
// fresh known_hosts.
func hostKeyCallback() (cryptossh.HostKeyCallback, error) {
	path, err := knownHostsPath()
	if err != nil {
		return nil, err
	}
	return buildHostKeyCallback(path)
}

// buildHostKeyCallback is the testable core; caller ensures the file exists.
func buildHostKeyCallback(path string) (cryptossh.HostKeyCallback, error) {
	db, err := knownhosts.NewDB(path)
	if err != nil {
		return nil, fmt.Errorf("parse known_hosts %s: %w", path, err)
	}
	return func(hostname string, remote net.Addr, key cryptossh.PublicKey) error {
		err := db.HostKeyCallback()(hostname, remote, key)
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
