package secret

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/term"
)

const (
	envHost   = "SSHIN_ASKPASS_HOST"
	envTarget = "SSHIN_ASKPASS_TARGET"
)

// SSHEnv returns the environment to run ssh with for host: when the host has a
// saved password or TOTP secret, ssh is told to ask sshin for them
// (SSH_ASKPASS, OpenSSH >= 8.4). It returns nil (keep the current environment)
// when there is none or the keyring cannot be read, so ssh prompts on the
// terminal as usual.
func SSHEnv(configFile, host string) []string {
	pw, _ := Get(host)
	totp, _ := Get(TOTPKey(host))
	if pw == "" && totp == "" {
		return nil
	}
	target, err := sshTarget(configFile, host)
	if err != nil {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	return append(os.Environ(),
		"SSH_ASKPASS="+exe,
		"SSH_ASKPASS_REQUIRE=force",
		envHost+"="+host,
		envTarget+"="+target,
	)
}

// sshTarget returns "user@hostname" as ssh resolves them for host
func sshTarget(configFile, host string) (string, error) {
	args := []string{"-G"}
	if configFile != "" {
		args = append(args, "-F", configFile)
	}
	out, err := exec.Command("ssh", append(args, host)...).Output()
	if err != nil {
		return "", err
	}
	return parseTarget(string(out))
}

func parseTarget(sshG string) (string, error) {
	var user, hostname string
	for _, line := range strings.Split(sshG, "\n") {
		key, value, _ := strings.Cut(strings.TrimSpace(line), " ")
		switch key {
		case "user":
			user = value
		case "hostname":
			hostname = value
		}
	}
	if user == "" || hostname == "" {
		return "", fmt.Errorf("ssh -G gave no user or hostname")
	}
	return user + "@" + hostname, nil
}

// IsAskpass reports whether sshin was started by ssh as its askpass program
func IsAskpass() bool {
	return os.Getenv(envHost) != ""
}

// Askpass answers one ssh prompt and returns the exit code: the saved password
// for the password prompt of the target host, a code from the saved TOTP secret
// for its verification code prompt, the user's own answer (typed on
// the terminal) for anything else, such as a key passphrase or a host key check.
func Askpass(args []string) int {
	prompt := strings.Join(args, " ")
	answer, err := answer(prompt, os.Getenv(envHost), os.Getenv(envTarget))
	if err != nil {
		fmt.Fprintf(os.Stderr, "sshin: %v\n", err)
		return 1
	}
	fmt.Println(answer)
	return 0
}

func answer(prompt, host, target string) (string, error) {
	switch {
	// Checked first: "One-time password:" also ends with "password:"
	case isCodePrompt(prompt, target):
		if s, err := Get(TOTPKey(host)); err == nil && s != "" {
			if firstTry(markerPath("totp"), time.Now()) {
				return TOTP(s, time.Now())
			}
			prompt = "sshin: the code from the saved TOTP secret was rejected, type one (check the secret with e in sshin, and the clock)\n" + prompt
		}
	case isPasswordPrompt(prompt, target):
		if pw, err := Get(host); err == nil && pw != "" {
			if firstTry(markerPath("password"), time.Now()) {
				return pw, nil
			}
			// ssh asks again: the server rejected the saved password
			prompt = "sshin: the saved password was rejected, type it (and fix it with e in sshin)\n" + prompt
		}
	}
	return askTTY(prompt)
}

// markerPath is the file that records that the saved password was sent to
// the ssh process that runs this askpass (each prompt is a new askpass process)
// shortcut: one tiny file per connection is left in the temp dir, cleaned
// with it; remove it after the login if it ever matters
func markerPath(kind string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("sshin-askpass-%d-%s", os.Getppid(), kind))
}

// firstTry reports whether the saved password has not been sent yet to this
// ssh process, and records that it is sent now. A marker older than a minute
// belongs to an earlier ssh process that had the same pid.
func firstTry(marker string, now time.Time) bool {
	if info, err := os.Stat(marker); err == nil && now.Sub(info.ModTime()) < time.Minute {
		return false
	}
	_ = os.WriteFile(marker, nil, 0o600)
	_ = os.Chtimes(marker, now, now)
	return true
}

// isPasswordPrompt matches the password prompts of the target only
// ("user@host's password:" and keyboard-interactive "(user@host) Password:"),
// so a ProxyJump host asking for its own password never gets this one
func isPasswordPrompt(prompt, target string) bool {
	p := strings.ToLower(strings.TrimSpace(prompt))
	return target != "" && strings.Contains(p, strings.ToLower(target)) && strings.HasSuffix(p, "password:")
}

// isCodePrompt matches the one-time code prompts of the target, such as
// "(user@host) Verification code:" of the google-authenticator PAM module
func isCodePrompt(prompt, target string) bool {
	p := strings.ToLower(strings.TrimSpace(prompt))
	if target == "" || !strings.Contains(p, strings.ToLower(target)) {
		return false
	}
	for _, s := range []string{"verification code:", "one-time password:", "otp:", "authenticator code:"} {
		if strings.HasSuffix(p, s) {
			return true
		}
	}
	return false
}

// askTTY asks the prompt on the terminal, as ssh would have done itself
func askTTY(prompt string) (string, error) {
	inName, outName := "/dev/tty", "/dev/tty"
	if runtime.GOOS == "windows" {
		inName, outName = "CONIN$", "CONOUT$"
	}
	in, err := os.OpenFile(inName, os.O_RDWR, 0)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.OpenFile(outName, os.O_RDWR, 0)
	if err != nil {
		return "", err
	}
	defer out.Close()

	fmt.Fprint(out, prompt)
	if strings.Contains(prompt, "(yes/no") {
		line, err := bufio.NewReader(in).ReadString('\n')
		return strings.TrimRight(line, "\r\n"), err
	}
	b, err := term.ReadPassword(int(in.Fd()))
	fmt.Fprintln(out)
	return string(b), err
}
