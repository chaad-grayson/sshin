package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chaad-grayson/sshin/internal/config"
	"github.com/chaad-grayson/sshin/internal/secret"

	tea "github.com/charmbracelet/bubbletea"
)

type copyIDDoneMsg struct {
	host string
	err  error
}

// copyIDCmd runs ssh-copy-id on host, with the saved password filled in, so
// the next logins use the key. It returns nil and an error to show when
// ssh-copy-id cannot run.
func (m Model) copyIDCmd(hostName string) (tea.Cmd, error) {
	if _, err := exec.LookPath("ssh-copy-id"); err != nil {
		return nil, fmt.Errorf("ssh-copy-id not found (it comes with OpenSSH, not with Windows)")
	}
	args := []string{}
	if m.configFile != "" {
		args = append(args, "-F", m.configFile)
	}
	if pub := hostPublicKey(hostName, m.configFile); pub != "" {
		args = append(args, "-i", pub)
	}
	c := exec.Command("ssh-copy-id", append(args, hostName)...)
	c.Env = secret.SSHEnv(m.configFile, hostName)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return copyIDDoneMsg{host: hostName, err: err}
	}), nil
}

// hostPublicKey returns the .pub file of the host's IdentityFile, or "" to let
// ssh-copy-id pick the default key
func hostPublicKey(hostName, configFile string) string {
	var host *config.SSHHost
	var err error
	if configFile != "" {
		host, err = config.GetSSHHostFromFile(hostName, configFile)
	} else {
		host, err = config.GetSSHHost(hostName)
	}
	if err != nil || host.Identity == "" {
		return ""
	}
	pub := host.Identity + ".pub"
	if rest, ok := strings.CutPrefix(pub, "~/"); ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		pub = filepath.Join(home, rest)
	}
	if _, err := os.Stat(pub); err != nil {
		return ""
	}
	return pub
}
