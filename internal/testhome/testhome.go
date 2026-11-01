// Package testhome runs a package's tests with a throwaway home directory, so
// no test can read or write the user's real ~/.ssh or ~/.config/sshgo.
package testhome

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Run points HOME (and its Windows and XDG equivalents) to a temp dir, runs
// the tests and returns their exit code. Use it from TestMain.
func Run(m *testing.M) int {
	home, err := os.MkdirTemp("", "sshgo-test-home-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(home)
	for k, v := range map[string]string{
		"HOME":            home,
		"USERPROFILE":     home,
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"APPDATA":         filepath.Join(home, "AppData"),
	} {
		os.Setenv(k, v)
	}
	return m.Run()
}
