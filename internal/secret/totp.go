package secret

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// TOTPKey is the keyring key of the TOTP secret of host. Host names never
// contain spaces, so it cannot clash with the password of another host.
func TOTPKey(host string) string {
	return host + " totp"
}

// ParseTOTPSecret checks a base32 TOTP secret as authenticator apps show it
// (spaces and lower case allowed) and returns it normalized
func ParseTOTPSecret(s string) (string, error) {
	s = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	s = strings.TrimRight(s, "=")
	if _, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s); err != nil || s == "" {
		return "", fmt.Errorf("the TOTP secret must be base32 (A-Z, 2-7), as shown when 2FA was set up")
	}
	return s, nil
}

// TOTP returns the 6-digit code of secret at t (RFC 6238: SHA-1, 30 s steps),
// the default of Google Authenticator and of the google-authenticator PAM module
// shortcut: SHA-1/6 digits/30 s only; read an otpauth:// URI if a server needs other settings
func TOTP(secret string, t time.Time) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return "", err
	}
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(t.Unix()/30))
	mac := hmac.New(sha1.New, key)
	mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	code := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", code%1000000), nil
}
