package connectivity

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	osuser "os/user"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chaad-grayson/sshin/internal/config"
)

// PingStatus represents the connectivity status of an SSH host
type PingStatus int

const (
	StatusUnknown PingStatus = iota
	StatusConnecting
	StatusOnline
	StatusOffline
)

func (s PingStatus) String() string {
	switch s {
	case StatusUnknown:
		return "unknown"
	case StatusConnecting:
		return "connecting"
	case StatusOnline:
		return "online"
	case StatusOffline:
		return "offline"
	}
	return "unknown"
}

// HostPingResult represents the result of pinging a host
type HostPingResult struct {
	HostName string
	Status   PingStatus
	Error    error
	Duration time.Duration
}

// PingManager manages SSH connectivity checks for multiple hosts
type PingManager struct {
	results    map[string]*HostPingResult
	mutex      sync.RWMutex
	timeout    time.Duration
	configFile string
	// proxySlots caps concurrent proxied pings: each one is a login on the jump
	// host, and a burst past sshd's MaxStartups (10 by default) gets dropped or penalized
	proxySlots chan struct{}
}

// NewPingManager creates a new ping manager with the specified timeout
func NewPingManager(timeout time.Duration, configFile string) *PingManager {
	return &PingManager{
		results:    make(map[string]*HostPingResult),
		timeout:    timeout,
		configFile: configFile,
		proxySlots: make(chan struct{}, 3),
	}
}

// GetStatus returns the current status for a host
func (pm *PingManager) GetStatus(hostName string) PingStatus {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	if result, exists := pm.results[hostName]; exists {
		return result.Status
	}
	return StatusUnknown
}

// GetResult returns the complete result for a host
func (pm *PingManager) GetResult(hostName string) (*HostPingResult, bool) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	result, exists := pm.results[hostName]
	return result, exists
}

// updateStatus updates the status for a host
func (pm *PingManager) updateStatus(hostName string, status PingStatus, err error, duration time.Duration) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.results[hostName] = &HostPingResult{
		HostName: hostName,
		Status:   status,
		Error:    err,
		Duration: duration,
	}
}

// PingHost performs an SSH connectivity check for a single host
func (pm *PingManager) PingHost(ctx context.Context, host config.SSHHost) *HostPingResult {
	start := time.Now()

	// Mark as connecting
	pm.updateStatus(host.Name, StatusConnecting, nil, 0)

	// Hosts behind a jump/proxy can't be dialed directly; open the tunnel instead.
	if host.ProxyJump != "" || host.ProxyCommand != "" {
		return pm.pingThroughProxy(ctx, host, start)
	}

	// TCP connect only: an SSH handshake or auth attempt shows up in the server's
	// auth log and can get the client banned by fail2ban.
	pingCtx, cancel := context.WithTimeout(ctx, pm.timeout)
	defer cancel()

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(pingCtx, "tcp", net.JoinHostPort(targetHostPort(host)))
	if err == nil {
		conn.Close()
	}
	return pm.finish(host.Name, err, start)
}

func targetHostPort(host config.SSHHost) (string, string) {
	hostname, port := host.Hostname, host.Port
	if hostname == "" {
		hostname = host.Name
	}
	if port == "" {
		port = "22"
	}
	return hostname, port
}

func (pm *PingManager) finish(name string, err error, start time.Time) *HostPingResult {
	status := StatusOnline
	if err != nil {
		status = StatusOffline
	}
	duration := time.Since(start)
	pm.updateStatus(name, status, err, duration)
	return &HostPingResult{HostName: name, Status: status, Error: err, Duration: duration}
}

// pingThroughProxy opens a raw stream to the target through its ProxyJump or
// ProxyCommand and waits for the SSH banner. It never authenticates to the
// target, so the target's sshd logs no failed login.
func (pm *PingManager) pingThroughProxy(ctx context.Context, host config.SSHHost, start time.Time) *HostPingResult {
	select {
	case pm.proxySlots <- struct{}{}:
		defer func() { <-pm.proxySlots }()
	case <-ctx.Done():
		return pm.finish(host.Name, ctx.Err(), start)
	}

	pingCtx, cancel := context.WithTimeout(ctx, pm.timeout)
	defer cancel()

	cmd := pm.proxyCmd(pingCtx, host)
	stdout, err := cmd.StdoutPipe()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		return pm.finish(host.Name, err, start)
	}
	defer func() {
		cancel() // kills the proxy process
		_ = cmd.Wait()
	}()

	banner := make([]byte, 4)
	if _, err := io.ReadFull(stdout, banner); err != nil {
		return pm.finish(host.Name, err, start)
	}
	if string(banner) != "SSH-" {
		return pm.finish(host.Name, fmt.Errorf("unexpected banner %q", banner), start)
	}
	return pm.finish(host.Name, nil, start)
}

func (pm *PingManager) proxyCmd(ctx context.Context, host config.SSHHost) *exec.Cmd {
	hostname, port := targetHostPort(host)

	if host.ProxyJump != "" {
		// ProxyJump a,b means: ssh -J a -W target b. Auth happens only on the jump hosts.
		jumps := strings.Split(host.ProxyJump, ",")
		last := strings.TrimSpace(jumps[len(jumps)-1])
		if strings.Contains(last, ":") && !strings.HasPrefix(last, "ssh://") {
			last = "ssh://" + last // ssh only accepts host:port in URI form
		}
		timeoutSec := int(pm.timeout.Seconds())
		if timeoutSec < 1 {
			timeoutSec = 1
		}
		args := []string{"-q", "-o", "BatchMode=yes", "-o", fmt.Sprintf("ConnectTimeout=%d", timeoutSec)}
		if pm.configFile != "" {
			args = append(args, "-F", pm.configFile)
		}
		if len(jumps) > 1 {
			args = append(args, "-J", strings.Join(jumps[:len(jumps)-1], ","))
		}
		args = append(args, "-W", net.JoinHostPort(hostname, port), last)
		return exec.CommandContext(ctx, "ssh", args...)
	}

	user := host.User
	if user == "" {
		if u, err := osuser.Current(); err == nil {
			user = u.Username
		}
	}
	proxy := strings.NewReplacer("%%", "%", "%h", hostname, "%p", port, "%r", user, "%n", host.Name).Replace(host.ProxyCommand)
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", proxy)
	}
	return exec.CommandContext(ctx, "sh", "-c", proxy)
}

// PingAllHosts pings all hosts concurrently and returns a channel of results
func (pm *PingManager) PingAllHosts(ctx context.Context, hosts []config.SSHHost) <-chan *HostPingResult {
	resultChan := make(chan *HostPingResult, len(hosts))

	var wg sync.WaitGroup

	for _, host := range hosts {
		wg.Add(1)
		go func(h config.SSHHost) {
			defer wg.Done()
			result := pm.PingHost(ctx, h)
			select {
			case resultChan <- result:
			case <-ctx.Done():
				return
			}
		}(host)
	}

	// Close the channel when all goroutines are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	return resultChan
}
