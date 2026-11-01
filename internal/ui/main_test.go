package ui

import (
	"os"
	"testing"

	"github.com/chandra-bachtiar/sshgo/internal/testhome"
	"github.com/zalando/go-keyring"
)

// The forms read saved passwords and the config dir: keep the tests off the
// real OS keyring and the real home directory
func TestMain(m *testing.M) {
	keyring.MockInit()
	os.Exit(testhome.Run(m))
}
