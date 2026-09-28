package identity

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRepositoryEncryptsSensitiveStateAndLoadsByID(t *testing.T) {
	directory := t.TempDir()
	key := bytes.Repeat([]byte{0x42}, 32)
	repository, err := NewRepository(directory, key)
	if err != nil {
		t.Fatal(err)
	}

	check := Check{
		ID:                  "chk_test12345",
		ExpectedCPF:         "39053344705",
		MinimumDocumentDate: time.Date(2026, 9, 25, 3, 0, 0, 0, time.UTC),
		Status:              StatusPendingDocument,
		CreatedAt:           time.Now().UTC(),
		ExpiresAt:           time.Now().UTC().Add(time.Hour),
	}
	const token = "identity-token-that-must-never-be-stored"

	if err := repository.Create(token, check); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte(check.ExpectedCPF)) || bytes.Contains(data, []byte(token)) {
			t.Fatal("repository leaked plaintext CPF or public token")
		}
	}

	loaded, err := repository.LoadByID(check.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ExpectedCPF != check.ExpectedCPF || loaded.ID != check.ID {
		t.Fatalf("unexpected loaded check: %+v", loaded)
	}
}

func TestRepositoryUpdateByID(t *testing.T) {
	repository, err := NewRepository(t.TempDir(), bytes.Repeat([]byte{0x31}, 32))
	if err != nil {
		t.Fatal(err)
	}
	check := Check{
		ID:                 "chk_update123",
		Status:             StatusBiometryPending,
		ReferenceEmbedding: []float64{0.1, 0.2},
		ExpiresAt:          time.Now().UTC().Add(-time.Minute),
	}
	if err := repository.Create("public-token", check); err != nil {
		t.Fatal(err)
	}

	updated, err := repository.UpdateByID(check.ID, func(current *Check) error {
		current.Status = StatusExpired
		current.ReferenceEmbedding = nil
		current.ReferenceEmbeddingModel = ""
		current.CaptureSessionID = ""
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != StatusExpired || len(updated.ReferenceEmbedding) != 0 {
		t.Fatalf("unexpected updated check: %+v", updated)
	}

	loaded, err := repository.Load("public-token")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != StatusExpired || len(loaded.ReferenceEmbedding) != 0 {
		t.Fatalf("expiration was not persisted: %+v", loaded)
	}
}

func TestRepositoryRejectsUnsafeCheckID(t *testing.T) {
	repository, err := NewRepository(t.TempDir(), bytes.Repeat([]byte{0x24}, 32))
	if err != nil {
		t.Fatal(err)
	}

	err = repository.Create("token", Check{ID: "../escape"})
	if err == nil {
		t.Fatal("expected unsafe check id to be rejected")
	}
}
