package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"faceproof/services/api/internal/analytics"
	"faceproof/services/api/internal/document/cnh"
	"faceproof/services/api/internal/identity"
)

func TestDemoVIOPreviewOnlyForValidatedInternalShadow(t *testing.T) {
	const validPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScL/nwAAAABJRU5ErkJggg=="
	raw, err := base64.StdEncoding.DecodeString(validPNG)
	if err != nil {
		t.Fatal(err)
	}
	portrait := cnh.DualPortrait{
		Status: cnh.DualPortraitReady,
		MIME: "image/png",
		Photo: raw,
		Width: 45,
		Height: 61,
	}
	preview := demoVIOPhotoDataURL(true, portrait)
	if preview != "data:image/png;base64,"+validPNG {
		t.Fatalf("unexpected preview %q", preview)
	}
	check := identity.Check{
		TenantID: "internal",
		DemoArtifacts: &identity.DemoArtifacts{
			VIOPhotoDataURL: preview,
		},
	}
	if demoVIOPreviewForAdmin(check, true, true) != preview {
		t.Fatal("internal admin preview missing")
	}
	if demoVIOPreviewForAdmin(check, false, true) != "" ||
		demoVIOPreviewForAdmin(check, true, false) != "" {
		t.Fatal("VIO preview escaped demo and shadow gates")
	}
	check.TenantID = "external"
	if demoVIOPreviewForAdmin(check, true, true) != "" {
		t.Fatal("external tenant must not have VIO preview")
	}
	check.TenantID = "internal"
	check.DemoArtifacts.VIOPhotoDataURL = "javascript:alert(1)"
	if demoVIOPreviewForAdmin(check, true, true) != "" {
		t.Fatal("unsupported data URI accepted")
	}
}

func TestDemoVIOPreviewRejectsBadSources(t *testing.T) {
	portrait := cnh.DualPortrait{
		Status: cnh.DualPortraitReady,
		MIME: "image/png", Photo: []byte{0x89, 0x50, 0x4e, 0x47},
		Width: 45, Height: 61,
	}
	if demoVIOPhotoDataURL(false, portrait) != "" {
		t.Fatal("preview generated with shadow disabled")
	}
	portrait.Status = cnh.DualPortraitAnalysisFailed
	if demoVIOPhotoDataURL(true, portrait) != "" {
		t.Fatal("preview generated without VIO validation")
	}
	portrait.Status = cnh.DualPortraitReady
	portrait.MIME = "image/svg+xml"
	if demoVIOPhotoDataURL(true, portrait) != "" {
		t.Fatal("untrusted mime escaped")
	}
}

func TestAnalyticsDTOOmitsVIOPreviewWhenNotExplicitlyRequested(t *testing.T) {
	item := analytics.CheckListItem{CheckID: "chk_sample12345"}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "vioPhotoDataUrl") {
		t.Fatal("VIO photo should be omitted from list and report by default")
	}
}
