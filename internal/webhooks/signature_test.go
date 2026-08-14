package webhooks

import "testing"

func TestSignVerify(t *testing.T) {
	body := []byte(`{"event":"instance.created"}`)
	sig := Sign("super-secret", body)
	if sig == "" {
		t.Fatal("expected a signature")
	}
	if !Verify("super-secret", body, sig) {
		t.Error("expected signature to verify")
	}
	if Verify("wrong-secret", body, sig) {
		t.Error("signature must not verify with a different secret")
	}
	if Verify("super-secret", []byte(`{"event":"tampered"}`), sig) {
		t.Error("signature must not verify with tampered body")
	}
}
