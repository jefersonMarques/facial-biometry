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
	SessionID string `json:"sid"`
	ExpiresAt int64  `json:"exp"`
}

type Signer struct {
	secret []byte
}

func NewSigner(secret []byte) *Signer {
	return &Signer{secret: secret}
}

func (signer *Signer) Sign(sessionID string, expiresAt time.Time) (string, error) {
	payloadBytes, err := json.Marshal(tokenPayload{SessionID: sessionID, ExpiresAt: expiresAt.Unix()})
	if err != nil {
		return "", err
	}

	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signature := signer.signature(payloadEncoded)
	return payloadEncoded + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (signer *Signer) Verify(token string, expectedSessionID string) error {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return errors.New("invalid session token")
	}

	expectedSignature := signer.signature(parts[0])
	providedSignature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(expectedSignature, providedSignature) {
		return errors.New("invalid session token signature")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return errors.New("invalid session token payload")
	}

	var payload tokenPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return errors.New("invalid session token payload")
	}
	if payload.SessionID != expectedSessionID {
		return errors.New("session token mismatch")
	}
	if time.Now().Unix() > payload.ExpiresAt {
		return errors.New("session token expired at " + strconv.FormatInt(payload.ExpiresAt, 10))
	}
	return nil
}

func (signer *Signer) signature(payload string) []byte {
	mac := hmac.New(sha256.New, signer.secret)
	mac.Write([]byte(payload))
	return mac.Sum(nil)
}
