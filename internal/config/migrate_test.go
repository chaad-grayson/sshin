package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMigrateFromSSHM(t *testing.T) {
	useTempConfigDir(t)
	oldDir, _ := appConfigDir("sshm")
	newDir, _ := GetSSHGOConfigDir()

	write := func(rel, content string, perm os.FileMode) {
		p := filepath.Join(oldDir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), perm); err != nil {
			t.Fatal(err)
		}
	}
	write("config.json", `{"check_for_updates":false}`, 0644)
	write("sshm_history.json", `{"connections":{}}`, 0644)
	write("backups/config.backup", "Host a\n", 0600)

	from, err := MigrateFromSSHM()
	if err != nil || from != oldDir {
		t.Fatalf("MigrateFromSSHM() = %q, %v; want %q, nil", from, err, oldDir)
	}

	for rel, want := range map[string]string{
		"config.json":           `{"check_for_updates":false}`,
		"sshgo_history.json":    `{"connections":{}}`,
		"backups/config.backup": "Host a\n",
	} {
		got, err := os.ReadFile(filepath.Join(newDir, rel))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q", rel, got, err, want)
		}
	}
	if _, err := os.Stat(filepath.Join(newDir, "sshm_history.json")); !os.IsNotExist(err) {
		t.Error("sshm_history.json should have been renamed to sshgo_history.json")
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(filepath.Join(newDir, "backups/config.backup"))
		if info.Mode().Perm() != 0600 {
			t.Errorf("backup perm = %v, want 0600", info.Mode().Perm())
		}
	}
	if _, err := os.Stat(filepath.Join(oldDir, "sshm_history.json")); err != nil {
		t.Error("the sshm directory must be left untouched")
	}

	// Second run: sshgo dir exists, nothing is overwritten
	write("config.json", `{"changed":true}`, 0644)
	if from, err := MigrateFromSSHM(); err != nil || from != "" {
		t.Fatalf("second MigrateFromSSHM() = %q, %v; want \"\", nil", from, err)
	}
	got, _ := os.ReadFile(filepath.Join(newDir, "config.json"))
	if string(got) != `{"check_for_updates":false}` {
		t.Errorf("config.json overwritten on second run: %s", got)
	}
}

func TestMigrateFromSSHM_NoSSHM(t *testing.T) {
	useTempConfigDir(t)
	if from, err := MigrateFromSSHM(); err != nil || from != "" {
		t.Fatalf("MigrateFromSSHM() = %q, %v; want \"\", nil", from, err)
	}
	newDir, _ := GetSSHGOConfigDir()
	if _, err := os.Stat(newDir); !os.IsNotExist(err) {
		t.Error("no sshgo dir should be created when there is no sshm dir")
	}
}
