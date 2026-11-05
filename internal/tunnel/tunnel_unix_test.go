//go:build !windows

package tunnel

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestRunning(t *testing.T) {
	if rds.Running() {
		t.Fatal("no socket: must be off")
	}
	sock, _ := rds.path(".sock")
	os.MkdirAll(filepath.Dir(sock), 0o700)
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Skip("unix sockets unavailable:", err)
	}
	if !rds.Running() {
		t.Error("listening socket: must be on")
	}
	l.Close()
	os.WriteFile(sock, nil, 0o600) // stale file left by a dead ssh
	if rds.Running() {
		t.Error("stale socket file: must be off")
	}
}
