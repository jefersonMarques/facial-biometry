package security

import (
	"testing"
	"time"
)

func TestSignerRoundTrip(t *testing.T) {
	signer := NewSigner([]byte("01234567890123456789012345678901"))
	token, err := signer.Sign("session-1", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := signer.Verify(token, "session-1"); err != nil {
		t.Fatal(err)
	}
}

func TestSignerRejectsDifferentSession(t *testing.T) {
	signer := NewSigner([]byte("01234567890123456789012345678901"))
	token, err := signer.Sign("session-1", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := signer.Verify(token, "session-2"); err == nil {
		t.Fatal("expected token mismatch")
	}
}
