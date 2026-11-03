// Package secret keeps host passwords in the OS keyring (macOS Keychain,
// Secret Service on Linux, Windows Credential Manager) and hands them to ssh
// through SSH_ASKPASS, so they are never written to the SSH config or to disk.
package secret

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const service = "sshin"

// oldService is where sshgo (the old name of sshin) kept passwords
const oldService = "sshgo"

// shortcut: passwords are keyed by host name only, so two config files (-c)
// with the same host name share one password; key by file too if that matters.

// Get returns the saved password of a host, or "" when there is none.
// A password saved by sshgo is moved to sshin the first time it is read.
func Get(host string) (string, error) {
	pw, err := keyring.Get(service, host)
	if !errors.Is(err, keyring.ErrNotFound) {
		return pw, err
	}
	pw, err = keyring.Get(oldService, host)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if keyring.Set(service, host, pw) == nil {
		_ = keyring.Delete(oldService, host)
	}
	return pw, nil
}

// Set saves the password of a host
func Set(host, password string) error {
	return keyring.Set(service, host, password)
}

// Delete removes the saved password of a host, if any
// (also the sshgo copy, so Get cannot bring it back)
func Delete(host string) error {
	for _, svc := range []string{service, oldService} {
		if err := keyring.Delete(svc, host); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return err
		}
	}
	return nil
}

// UpdateBlock applies an edit of a Host block to the saved passwords.
// oldNames are the names the block had, names the ones it has now.
// A non-empty password is saved for every name; remove deletes them all;
// otherwise the password of from is copied to new names (a rename).
// Names dropped from the block lose their password.
func UpdateBlock(oldNames, names []string, from, password string, remove bool) error {
	kept := map[string]bool{}
	for _, n := range names {
		kept[n] = true
	}

	switch {
	case remove:
		for _, n := range append(oldNames, names...) {
			if err := Delete(n); err != nil {
				return err
			}
		}
		return nil
	case password == "":
		pw, err := Get(from)
		if err != nil {
			return err
		}
		password = pw
	}

	if password != "" {
		for _, n := range names {
			if err := Set(n, password); err != nil {
				return err
			}
		}
	}
	for _, n := range oldNames {
		if !kept[n] {
			if err := Delete(n); err != nil {
				return err
			}
		}
	}
	return nil
}
