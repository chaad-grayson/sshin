//go:build !windows

package tunnel

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

// On Linux and macOS each running tunnel is an ssh ControlMaster that went to
// the background by itself (-f): its socket tells whether it runs, and stops
// it with "ssh -O exit".

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
func (t Tunnel) StartCmd(configFile string) (*Starter, error) {
	log, err := t.prepare()
	if err != nil {
		return nil, err
	}
	sock, err := t.path(".sock")
	if err != nil {
		return nil, err
	}
	_ = os.Remove(sock) // a stale socket would block -M
	c := exec.Command("ssh", t.args(configFile, log, "-f", "-M", "-S", sock, "-o", "ControlPersist=no")...)
	return &Starter{Cmd: c, run: c.Run}, nil
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
