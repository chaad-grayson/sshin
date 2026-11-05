package tunnel

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Windows OpenSSH has neither ControlMaster nor -f: ssh runs as a child of
// sshin that stays on after the start returns, and a pid file tells other
// sshin sessions about it.
// shortcut: the tunnel shares the console of the terminal, so closing that
// window stops it; start it detached (or as a service) if that matters.

func (t Tunnel) pid() (int, error) {
	f, err := t.path(".pid")
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(f)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// listening reports whether something accepts connections on the local port of t
func (t Tunnel) listening() bool {
	c, err := net.DialTimeout("tcp", net.JoinHostPort("localhost", strconv.Itoa(t.LocalPort)), 300*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// Running reports whether the ssh process of t is up. The port check guards
// against a pid that Windows gave to another process after ssh exited.
func (t Tunnel) Running() bool {
	pid, err := t.pid()
	if err != nil {
		return false
	}
	p, err := os.FindProcess(pid) // fails when no such process exists
	if err != nil {
		return false
	}
	p.Release()
	return t.listening()
}

// StartCmd returns the command that starts ssh for t and returns once the
// local port listens, leaving ssh on. Prompts ssh may need are asked on the
// terminal meanwhile. Its messages go to a log file: LastError reads them
// when it fails.
func (t Tunnel) StartCmd(configFile string) (*Starter, error) {
	if t.listening() {
		return nil, fmt.Errorf("localhost:%d is already in use", t.LocalPort)
	}
	log, err := t.prepare()
	if err != nil {
		return nil, err
	}
	pidFile, err := t.path(".pid")
	if err != nil {
		return nil, err
	}
	// -n: ssh must not read keys typed in sshin once it runs in the background
	c := exec.Command("ssh", t.args(configFile, log, "-n")...)
	// its own process group, so Ctrl+C in the terminal does not stop it
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	s := &Starter{Cmd: c}
	s.run = func() error {
		if err := c.Start(); err != nil {
			return err
		}
		if err := os.WriteFile(pidFile, []byte(strconv.Itoa(c.Process.Pid)), 0o600); err != nil {
			_ = c.Process.Kill()
			return err
		}
		exited := make(chan error, 1)
		go func() { exited <- c.Wait() }()
		tick := time.NewTicker(200 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case err := <-exited:
				_ = os.Remove(pidFile)
				if err == nil {
					err = errors.New("ssh exited")
				}
				return err
			case <-tick.C:
				if t.listening() {
					return nil
				}
			}
		}
	}
	return s, nil
}

// Stop stops the ssh process of t
func (t Tunnel) Stop() error {
	// Running first: never kill a process that took over a stale pid
	if !t.Running() {
		return errors.New("not running")
	}
	pid, err := t.pid()
	if err != nil {
		return err
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	defer p.Release()
	if err := p.Kill(); err != nil {
		return err
	}
	if f, err := t.path(".pid"); err == nil {
		_ = os.Remove(f)
	}
	return nil
}
