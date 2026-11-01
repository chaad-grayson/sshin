package secret

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"golang.org/x/term"
)

const (
	envHost   = "SSHGO_ASKPASS_HOST"
	envTarget = "SSHGO_ASKPASS_TARGET"
)

// SSHEnv returns the environment to run ssh with for host: when the host has a
// saved password, ssh is told to ask sshgo for it (SSH_ASKPASS, OpenSSH >= 8.4).
// It returns nil (keep the current environment) when there is no password or
// the keyring cannot be read, so ssh prompts on the terminal as usual.
func SSHEnv(configFile, host string) []string {
	if pw, err := Get(host); err != nil || pw == "" {
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

// IsAskpass reports whether sshgo was started by ssh as its askpass program
func IsAskpass() bool {
	return os.Getenv(envHost) != ""
}

// Askpass answers one ssh prompt and returns the exit code: the saved password
// for the password prompt of the target host, the user's own answer (typed on
// the terminal) for anything else, such as a key passphrase or a host key check.
func Askpass(args []string) int {
	prompt := strings.Join(args, " ")
	answer, err := answer(prompt, os.Getenv(envHost), os.Getenv(envTarget))
	if err != nil {
		fmt.Fprintf(os.Stderr, "sshgo: %v\n", err)
		return 1
	}
	fmt.Println(answer)
	return 0
}

func answer(prompt, host, target string) (string, error) {
	if isPasswordPrompt(prompt, target) {
		// shortcut: a wrong saved password is sent again on each retry (3 by
		// default) until ssh gives up; fix it in the edit form
		if pw, err := Get(host); err == nil && pw != "" {
			return pw, nil
		}
	}
	return askTTY(prompt)
}

// isPasswordPrompt matches the password prompts of the target only
// ("user@host's password:" and keyboard-interactive "(user@host) Password:"),
// so a ProxyJump host asking for its own password never gets this one
func isPasswordPrompt(prompt, target string) bool {
	p := strings.ToLower(strings.TrimSpace(prompt))
	return target != "" && strings.Contains(p, strings.ToLower(target)) && strings.HasSuffix(p, "password:")
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
