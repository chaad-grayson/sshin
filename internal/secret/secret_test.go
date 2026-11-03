package secret

import (
	"path/filepath"
	"testing"
	"time"

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
	// Windows ssh ends its lines with \r\n
	got, err = parseTarget("user deploy\r\nhostname 10.0.0.5\r\n")
	if err != nil || got != "deploy@10.0.0.5" {
		t.Errorf("parseTarget() with CRLF = %q, %v", got, err)
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

func TestFirstTry(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker")
	now := time.Now()
	if !firstTry(marker, now) {
		t.Error("first prompt: the saved password must be sent")
	}
	if firstTry(marker, now.Add(2*time.Second)) {
		t.Error("second prompt of the same ssh: the password was rejected, ask the user")
	}
	if !firstTry(marker, now.Add(2*time.Minute)) {
		t.Error("an old marker from a reused pid must not block the password")
	}
}

func TestTOTP(t *testing.T) {
	// RFC 6238 test vectors (SHA-1), last 6 digits
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" // "12345678901234567890"
	for unix, want := range map[int64]string{59: "287082", 1111111109: "081804", 2000000000: "279037"} {
		if got, err := TOTP(secret, time.Unix(unix, 0)); err != nil || got != want {
			t.Errorf("TOTP(%d) = %q, %v, want %q", unix, got, err, want)
		}
	}
}

func TestParseTOTPSecret(t *testing.T) {
	if got, err := ParseTOTPSecret(" gezd gnbv gy3t qojq== "); err != nil || got != "GEZDGNBVGY3TQOJQ" {
		t.Errorf("ParseTOTPSecret() = %q, %v", got, err)
	}
	for _, bad := range []string{"", "not base32!", "12345678"} {
		if _, err := ParseTOTPSecret(bad); err == nil {
			t.Errorf("ParseTOTPSecret(%q) should fail", bad)
		}
	}
}

func TestIsCodePrompt(t *testing.T) {
	target := "chad@18.143.119.254"
	tests := map[string]bool{
		"(chad@18.143.119.254) Verification code: ": true,
		"(chad@18.143.119.254) One-time password: ": true,
		"(jump@bastion) Verification code: ":        false,
		"chad@18.143.119.254's password: ":          false,
		"(chad@18.143.119.254) Password: ":          false,
	}
	for prompt, want := range tests {
		if got := isCodePrompt(prompt, target); got != want {
			t.Errorf("isCodePrompt(%q) = %v, want %v", prompt, got, want)
		}
	}
}
