package app

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/hotp"
	"github.com/pquerna/otp/totp"
)

const (
	totpPeriod = 30
	totpSkew   = 1
)

var (
	usernamePattern  = regexp.MustCompile(`^[a-z][a-z0-9_]{2,23}$`)
	recoveryAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	reservedNames    = map[string]bool{
		"admin": true, "administrator": true, "destek": true, "support": true,
		"mod": true, "moderator": true, "root": true, "system": true,
		"sistem": true, "icinidok": true, "official": true, "resmi": true,
	}
)

// NormalizeUsername trims and lowercases a username, then validates it.
func NormalizeUsername(raw string) (string, error) {
	username := strings.ToLower(strings.TrimSpace(raw))
	if !usernamePattern.MatchString(username) {
		return "", fmt.Errorf("kullanıcı adı 3-24 karakter olmalı; küçük harf, rakam ve alt çizgi kullan")
	}
	if reservedNames[username] {
		return "", fmt.Errorf("bu kullanıcı adı ayrılmış")
	}
	return username, nil
}

func GenerateTOTP(issuer, username string) (*otp.Key, error) {
	return totp.Generate(totp.GenerateOpts{Issuer: issuer, AccountName: username})
}

// VerifyTOTP checks a 6-digit code and returns the matching time step.
// The step lets callers reject replay of the same code.
func VerifyTOTP(secret, code string, at time.Time) (int64, bool) {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return 0, false
	}
	for _, char := range code {
		if char < '0' || char > '9' {
			return 0, false
		}
	}
	current := at.Unix() / totpPeriod
	for offset := int64(-totpSkew); offset <= totpSkew; offset++ {
		step := current + offset
		expected, err := hotp.GenerateCodeCustom(secret, uint64(step), hotp.ValidateOpts{
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		})
		if err != nil {
			return 0, false
		}
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return step, true
		}
	}
	return 0, false
}

func GenerateRecoveryCodes(count int) (codes []string, hashes []string, err error) {
	for i := 0; i < count; i++ {
		raw := make([]byte, 8)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, err
		}
		var builder strings.Builder
		for j, b := range raw {
			if j == 4 {
				builder.WriteByte('-')
			}
			builder.WriteByte(recoveryAlphabet[int(b)%len(recoveryAlphabet)])
		}
		code := builder.String()
		codes = append(codes, code)
		hashes = append(hashes, HashRecoveryCode(strings.ReplaceAll(code, "-", "")))
	}
	return codes, hashes, nil
}

func NormalizeRecoveryCode(raw string) (string, bool) {
	code := strings.ToUpper(strings.TrimSpace(raw))
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	if len(code) != 8 {
		return "", false
	}
	for _, char := range code {
		if !strings.ContainsRune(recoveryAlphabet, char) {
			return "", false
		}
	}
	return code, true
}

func HashRecoveryCode(normalized string) string {
	sum := sha256.Sum256([]byte("recovery:" + normalized))
	return hex.EncodeToString(sum[:])
}

func NewID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(raw)
}

func NewSessionToken() (token string, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashToken(token), nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
