// Package tunnel keeps named SSH tunnels (local port forwards) that run in the
// background and can be turned on and off, also from another sshin session.
// Each running tunnel is an ssh ControlMaster: its socket tells whether it
// runs, and stops it with "ssh -O exit".
package tunnel

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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

var validName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// Validate checks the fields of t
func (t Tunnel) Validate() error {
	switch {
	case !validName.MatchString(t.Name):
		return errors.New("name: letters, digits, '.', '_' and '-' only")
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

// Running reports whether the ssh process of t is up
func (t Tunnel) Running() bool {
	sock, err := t.path(".sock")
	if err != nil {
		return false
	}
	c, err := net.DialTimeout("unix", sock, 300*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// StartCmd returns the ssh command that opens t and goes to the background
// once connected (-f), so prompts it may need are asked on the terminal first.
// Its messages go to a log file: LastError reads them when it fails.
func (t Tunnel) StartCmd(configFile string) (*exec.Cmd, error) {
	if runtime.GOOS == "windows" {
		// shortcut: needs ssh ControlMaster, which Windows OpenSSH lacks; track the pid there if needed
		return nil, errors.New("tunnels are not supported on Windows yet")
	}
	sock, err := t.path(".sock")
	if err != nil {
		return nil, err
	}
	log, err := t.path(".log")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(sock), 0o700); err != nil {
		return nil, err
	}
	// Start each run with an empty log; a stale socket would block -M
	if err := os.WriteFile(log, nil, 0o600); err != nil {
		return nil, err
	}
	_ = os.Remove(sock)
	return exec.Command("ssh", t.args(configFile, sock, log)...), nil
}

func (t Tunnel) args(configFile, sock, log string) []string {
	var args []string
	if configFile != "" {
		args = append(args, "-F", configFile)
	}
	return append(args,
		"-f", "-N", "-M", "-S", sock, "-E", log,
		"-o", "ControlPersist=no",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
		"-o", "ExitOnForwardFailure=yes",
		"-L", t.Forward(),
		t.Host,
	)
}

// Stop stops the ssh process of t
func (t Tunnel) Stop() error {
	sock, err := t.path(".sock")
	if err != nil {
		return err
	}
	out, err := exec.Command("ssh", "-S", sock, "-O", "exit", t.Host).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
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
