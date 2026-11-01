package version

import (
	"os"
	"testing"

	"github.com/chandra-bachtiar/sshgo/internal/testhome"
)

func TestMain(m *testing.M) {
	os.Exit(testhome.Run(m))
}
