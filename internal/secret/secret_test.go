package secret

import (
	"testing"

	"github.com/zalando/go-keyring"
)

func TestIsPasswordPrompt(t *testing.T) {
	target := "deploy@10.0.0.5"
	tests := []struct {
		prompt string
		want   bool
	}{
		{"deploy@10.0.0.5's password: ", true},
		{"(deploy@10.0.0.5) Password: ", true},
		{"jump@bastion's password: ", false}, // ProxyJump host must not get it
		{"Enter passphrase for key '/home/u/.ssh/id_ed25519': ", false},
		{"Are you sure you want to continue connecting (yes/no/[fingerprint])? ", false},
		{"(deploy@10.0.0.5) Verification code: ", false},
	}
	for _, tt := range tests {
		if got := isPasswordPrompt(tt.prompt, target); got != tt.want {
			t.Errorf("isPasswordPrompt(%q) = %v, want %v", tt.prompt, got, tt.want)
		}
	}
	if isPasswordPrompt("deploy@10.0.0.5's password: ", "") {
		t.Error("an empty target must never match")
	}
}

func TestParseTarget(t *testing.T) {
	got, err := parseTarget("host myserver\nuser deploy\nhostname 10.0.0.5\nport 22\n")
	if err != nil || got != "deploy@10.0.0.5" {
		t.Errorf("parseTarget() = %q, %v", got, err)
	}
	if _, err := parseTarget("port 22\n"); err == nil {
		t.Error("parseTarget() without user/hostname should fail")
	}
}

func TestUpdateBlock(t *testing.T) {
	keyring.MockInit()
	get := func(n string) string { pw, _ := Get(n); return pw }

	// New password for a renamed host: saved under the new name only
	_ = Set("old", "pw1")
	if err := UpdateBlock([]string{"old"}, []string{"new"}, "old", "pw2", false); err != nil {
		t.Fatal(err)
	}
	if get("old") != "" || get("new") != "pw2" {
		t.Errorf("set+rename: old=%q new=%q", get("old"), get("new"))
	}

	// Password left empty on rename: the saved one moves with the host
	if err := UpdateBlock([]string{"new"}, []string{"a", "b"}, "new", "", false); err != nil {
		t.Fatal(err)
	}
	if get("new") != "" || get("a") != "pw2" || get("b") != "pw2" {
		t.Errorf("keep+rename: new=%q a=%q b=%q", get("new"), get("a"), get("b"))
	}

	// Remove
	if err := UpdateBlock([]string{"a", "b"}, []string{"a", "b"}, "a", "", true); err != nil {
		t.Fatal(err)
	}
	if get("a") != "" || get("b") != "" {
		t.Errorf("remove: a=%q b=%q", get("a"), get("b"))
	}

	// No password anywhere: nothing is created
	if err := UpdateBlock([]string{"x"}, []string{"y"}, "x", "", false); err != nil {
		t.Fatal(err)
	}
	if get("y") != "" {
		t.Errorf("no password: y=%q", get("y"))
	}
}
