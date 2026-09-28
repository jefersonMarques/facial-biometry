package httpapi

import (
	"bytes"
	"encoding/json"
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

func TestIssuerCreatesAndReadsIdentityCheckWithoutExposingSecrets(t *testing.T) {
	const issuerKey = "issuer-key-with-at-least-thirty-two-characters"
	const expectedCPF = "12345678909"

	repository, err := identity.NewRepository(t.TempDir(), bytes.Repeat([]byte{0x42}, 32))
	if err != nil {
		t.Fatal(err)
	}

	configuration := config.Config{
		IdentityIssuerKey: []byte(issuerKey),
		IdentityVerifyURL: "https://verify.example.test/verify.html",
		IdentityLinkTTL:   time.Hour,
		LivenessThreshold: 0.68,
		ReviewLivenessThreshold: 0.55,
		MatchThreshold: 0.363,
		RequirePassivePAD: true,
	}

	handler := NewHandler(
		configuration,
		session.NewStore(),
		security.NewSigner([]byte("session-secret-with-at-least-thirty-two-characters")),
		nil,
		nil,
		repository,
		cnh.NewService(nil, nil, nil),
	)

	createBody := `{"cpf":"` + expectedCPF + `","minimumDocumentDate":"2020-01-01","expiresInMinutes":60}`
	createRequest := httptest.NewRequest(http.MethodPost, "/v1/identity/checks", strings.NewReader(createBody))
	createRequest.Header.Set("Authorization", "Bearer "+issuerKey)
	createRequest.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()

	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created createIdentityCheckResponse
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || !strings.HasPrefix(created.ID, "chk_") {
		t.Fatalf("unexpected check id: %q", created.ID)
	}
	if !strings.Contains(created.VerificationURL, "#identity=") {
		t.Fatalf("verification URL does not keep token in fragment: %q", created.VerificationURL)
	}
	if strings.Contains(createRecorder.Body.String(), expectedCPF) {
		t.Fatal("create response leaked expected CPF")
	}

	statusRequest := httptest.NewRequest(http.MethodGet, "/v1/identity/checks/"+created.ID, nil)
	statusRequest.Header.Set("Authorization", "Bearer "+issuerKey)
	statusRecorder := httptest.NewRecorder()

	handler.ServeHTTP(statusRecorder, statusRequest)
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, body = %s", statusRecorder.Code, statusRecorder.Body.String())
	}
	if strings.Contains(statusRecorder.Body.String(), expectedCPF) {
		t.Fatal("issuer result leaked expected CPF")
	}
	if strings.Contains(statusRecorder.Body.String(), "#identity=") {
		t.Fatal("issuer result leaked public verification token")
	}
}
