package config

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// MigrateOldConfig copies the config directory (config.json, history,
// port forwards, backups) of sshgo, the old name of sshin, or else of sshm,
// to the sshin one the first time sshin runs.
// It returns the directory it copied from, or "" when there was nothing to do.
// The old directory is left untouched so the old tool keeps working.
func MigrateOldConfig() (string, error) {
	newDir, err := GetSSHINConfigDir()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(newDir); !os.IsNotExist(err) {
		return "", err // sshin already set up (or stat failed)
	}
	for _, name := range []string{"sshgo", "sshm"} {
		oldDir, err := appConfigDir(name)
		if err != nil {
			return "", err
		}
		if info, err := os.Stat(oldDir); err != nil || !info.IsDir() {
			continue
		}
		return oldDir, migrateDir(oldDir, newDir, name+"_history.json")
	}
	return "", nil // nothing to migrate from
}

func migrateDir(oldDir, newDir, oldHistoryName string) error {
	// Copy into a sibling temp dir and rename it at the end, so a failed copy
	// never leaves a half-filled sshin directory that would block a retry
	if err := os.MkdirAll(filepath.Dir(newDir), 0755); err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp(filepath.Dir(newDir), ".sshin-migrate-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err := copyTree(oldDir, tmpDir); err != nil {
		return err
	}
	oldHistory := filepath.Join(tmpDir, oldHistoryName)
	if _, err := os.Stat(oldHistory); err == nil {
		if err := os.Rename(oldHistory, filepath.Join(tmpDir, "sshin_history.json")); err != nil {
			return err
		}
	}
	return os.Rename(tmpDir, newDir)
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
