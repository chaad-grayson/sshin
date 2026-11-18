package tunnel

import (
	"errors"
	"fmt"
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

// listeners returns the pids listening on the local port of t. It reads the
// socket table instead of connecting: a test connection would go through the
// tunnel to the remote service, and services such as MySQL count aborted
// connections and then block the SSH server's address.
func (t Tunnel) listeners() map[int]bool {
	out, err := exec.Command("netstat", "-ano", "-p", "TCP").Output()
	if err != nil {
		return nil
	}
	pids := listenerPIDs(string(out), t.LocalPort)
	if out6, err := exec.Command("netstat", "-ano", "-p", "TCPv6").Output(); err == nil {
		for pid := range listenerPIDs(string(out6), t.LocalPort) {
			pids[pid] = true
		}
	}
	return pids
}

// listening reports whether anything listens on the local port of t
func (t Tunnel) listening() bool {
	return len(t.listeners()) > 0
}

// Running reports whether the ssh process of t is up: the pid in its pid file
// listens on its local port (which also rules out a pid Windows reused)
func (t Tunnel) Running() bool {
	pid, err := t.pid()
	if err != nil {
		return false
	}
	return t.listeners()[pid]
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
				if t.listeners()[c.Process.Pid] {
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
