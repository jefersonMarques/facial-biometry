package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

type tokenPayload struct {
	SessionID    string `json:"sid"`
	CaptureRunID string `json:"rid,omitempty"`
	ExpiresAt    int64  `json:"exp"`
}

type Signer struct {
	secret []byte
}

func NewSigner(secret []byte) *Signer {
	return &Signer{secret: secret}
}

func (signer *Signer) Sign(sessionID string, expiresAt time.Time) (string, error) {
	return signer.signPayload(tokenPayload{
		SessionID: sessionID,
		ExpiresAt: expiresAt.Unix(),
	})
}

func (signer *Signer) SignCapture(
	sessionID string,
	captureRunID string,
	expiresAt time.Time,
) (string, error) {
	return signer.signPayload(tokenPayload{
		SessionID:    sessionID,
		CaptureRunID: captureRunID,
		ExpiresAt:    expiresAt.Unix(),
	})
}

func (signer *Signer) signPayload(payload tokenPayload) (string, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signature := signer.signature(payloadEncoded)
	return payloadEncoded + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (signer *Signer) VerifyCapture(
	token string,
	expectedSessionID string,
	expectedCaptureRunID string,
) error {
	payload, err := signer.verifyPayload(token)
	if err != nil {
		return err
	}
	if payload.SessionID != expectedSessionID {
		return errors.New("session token mismatch")
	}
	if payload.CaptureRunID == "" || payload.CaptureRunID != expectedCaptureRunID {
		return errors.New("capture run token mismatch")
	}
	return nil
}

func (signer *Signer) Verify(token string, expectedSessionID string) error {
	payload, err := signer.verifyPayload(token)
	if err != nil {
		return err
	}
	if payload.SessionID != expectedSessionID {
		return errors.New("session token mismatch")
	}
	return nil
}

func (signer *Signer) verifyPayload(token string) (tokenPayload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return tokenPayload{}, errors.New("invalid session token")
	}

	expectedSignature := signer.signature(parts[0])
	providedSignature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(expectedSignature, providedSignature) {
		return tokenPayload{}, errors.New("invalid session token signature")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return tokenPayload{}, errors.New("invalid session token payload")
	}

	var payload tokenPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return tokenPayload{}, errors.New("invalid session token payload")
	}
	if time.Now().Unix() > payload.ExpiresAt {
		return tokenPayload{}, errors.New("session token expired at " + strconv.FormatInt(payload.ExpiresAt, 10))
	}
	return payload, nil
}

func (signer *Signer) signature(payload string) []byte {
	mac := hmac.New(sha256.New, signer.secret)
	mac.Write([]byte(payload))
	return mac.Sum(nil)
}
