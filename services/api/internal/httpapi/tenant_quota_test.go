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

func TestTenantMonthlyQuotaBlocksSecondCheck(t *testing.T) {
	t.Parallel()

	const apiKey = "tenant-quota-key-with-at-least-thirty-two-characters"
	const expectedCPF = "12345678909"

	repository, err := identity.NewRepository(
		t.TempDir(),
		bytes.Repeat([]byte{0x61}, 32),
	)
	if err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(
		config.Config{
			IdentityVerifyURL: "https://verify.example.test/verify.html",
			IdentityLinkTTL:   time.Hour,
		},
		session.NewStore(),
		security.NewSigner(
			[]byte("session-secret-with-at-least-thirty-two-characters"),
		),
		nil,
		nil,
		repository,
		cnh.NewService(nil, nil),
		nil,
	)

	sum := sha256.Sum256([]byte(apiKey))
	registryPath := filepath.Join(t.TempDir(), "tenants.json")
	registryDocument := "{\n" +
		"  \"schemaVersion\": 1,\n" +
		"  \"tenants\": [\n" +
		"    {\n" +
		"      \"id\": \"tenant-quota\",\n" +
		"      \"name\": \"Tenant Quota\",\n" +
		"      \"apiKeySha256\": \"" + hex.EncodeToString(sum[:]) + "\",\n" +
		"      \"enabled\": true,\n" +
		"      \"monthlyCheckLimit\": 1\n" +
		"    }\n" +
		"  ]\n" +
		"}\n"
	if err := os.WriteFile(registryPath, []byte(registryDocument), 0o600); err != nil {
		t.Fatal(err)
	}

	registry, err := tenant.Load(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	handler.SetTenantRegistry(registry)

	create := func() *httptest.ResponseRecorder {
		body := "{\"cpf\":\"" + expectedCPF +
			"\",\"minimumDocumentDate\":\"2020-01-01\",\"expiresInMinutes\":60}"
		request := httptest.NewRequest(
			http.MethodPost,
			"/v1/identity/checks",
			strings.NewReader(body),
		)
		request.Header.Set("Authorization", "Bearer "+apiKey)
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder
	}

	first := create()
	if first.Code != http.StatusCreated {
		t.Fatalf("first create status = %d, body=%s", first.Code, first.Body.String())
	}

	second := create()
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf(
			"second create status = %d, want %d; body=%s",
			second.Code,
			http.StatusTooManyRequests,
			second.Body.String(),
		)
	}

	usageRequest := httptest.NewRequest(
		http.MethodGet,
		"/v1/identity/usage",
		nil,
	)
	usageRequest.Header.Set("Authorization", "Bearer "+apiKey)
	usageRecorder := httptest.NewRecorder()
	handler.ServeHTTP(usageRecorder, usageRequest)

	if usageRecorder.Code != http.StatusOK {
		t.Fatalf(
			"usage status = %d, body=%s",
			usageRecorder.Code,
			usageRecorder.Body.String(),
		)
	}

	var usage struct {
		TenantID           string `json:"tenantId"`
		Issued             int64  `json:"issued"`
		MonthlyCheckLimit  int    `json:"monthlyCheckLimit"`
		RemainingChecks    *int   `json:"remainingChecks"`
		AnalyticsAvailable bool   `json:"analyticsAvailable"`
	}
	if err := json.Unmarshal(usageRecorder.Body.Bytes(), &usage); err != nil {
		t.Fatal(err)
	}
	if usage.TenantID != "tenant-quota" {
		t.Fatalf("tenantId = %q", usage.TenantID)
	}
	if usage.Issued != 1 {
		t.Fatalf("issued = %d, want 1", usage.Issued)
	}
	if usage.MonthlyCheckLimit != 1 {
		t.Fatalf("monthlyCheckLimit = %d, want 1", usage.MonthlyCheckLimit)
	}
	if usage.RemainingChecks == nil || *usage.RemainingChecks != 0 {
		t.Fatalf("remainingChecks = %v, want 0", usage.RemainingChecks)
	}
	if usage.AnalyticsAvailable {
		t.Fatal("analyticsAvailable should be false in this test")
	}
}
