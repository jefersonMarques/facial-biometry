package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"faceproof/services/api/internal/config"
	"faceproof/services/api/internal/document/cnh"
	"faceproof/services/api/internal/identity"
	"faceproof/services/api/internal/security"
	"faceproof/services/api/internal/session"
	"faceproof/services/api/internal/tenant"
)

func TestIssuerCannotReadAnotherTenantCheck(t *testing.T) {
	t.Parallel()

	const tenantAKey = "tenant-a-key-with-at-least-thirty-two-characters"
	const tenantBKey = "tenant-b-key-with-at-least-thirty-two-characters"
	const expectedCPF = "12345678909"

	repository, err := identity.NewRepository(
		t.TempDir(),
		bytes.Repeat([]byte{0x43}, 32),
	)
	if err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(
		config.Config{
			IdentityVerifyURL:       "https://verify.example.test/verify.html",
			IdentityLinkTTL:         time.Hour,
			LivenessThreshold:       0.68,
			ReviewLivenessThreshold: 0.55,
			MatchThreshold:          0.363,
			RequirePassivePAD:       true,
		},
		session.NewStore(),
		security.NewSigner(
			[]byte("session-secret-with-at-least-thirty-two-characters"),
		),
		nil,
		nil,
		repository,
		cnh.NewService(nil, nil),
	)

	sumA := sha256.Sum256([]byte(tenantAKey))
	sumB := sha256.Sum256([]byte(tenantBKey))
	registryPath := filepath.Join(t.TempDir(), "tenants.json")
	registryDocument := "{\n" +
		"  \"schemaVersion\": 1,\n" +
		"  \"tenants\": [\n" +
		"    {\n" +
		"      \"id\": \"tenant-a\",\n" +
		"      \"name\": \"Tenant A\",\n" +
		"      \"apiKeySha256\": \"" + hex.EncodeToString(sumA[:]) + "\",\n" +
		"      \"enabled\": true\n" +
		"    },\n" +
		"    {\n" +
		"      \"id\": \"tenant-b\",\n" +
		"      \"name\": \"Tenant B\",\n" +
		"      \"apiKeySha256\": \"" + hex.EncodeToString(sumB[:]) + "\",\n" +
		"      \"enabled\": true\n" +
		"    }\n" +
		"  ]\n" +
		"}\n"
	if err := os.WriteFile(
		registryPath,
		[]byte(registryDocument),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	registry, err := tenant.Load(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	handler.SetTenantRegistry(registry)

	createBody := "{\"cpf\":\"" + expectedCPF + "\",\"minimumDocumentDate\":\"2020-01-01\",\"expiresInMinutes\":60}"
	createRequest := httptest.NewRequest(
		http.MethodPost,
		"/v1/identity/checks",
		strings.NewReader(createBody),
	)
	createRequest.Header.Set("Authorization", "Bearer "+tenantAKey)
	createRequest.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	handler.ServeHTTP(createRecorder, createRequest)

	if createRecorder.Code != http.StatusCreated {
		t.Fatalf(
			"create status = %d, body = %s",
			createRecorder.Code,
			createRecorder.Body.String(),
		)
	}

	var created createIdentityCheckResponse
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	foreignRequest := httptest.NewRequest(
		http.MethodGet,
		"/v1/identity/checks/"+created.ID,
		nil,
	)
	foreignRequest.Header.Set("Authorization", "Bearer "+tenantBKey)
	foreignRecorder := httptest.NewRecorder()
	handler.ServeHTTP(foreignRecorder, foreignRequest)

	if foreignRecorder.Code != http.StatusNotFound {
		t.Fatalf(
			"cross-tenant status = %d, want %d; body=%s",
			foreignRecorder.Code,
			http.StatusNotFound,
			foreignRecorder.Body.String(),
		)
	}
}
