package config

import (
	"os"
	"testing"

	"github.com/chaad-grayson/sshin/internal/testhome"
)

func TestMain(m *testing.M) {
	os.Exit(testhome.Run(m))
}
