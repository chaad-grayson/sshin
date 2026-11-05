// Package tunnel keeps named SSH tunnels (local port forwards) that run in the
// background and can be turned on and off, also from another sshin session.
// How a tunnel runs in the background depends on the OS: see tunnel_unix.go
// and tunnel_windows.go.
package tunnel

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chaad-grayson/sshin/internal/config"
)

// Tunnel forwards localhost:LocalPort to RemoteHost:RemotePort through Host
type Tunnel struct {
	Name       string `json:"name"`
	Host       string `json:"host"` // a Host of the SSH config
	LocalPort  int    `json:"local_port"`
	RemoteHost string `json:"remote_host"`
	RemotePort int    `json:"remote_port"`
	// LastActive is when the tunnel was last started or stopped
	LastActive time.Time `json:"last_active,omitzero"`
}

// 32 at most: macOS limits a unix socket path to 104 bytes
var validName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)

// Validate checks the fields of t
func (t Tunnel) Validate() error {
	switch {
	case !validName.MatchString(t.Name):
		return errors.New("name: up to 32 letters, digits, '.', '_' and '-'")
	case t.Host == "":
		return errors.New("choose an SSH host")
	case t.LocalPort < 1 || t.LocalPort > 65535:
		return errors.New("local port must be 1-65535")
	case t.RemoteHost == "" || strings.ContainsAny(t.RemoteHost, " :"):
		return errors.New("remote host: a hostname or an IPv4 address")
	case t.RemotePort < 1 || t.RemotePort > 65535:
		return errors.New("remote port must be 1-65535")
	}
	return nil
}

// Forward is the -L argument of t
func (t Tunnel) Forward() string {
	return fmt.Sprintf("%d:%s:%d", t.LocalPort, t.RemoteHost, t.RemotePort)
}

func dir() (string, error) {
	d, err := config.GetSSHINConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "tunnels"), nil
}

func storeFile() (string, error) {
	d, err := config.GetSSHINConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "tunnels.json"), nil
}

// Load returns the saved tunnels, none when nothing is saved yet
func Load() ([]Tunnel, error) {
	f, err := storeFile()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(f)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var tunnels []Tunnel
	if err := json.Unmarshal(data, &tunnels); err != nil {
		return nil, fmt.Errorf("%s: %w", f, err)
	}
	return tunnels, nil
}

// Save replaces the saved tunnels
func Save(tunnels []Tunnel) error {
	f, err := storeFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tunnels, "", "  ")
	if err != nil {
		return err
	}
	tmp := f + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, f)
}

// Touch sets the LastActive time of the saved tunnel named name to now
func Touch(name string) error {
	list, err := Load()
	if err != nil {
		return err
	}
	for i := range list {
		if list[i].Name == name {
			list[i].LastActive = time.Now()
			return Save(list)
		}
	}
	return nil
}

func (t Tunnel) path(ext string) (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, t.Name+ext), nil
}

// Starter is the ssh command that opens a tunnel. It runs in the foreground,
// so ssh can ask its prompts on the terminal, until the tunnel is up, then
// the tunnel stays on in the background. It fits tea.Exec.
type Starter struct {
	*exec.Cmd
	run func() error
}

func (s *Starter) Run() error { return s.run() }

func (s *Starter) SetStdin(r io.Reader) {
	if s.Stdin == nil {
		s.Stdin = r
	}
}

func (s *Starter) SetStdout(w io.Writer) {
	if s.Stdout == nil {
		s.Stdout = w
	}
}

func (s *Starter) SetStderr(w io.Writer) {
	if s.Stderr == nil {
		s.Stderr = w
	}
}

// prepare makes the tunnels dir and empties the log of t, so each run starts
// with an empty log. It returns the log path.
func (t Tunnel) prepare() (string, error) {
	log, err := t.path(".log")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(log), 0o700); err != nil {
		return "", err
	}
	return log, os.WriteFile(log, nil, 0o600)
}

func (t Tunnel) args(configFile, log string, extra ...string) []string {
	var args []string
	if configFile != "" {
		args = append(args, "-F", configFile)
	}
	args = append(args, extra...)
	return append(args,
		"-N", "-E", log,
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
		"-o", "ExitOnForwardFailure=yes",
		"-L", t.Forward(),
		t.Host,
	)
}

// LastError returns what ssh logged on the last start, for a failed one
func (t Tunnel) LastError() string {
	log, err := t.path(".log")
	if err != nil {
		return ""
	}
	data, _ := os.ReadFile(log)
	var lines []string
	for _, l := range strings.Split(string(data), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "Warning: Permanently added") {
			lines = append(lines, l)
		}
	}
	if len(lines) > 2 {
		lines = lines[len(lines)-2:]
	}
	return strings.Join(lines, " ")
}

// ParsePort parses a port typed in a form
func ParsePort(s string) int {
	p, _ := strconv.Atoi(strings.TrimSpace(s))
	return p
}
