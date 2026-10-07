package tenant

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryAuthenticatesEnabledTenant(t *testing.T) {
	t.Parallel()

	const apiKey = "tenant-secret-api-key-with-sufficient-entropy"
	sum := sha256.Sum256([]byte(apiKey))

	path := filepath.Join(t.TempDir(), "tenants.json")
	document := "{\n" +
		"  \"schemaVersion\": 1,\n" +
		"  \"tenants\": [\n" +
		"    {\n" +
		"      \"id\": \"client-a\",\n" +
		"      \"name\": \"Client A\",\n" +
		"      \"apiKeySha256\": \"" + hex.EncodeToString(sum[:]) + "\",\n" +
		"      \"enabled\": true\n" +
		"    }\n" +
		"  ]\n" +
		"}\n"
	if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}

	registry, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	authenticated, ok := registry.AuthenticateBearer("Bearer " + apiKey)
	if !ok {
		t.Fatal("expected API key to authenticate")
	}
	if authenticated.ID != "client-a" {
		t.Fatalf("tenant id = %q", authenticated.ID)
	}
}

func TestRegistryRejectsWrongOrDisabledKey(t *testing.T) {
	t.Parallel()

	const apiKey = "tenant-secret-api-key-with-sufficient-entropy"
	sum := sha256.Sum256([]byte(apiKey))

	path := filepath.Join(t.TempDir(), "tenants.json")
	document := "{\n" +
		"  \"schemaVersion\": 1,\n" +
		"  \"tenants\": [\n" +
		"    {\n" +
		"      \"id\": \"client-a\",\n" +
		"      \"name\": \"Client A\",\n" +
		"      \"apiKeySha256\": \"" + hex.EncodeToString(sum[:]) + "\",\n" +
		"      \"enabled\": false\n" +
		"    }\n" +
		"  ]\n" +
		"}\n"
	if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}

	registry, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.AuthenticateBearer("Bearer " + apiKey); ok {
		t.Fatal("disabled tenant must not authenticate")
	}
	if _, ok := registry.AuthenticateBearer("Bearer wrong"); ok {
		t.Fatal("wrong API key must not authenticate")
	}
}
