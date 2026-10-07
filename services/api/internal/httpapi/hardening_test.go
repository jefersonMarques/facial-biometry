package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"faceproof/services/api/internal/config"
	"faceproof/services/api/internal/security"
	"faceproof/services/api/internal/session"
)

func TestSecurityHeadersAreApplied(t *testing.T) {
	t.Parallel()

	handler := NewHandler(
		config.Config{},
		session.NewStore(),
		security.NewSigner([]byte("session-secret-with-at-least-thirty-two-characters")),
		nil,
		nil,
		nil,
		nil,
	)

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	assertHeader(t, recorder, "Cache-Control", "no-store")
	assertHeader(t, recorder, "Pragma", "no-cache")
	assertHeader(t, recorder, "X-Content-Type-Options", "nosniff")
	assertHeader(t, recorder, "X-Frame-Options", "DENY")
	assertHeader(t, recorder, "Referrer-Policy", "no-referrer")
}

func TestPublicIdentityRateLimiterReturns429(t *testing.T) {
	t.Parallel()

	handler := NewHandler(
		config.Config{},
		session.NewStore(),
		security.NewSigner([]byte("session-secret-with-at-least-thirty-two-characters")),
		nil,
		nil,
		nil,
		nil,
	)

	const token = "public-test-token"
	for attempt := 0; attempt < 120; attempt++ {
		request := httptest.NewRequest(http.MethodGet, "/v1/identity/check", nil)
		request.Header.Set("X-FaceProof-Identity-Token", token)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)
		if recorder.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d was rate limited too early", attempt+1)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/identity/check", nil)
	request.Header.Set("X-FaceProof-Identity-Token", token)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
	if recorder.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header is missing")
	}
}

func assertHeader(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	name string,
	expected string,
) {
	t.Helper()
	if actual := recorder.Header().Get(name); actual != expected {
		t.Fatalf("%s = %q, want %q", name, actual, expected)
	}
}
