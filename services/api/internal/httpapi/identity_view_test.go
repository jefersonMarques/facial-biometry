package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"faceproof/services/api/internal/config"
	"faceproof/services/api/internal/document/cnh"
	"faceproof/services/api/internal/identity"
	"faceproof/services/api/internal/security"
	"faceproof/services/api/internal/session"
)

func TestConfirmedIdentityViewRequiresMinimumVisibleTime(t *testing.T) {
	t.Parallel()

	repository, err := identity.NewRepository(
		t.TempDir(),
		bytes.Repeat([]byte{0x71}, 32),
	)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	const token = "identity-view-token"
	if err := repository.Create(token, identity.Check{
		ID:        "chk_view12345",
		TenantID:  "default",
		Status:    identity.StatusPendingDocument,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(
		config.Config{},
		session.NewStore(),
		security.NewSigner(
			[]byte("session-secret-with-at-least-thirty-two-characters"),
		),
		nil,
		nil,
		repository,
		cnh.NewService(nil, nil),
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/identity/view",
		strings.NewReader(`{"visibleMs":2999}`),
	)
	request.Header.Set("X-FaceProof-Identity-Token", token)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d, want %d; body=%s",
			recorder.Code,
			http.StatusBadRequest,
			recorder.Body.String(),
		)
	}
}

func TestConfirmedIdentityViewAcceptsThreeSeconds(t *testing.T) {
	t.Parallel()

	repository, err := identity.NewRepository(
		t.TempDir(),
		bytes.Repeat([]byte{0x72}, 32),
	)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	const token = "identity-view-token-ok"
	if err := repository.Create(token, identity.Check{
		ID:        "chk_view67890",
		TenantID:  "default",
		Status:    identity.StatusPendingDocument,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(
		config.Config{},
		session.NewStore(),
		security.NewSigner(
			[]byte("session-secret-with-at-least-thirty-two-characters"),
		),
		nil,
		nil,
		repository,
		cnh.NewService(nil, nil),
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/identity/view",
		strings.NewReader(`{"visibleMs":3000}`),
	)
	request.Header.Set("X-FaceProof-Identity-Token", token)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d; body=%s",
			recorder.Code,
			http.StatusOK,
			recorder.Body.String(),
		)
	}
}
