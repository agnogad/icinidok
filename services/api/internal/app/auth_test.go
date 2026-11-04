package app

import (
	"testing"
	"time"

	"github.com/pquerna/otp/hotp"
)

func TestNormalizeUsername(t *testing.T) {
	valid := []string{"ayse", "gece_kirpi", "user123", "a_b"}
	for _, raw := range valid {
		if _, err := NormalizeUsername(raw); err != nil {
			t.Errorf("%q should be valid: %v", raw, err)
		}
	}
	invalid := []string{"", "ab", "Ayşe", "1ayse", "ayse-2", "admin", "bu kullanıcı adı çok uzun bir şey"}
	for _, raw := range invalid {
		if _, err := NormalizeUsername(raw); err == nil {
			t.Errorf("%q should be invalid", raw)
		}
	}
	if got, _ := NormalizeUsername("  Gece_Kirpi "); got != "gece_kirpi" {
		t.Errorf("normalize: got %q", got)
	}
}

func TestTOTPVerifyAndStep(t *testing.T) {
	key, err := GenerateTOTP("içini dök.", "test_user")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Unix(1_800_000_000, 0)
	step := at.Unix() / totpPeriod
	code, err := hotp.GenerateCodeCustom(key.Secret(), uint64(step), hotp.ValidateOpts{})
	if err != nil {
		t.Fatal(err)
	}
	gotStep, ok := VerifyTOTP(key.Secret(), code, at)
	if !ok || gotStep != step {
		t.Fatalf("valid code rejected: ok=%v step=%d want=%d", ok, gotStep, step)
	}
	if _, ok := VerifyTOTP(key.Secret(), "000000", at); ok {
		t.Fatal("wrong code accepted")
	}
	if _, ok := VerifyTOTP(key.Secret(), "12345", at); ok {
		t.Fatal("short code accepted")
	}
	// Previous and next step are accepted inside the skew window.
	previous, _ := hotp.GenerateCodeCustom(key.Secret(), uint64(step-1), hotp.ValidateOpts{})
	if _, ok := VerifyTOTP(key.Secret(), previous, at); !ok {
		t.Fatal("previous step should be accepted within skew")
	}
}

func TestRecoveryCodes(t *testing.T) {
	codes, hashes, err := GenerateRecoveryCodes(8)
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 8 || len(hashes) != 8 {
		t.Fatalf("got %d codes and %d hashes", len(codes), len(hashes))
	}
	seen := map[string]bool{}
	for _, code := range codes {
		normalized, ok := NormalizeRecoveryCode(code)
		if !ok {
			t.Fatalf("generated code invalid: %q", code)
		}
		if HashRecoveryCode(normalized) == "" || seen[normalized] {
			t.Fatal("bad recovery code")
		}
		seen[normalized] = true
	}
	if _, ok := NormalizeRecoveryCode("0000-0000"); ok {
		t.Fatal("ambiguous characters should be rejected")
	}
}
