package httpapi

import (
	"encoding/base64"
	"net/http/httptest"
	"testing"

	"faceproof/services/api/internal/config"
	"faceproof/services/api/internal/identity"
)

func TestEffectiveFlowTypeKeepsLegacyChecksAsCNH(t *testing.T) {
	t.Parallel()

	if got := effectiveFlowType(identity.Check{}); got != identity.FlowCNH {
		t.Fatalf("flow = %q, want %q", got, identity.FlowCNH)
	}
}

func TestNormalizeDemoFlowType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  identity.FlowType
		ok    bool
	}{
		{"", identity.FlowCNH, true},
		{"cnh", identity.FlowCNH, true},
		{"face_enrollment", identity.FlowFaceEnrollment, true},
		{"face_verification", identity.FlowFaceVerification, true},
		{"photo_verification", identity.FlowPhotoVerification, true},
		{"other", "", false},
	}

	for _, test := range tests {
		got, ok := normalizeDemoFlowType(test.input)
		if ok != test.ok || got != test.want {
			t.Fatalf(
				"normalizeDemoFlowType(%q) = (%q, %v), want (%q, %v)",
				test.input,
				got,
				ok,
				test.want,
				test.ok,
			)
		}
	}
}

func TestDemoAdminBasicAuthorization(t *testing.T) {
	t.Parallel()

	handler := &Handler{
		config: config.Config{
			DemoMode:     true,
			DemoUser:     "demo",
			DemoPassword: []byte("safe-password"),
		},
	}
	request := httptest.NewRequest("GET", "/v1/admin/summary", nil)
	credentials := base64.StdEncoding.EncodeToString(
		[]byte("demo:safe-password"),
	)
	request.Header.Set("Authorization", "Basic "+credentials)

	if !handler.authorizeAdmin(request) {
		t.Fatal("valid demo basic credentials were rejected")
	}
}

func TestDemoAdminBasicAuthorizationRejectsWrongPassword(t *testing.T) {
	t.Parallel()

	handler := &Handler{
		config: config.Config{
			DemoMode:     true,
			DemoUser:     "demo",
			DemoPassword: []byte("safe-password"),
		},
	}
	request := httptest.NewRequest("GET", "/v1/admin/summary", nil)
	credentials := base64.StdEncoding.EncodeToString(
		[]byte("demo:wrong-password"),
	)
	request.Header.Set("Authorization", "Basic "+credentials)

	if handler.authorizeAdmin(request) {
		t.Fatal("wrong demo password was accepted")
	}
}

func TestAdminBearerStillWorksWithDemoMode(t *testing.T) {
	t.Parallel()

	handler := &Handler{
		config: config.Config{
			AdminKey:      []byte("01234567890123456789012345678901"),
			DemoMode:      true,
			DemoUser:      "demo",
			DemoPassword:  []byte("safe-password"),
		},
	}
	request := httptest.NewRequest("GET", "/v1/admin/summary", nil)
	request.Header.Set(
		"Authorization",
		"Bearer 01234567890123456789012345678901",
	)

	if !handler.authorizeAdmin(request) {
		t.Fatal("existing bearer admin authentication was broken")
	}
}
