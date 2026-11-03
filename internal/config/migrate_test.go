package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMigrateOldConfig(t *testing.T) {
	useTempConfigDir(t)
	oldDir, _ := appConfigDir("sshm")
	newDir, _ := GetSSHINConfigDir()

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

	from, err := MigrateOldConfig()
	if err != nil || from != oldDir {
		t.Fatalf("MigrateOldConfig() = %q, %v; want %q, nil", from, err, oldDir)
	}

	for rel, want := range map[string]string{
		"config.json":           `{"check_for_updates":false}`,
		"sshin_history.json":    `{"connections":{}}`,
		"backups/config.backup": "Host a\n",
	} {
		got, err := os.ReadFile(filepath.Join(newDir, rel))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q", rel, got, err, want)
		}
	}
	if _, err := os.Stat(filepath.Join(newDir, "sshm_history.json")); !os.IsNotExist(err) {
		t.Error("sshm_history.json should have been renamed to sshin_history.json")
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

	// Second run: sshin dir exists, nothing is overwritten
	write("config.json", `{"changed":true}`, 0644)
	if from, err := MigrateOldConfig(); err != nil || from != "" {
		t.Fatalf("second MigrateOldConfig() = %q, %v; want \"\", nil", from, err)
	}
	got, _ := os.ReadFile(filepath.Join(newDir, "config.json"))
	if string(got) != `{"check_for_updates":false}` {
		t.Errorf("config.json overwritten on second run: %s", got)
	}
}

func TestMigrateOldConfig_NoSSHM(t *testing.T) {
	useTempConfigDir(t)
	if from, err := MigrateOldConfig(); err != nil || from != "" {
		t.Fatalf("MigrateOldConfig() = %q, %v; want \"\", nil", from, err)
	}
	newDir, _ := GetSSHINConfigDir()
	if _, err := os.Stat(newDir); !os.IsNotExist(err) {
		t.Error("no sshin dir should be created when there is no sshm dir")
	}
}

func TestMigrateOldConfig_PrefersSSHGO(t *testing.T) {
	useTempConfigDir(t)
	newDir, _ := GetSSHINConfigDir()
	for name, content := range map[string]string{"sshgo": "from sshgo", "sshm": "from sshm"} {
		dir, _ := appConfigDir(name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+"_history.json"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	sshgoDir, _ := appConfigDir("sshgo")
	if from, err := MigrateOldConfig(); err != nil || from != sshgoDir {
		t.Fatalf("MigrateOldConfig() = %q, %v; want %q, nil", from, err, sshgoDir)
	}
	got, err := os.ReadFile(filepath.Join(newDir, "sshin_history.json"))
	if err != nil || string(got) != "from sshgo" {
		t.Errorf("sshin_history.json = %q, %v; want %q", got, err, "from sshgo")
	}
}
