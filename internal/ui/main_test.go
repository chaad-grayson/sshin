package ui

import (
	"os"
	"strings"
	"testing"

	"github.com/chaad-grayson/sshin/internal/config"
	"github.com/chaad-grayson/sshin/internal/testhome"
	"github.com/chaad-grayson/sshin/internal/tunnel"
	"github.com/zalando/go-keyring"
)

// The forms read saved passwords and the config dir: keep the tests off the
// real OS keyring and the real home directory
func TestMain(m *testing.M) {
	keyring.MockInit()
	os.Exit(testhome.Run(m))
}

// The first tunnel added to an empty Tunnels tab must be shown and selectable
func TestTunnelTabFirstTunnel(t *testing.T) {
	m := NewModel(nil, "", false, "dev", true)
	m.width, m.height = 120, 40
	m.showTunnels() // no tunnel yet
	if err := tunnel.Save([]tunnel.Tunnel{{Name: "rds", Host: "vm", LocalPort: 3307, RemoteHost: "db", RemotePort: 3306}}); err != nil {
		t.Fatal(err)
	}
	m.reloadTunnels()
	if tun, ok := m.selectedTunnel(); !ok || tun.Name != "rds" {
		t.Errorf("selectedTunnel() = %v, %v", tun, ok)
	}
	if !strings.Contains(m.tunnels.table.View(), "rds") {
		t.Errorf("tunnel row not rendered:\n%s", m.tunnels.table.View())
	}
}

func TestSecurityIcons(t *testing.T) {
	m := Model{secrets: map[string]string{"a": "", "b": ""}}
	withKey := config.SSHHost{Name: "a", Identity: "~/.ssh/work"}
	noKey := config.SSHHost{Name: "b"}
	if got := m.securityIcons(withKey); got != iconKey {
		t.Errorf("IdentityFile: %q", got)
	}
	if got := m.securityIcons(noKey); got != "" {
		t.Errorf("no IdentityFile, no default key: %q", got)
	}
	m.defaultKey = true
	if got := m.securityIcons(noKey); got != iconDefaultKey {
		t.Errorf("no IdentityFile, default key: %q", got)
	}
}
