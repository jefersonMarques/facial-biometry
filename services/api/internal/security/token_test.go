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

func TestSignerBindsCaptureRunID(t *testing.T) {
	signer := NewSigner([]byte("01234567890123456789012345678901"))
	token, err := signer.SignCapture(
		"session-1",
		"123e4567-e89b-42d3-a456-426614174000",
		time.Now().Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := signer.VerifyCapture(
		token,
		"session-1",
		"123e4567-e89b-42d3-a456-426614174000",
	); err != nil {
		t.Fatal(err)
	}
	if err := signer.VerifyCapture(
		token,
		"session-1",
		"223e4567-e89b-42d3-a456-426614174000",
	); err == nil {
		t.Fatal("expected capture run mismatch")
	}
}

func TestLegacyVerifyStillAcceptsCaptureBoundTokenForSession(t *testing.T) {
	signer := NewSigner([]byte("01234567890123456789012345678901"))
	token, err := signer.SignCapture(
		"session-1",
		"123e4567-e89b-42d3-a456-426614174000",
		time.Now().Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := signer.Verify(token, "session-1"); err != nil {
		t.Fatal(err)
	}
}
