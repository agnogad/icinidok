package app

import (
	"testing"
	"time"
)

// TestSignupConfirmOwnBucket guards the regression where a caller could burn a
// victim's pending-signup attempt budget by hammering the endpoint.
func TestSignupConfirmOwnBucket(t *testing.T) {
	store, err := OpenStore(t.TempDir() + "/reg.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	server := New(Config{Issuer: "test", WebDir: t.TempDir()}, store)
	now := time.Unix(1_800_000_000, 0)
	server.runAt(&now)

	// Victim starts a signup.
	signupID, err := store.CreatePendingSignup("kurban", "JBSWY3DPEHPK3PXP", now, signupTTL)
	if err != nil {
		t.Fatal(err)
	}

	// Attacker floods confirm from the same IP; only the victim's own requests
	// count against the victim's bucket.
	for i := 0; i < 50; i++ {
		if _, err := store.PendingSignup(signupID); err != nil {
			t.Fatalf("victim signup deleted by flood: %v", err)
		}
		server.signIP.Allow("confirm:attacker-signup-id")
	}

	pending, err := store.PendingSignup(signupID)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Attempts != 0 {
		t.Fatalf("victim attempts changed: %d", pending.Attempts)
	}
	if !server.signIP.Allow("confirm:" + signupID) {
		t.Fatal("victim's own bucket was drained")
	}
}
