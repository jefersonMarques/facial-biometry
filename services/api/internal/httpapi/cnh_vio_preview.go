package httpapi

import (
	"encoding/base64"

	"faceproof/services/api/internal/document/cnh"
)

const maxDemoVIOPreviewBytes = 4 << 20

// demoVIOPhotoDataURL is a diagnostic convenience only: the original VIO
// portrait is never included in production check responses or analytics events.
func demoVIOPhotoDataURL(authorized bool, portrait cnh.DualPortrait) string {
	if !authorized || portrait.Status != cnh.DualPortraitReady ||
		portrait.Width <= 0 || portrait.Height <= 0 ||
		portrait.Width > 2048 || portrait.Height > 2048 ||
		len(portrait.Photo) == 0 || len(portrait.Photo) > maxDemoVIOPreviewBytes {
		return ""
	}
	if portrait.MIME != "image/png" && portrait.MIME != "image/jpeg" {
		return ""
	}
	return "data:" + portrait.MIME + ";base64," +
		base64.StdEncoding.EncodeToString(portrait.Photo)
}
