package config

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// MigrateFromSSHM copies the sshm config directory (config.json, history,
// port forwards, backups) to the sshgo one the first time sshgo runs.
// It returns the sshm directory it copied from, or "" when there was nothing to do.
// The sshm directory is left untouched so sshm keeps working.
func MigrateFromSSHM() (string, error) {
	newDir, err := GetSSHGOConfigDir()
	if err != nil {
		return "", err
	}
	oldDir, err := appConfigDir("sshm")
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(newDir); !os.IsNotExist(err) {
		return "", err // sshgo already set up (or stat failed)
	}
	if info, err := os.Stat(oldDir); err != nil || !info.IsDir() {
		return "", nil // no sshm install to migrate from
	}

	// Copy into a sibling temp dir and rename it at the end, so a failed copy
	// never leaves a half-filled sshgo directory that would block a retry
	if err := os.MkdirAll(filepath.Dir(newDir), 0755); err != nil {
		return "", err
	}
	tmpDir, err := os.MkdirTemp(filepath.Dir(newDir), ".sshgo-migrate-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	if err := copyTree(oldDir, tmpDir); err != nil {
		return "", err
	}
	oldHistory := filepath.Join(tmpDir, "sshm_history.json")
	if _, err := os.Stat(oldHistory); err == nil {
		if err := os.Rename(oldHistory, filepath.Join(tmpDir, "sshgo_history.json")); err != nil {
			return "", err
		}
	}
	if err := os.Rename(tmpDir, newDir); err != nil {
		return "", err
	}
	return oldDir, nil
}

// copyTree copies the regular files and directories under src into dst,
// keeping their permissions (backups of the SSH config are 0600)
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(target, info.Mode().Perm())
		case info.Mode().IsRegular():
			return copyFile(path, target, info.Mode().Perm())
		default:
			return nil // skip symlinks, sockets and the like
		}
	})
}

func copyFile(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
