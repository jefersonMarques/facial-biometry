package modelpack

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateVerifyAndExtract(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	privateKey := filepath.Join(root, "private.key")
	publicKey := filepath.Join(root, "public.key")
	if err := GenerateKeyPair(privateKey, publicKey); err != nil {
		t.Fatal(err)
	}

	sources := testSources(t, root)
	packPath := filepath.Join(root, "faceproof.fpmp")
	manifest, err := Create(CreateOptions{
		OutputPath:           packPath,
		PrivateKeyPath:       privateKey,
		PackID:               "faceproof-test",
		PackVersion:          "0.1.0",
		SecureCoreMinVersion: "0.2.0",
		Models:               sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %d", manifest.SchemaVersion)
	}

	publicValue, err := os.ReadFile(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyAndExtract(
		packPath,
		strings.TrimSpace(string(publicValue)),
		filepath.Join(root, "cache"),
	)
	if err != nil {
		t.Fatal(err)
	}

	assertFileEquals(t, sources[0].Path, verified.YUNetPath)
	assertFileEquals(t, sources[1].Path, verified.SFacePath)
	assertFileEquals(t, sources[2].Path, verified.MiniFASNetPath)
}

func TestRejectsTamperedManifest(t *testing.T) {
	t.Parallel()

	root, packPath, publicKey := createFixturePack(t)
	tampered := filepath.Join(root, "tampered-manifest.fpmp")
	rewriteZipEntry(t, packPath, tampered, "manifest.json", func(data []byte) []byte {
		return bytes.Replace(data, []byte("0.1.0"), []byte("9.9.9"), 1)
	})

	_, err := VerifyAndExtract(
		tampered,
		publicKey,
		filepath.Join(root, "cache-manifest"),
	)
	if err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("expected signature error, got %v", err)
	}
}

func TestRejectsTamperedModel(t *testing.T) {
	t.Parallel()

	root, packPath, publicKey := createFixturePack(t)
	tampered := filepath.Join(root, "tampered-model.fpmp")
	rewriteZipEntry(t, packPath, tampered, "models/yunet.onnx", func(data []byte) []byte {
		result := append([]byte(nil), data...)
		result[len(result)/2] ^= 0xff
		return result
	})

	_, err := VerifyAndExtract(
		tampered,
		publicKey,
		filepath.Join(root, "cache-model"),
	)
	if err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("expected SHA-256 error, got %v", err)
	}
}

func createFixturePack(t *testing.T) (string, string, string) {
	t.Helper()

	root := t.TempDir()
	privateKey := filepath.Join(root, "private.key")
	publicKeyPath := filepath.Join(root, "public.key")
	if err := GenerateKeyPair(privateKey, publicKeyPath); err != nil {
		t.Fatal(err)
	}

	packPath := filepath.Join(root, "faceproof.fpmp")
	if _, err := Create(CreateOptions{
		OutputPath:           packPath,
		PrivateKeyPath:       privateKey,
		PackID:               "faceproof-test",
		PackVersion:          "0.1.0",
		SecureCoreMinVersion: "0.2.0",
		Models:               testSources(t, root),
	}); err != nil {
		t.Fatal(err)
	}

	publicKey, err := os.ReadFile(publicKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	return root, packPath, strings.TrimSpace(string(publicKey))
}

func testSources(t *testing.T, root string) []SourceModel {
	t.Helper()

	definitions := []SourceModel{
		{Role: RoleYUNet, Name: "YuNet", Version: "2023mar"},
		{Role: RoleSFace, Name: "SFace", Version: "2021dec"},
		{Role: RoleMiniFASNet, Name: "MiniFASNetV2", Version: "V2"},
	}
	for index := range definitions {
		path := filepath.Join(root, definitions[index].Role+".onnx")
		data := bytes.Repeat([]byte{byte(index + 1), 0x00, 0xff, 0x7f}, 128)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		definitions[index].Path = path
	}
	return definitions
}

func assertFileEquals(t *testing.T, expectedPath string, actualPath string) {
	t.Helper()
	expected, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(actualPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(expected, actual) {
		t.Fatalf("file mismatch: %s != %s", expectedPath, actualPath)
	}
}

func rewriteZipEntry(
	t *testing.T,
	sourcePath string,
	targetPath string,
	targetName string,
	transform func([]byte) []byte,
) {
	t.Helper()

	reader, err := zip.OpenReader(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	output, err := os.Create(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(output)

	found := false
	for _, entry := range reader.File {
		source, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(source)
		_ = source.Close()
		if err != nil {
			t.Fatal(err)
		}
		if entry.Name == targetName {
			data = transform(data)
			found = true
		}

		header := &zip.FileHeader{Name: entry.Name, Method: zip.Deflate}
		header.SetMode(entry.Mode())
		destination, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := destination.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if !found {
		t.Fatalf("entry %s not found", targetName)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
}
