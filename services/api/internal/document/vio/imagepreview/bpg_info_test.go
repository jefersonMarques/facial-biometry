package imagepreview

import "testing"

func TestInspectBPG(t *testing.T) {
	input := []byte{'B', 'P', 'G', 0xFB, 0x20, 0x00, 0x2D, 0x3D, 0x00}

	info, err := InspectBPG(input)
	if err != nil {
		t.Fatalf("InspectBPG returned error: %v", err)
	}
	if info.Width != 45 || info.Height != 61 {
		t.Fatalf("dimensions = %dx%d, want 45x61", info.Width, info.Height)
	}
	if info.BitDepth != 8 {
		t.Fatalf("bit depth = %d, want 8", info.BitDepth)
	}
	if info.PixelFormat != "4:2:0 JPEG" {
		t.Fatalf("pixel format = %q", info.PixelFormat)
	}
	if info.ColorSpace != "YCbCr BT.601" {
		t.Fatalf("color space = %q", info.ColorSpace)
	}
}
