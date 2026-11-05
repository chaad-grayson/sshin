package tunnel

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chaad-grayson/sshin/internal/testhome"
)

func TestMain(m *testing.M) { os.Exit(testhome.Run(m)) }

var rds = Tunnel{Name: "rds-prod", Host: "vm", LocalPort: 3307, RemoteHost: "db.example.rds.amazonaws.com", RemotePort: 3306}

func TestValidate(t *testing.T) {
	if err := rds.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := []func(*Tunnel){
		func(t *Tunnel) { t.Name = "has space" },
		func(t *Tunnel) { t.Name = strings.Repeat("a", 33) },
		func(t *Tunnel) { t.Host = "" },
		func(t *Tunnel) { t.LocalPort = 0 },
		func(t *Tunnel) { t.RemoteHost = "a:b" },
		func(t *Tunnel) { t.RemotePort = 70000 },
	}
	for i, f := range bad {
		tun := rds
		f(&tun)
		if tun.Validate() == nil {
			t.Errorf("case %d should fail", i)
		}
	}
}

func TestArgs(t *testing.T) {
	got := rds.args("/c", "/l.log", "-f", "-M", "-S", "/s.sock", "-o", "ControlPersist=no")
	want := []string{"-F", "/c", "-f", "-M", "-S", "/s.sock", "-o", "ControlPersist=no",
		"-N", "-E", "/l.log", "-o", "ServerAliveInterval=30", "-o", "ServerAliveCountMax=3",
		"-o", "ExitOnForwardFailure=yes", "-L", "3307:db.example.rds.amazonaws.com:3306", "vm"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("args =\n%v\nwant\n%v", got, want)
	}
}

func TestSaveLoad(t *testing.T) {
	if got, err := Load(); err != nil || got != nil {
		t.Fatalf("Load() before any save = %v, %v", got, err)
	}
	if err := Save([]Tunnel{rds}); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil || !reflect.DeepEqual(got, []Tunnel{rds}) {
		t.Errorf("Load() = %v, %v", got, err)
	}

	if err := Touch(rds.Name); err != nil {
		t.Fatal(err)
	}
	got, _ = Load()
	if len(got) != 1 || time.Since(got[0].LastActive) > time.Minute {
		t.Errorf("Touch() did not set LastActive: %v", got)
	}
}
