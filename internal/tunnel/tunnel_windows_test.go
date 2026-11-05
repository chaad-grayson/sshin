package tunnel

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestRunningWindows(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	tun := rds
	tun.LocalPort = l.Addr().(*net.TCPAddr).Port

	if tun.Running() {
		t.Fatal("no pid file: must be off")
	}
	pidFile, _ := tun.path(".pid")
	os.MkdirAll(filepath.Dir(pidFile), 0o700)
	os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o600)
	if !tun.Running() {
		t.Error("live pid and listening port: must be on")
	}
	if _, err := tun.StartCmd(""); err == nil {
		t.Error("StartCmd on a busy port must fail")
	}
	l.Close()
	if tun.Running() {
		t.Error("port closed: must be off")
	}
	if tun.Stop() == nil {
		t.Error("Stop of a tunnel that is off must fail, not kill the pid")
	}
}
