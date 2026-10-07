package identity

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
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

func TestRepositoryMonthlyQuotaBlocksExcessChecks(t *testing.T) {
	t.Parallel()

	repository, err := NewRepository(
		t.TempDir(),
		bytes.Repeat([]byte{0x52}, 32),
	)
	if err != nil {
		t.Fatal(err)
	}

	createdAt := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	for index := 0; index < 2; index++ {
		check := Check{
			ID:        fmt.Sprintf("chk_quota%04d", index),
			TenantID:  "tenant-a",
			Status:    StatusPendingDocument,
			CreatedAt: createdAt,
			ExpiresAt: createdAt.Add(time.Hour),
		}
		if err := repository.CreateWithMonthlyQuota(
			fmt.Sprintf("token-%d", index),
			check,
			2,
		); err != nil {
			t.Fatalf("create %d: %v", index, err)
		}
	}

	excess := Check{
		ID:        "chk_quota9999",
		TenantID:  "tenant-a",
		Status:    StatusPendingDocument,
		CreatedAt: createdAt,
		ExpiresAt: createdAt.Add(time.Hour),
	}
	if err := repository.CreateWithMonthlyQuota(
		"token-excess",
		excess,
		2,
	); !errors.Is(err, ErrTenantQuotaExceeded) {
		t.Fatalf("expected ErrTenantQuotaExceeded, got %v", err)
	}

	otherTenant := Check{
		ID:        "chk_other999",
		TenantID:  "tenant-b",
		Status:    StatusPendingDocument,
		CreatedAt: createdAt,
		ExpiresAt: createdAt.Add(time.Hour),
	}
	if err := repository.CreateWithMonthlyQuota(
		"token-other",
		otherTenant,
		2,
	); err != nil {
		t.Fatalf("other tenant should have independent quota: %v", err)
	}
}

func TestRepositoryMonthlyQuotaIsAtomicUnderConcurrency(t *testing.T) {
	t.Parallel()

	repository, err := NewRepository(
		t.TempDir(),
		bytes.Repeat([]byte{0x53}, 32),
	)
	if err != nil {
		t.Fatal(err)
	}

	const quota = 5
	const workers = 20
	createdAt := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

	var successes atomic.Int32
	var quotaExceeded atomic.Int32
	var unexpected atomic.Int32
	var wait sync.WaitGroup
	wait.Add(workers)

	for index := 0; index < workers; index++ {
		index := index
		go func() {
			defer wait.Done()
			check := Check{
				ID:        fmt.Sprintf("chk_conc%04d", index),
				TenantID:  "tenant-a",
				Status:    StatusPendingDocument,
				CreatedAt: createdAt,
				ExpiresAt: createdAt.Add(time.Hour),
			}
			err := repository.CreateWithMonthlyQuota(
				fmt.Sprintf("token-concurrent-%d", index),
				check,
				quota,
			)
			switch {
			case err == nil:
				successes.Add(1)
			case errors.Is(err, ErrTenantQuotaExceeded):
				quotaExceeded.Add(1)
			default:
				unexpected.Add(1)
			}
		}()
	}

	wait.Wait()

	if successes.Load() != quota {
		t.Fatalf("successful creates = %d, want %d", successes.Load(), quota)
	}
	if quotaExceeded.Load() != workers-quota {
		t.Fatalf(
			"quota exceeded = %d, want %d",
			quotaExceeded.Load(),
			workers-quota,
		)
	}
	if unexpected.Load() != 0 {
		t.Fatalf("unexpected errors = %d", unexpected.Load())
	}
}

func TestRepositoryMonthlyQuotaResetsByMonth(t *testing.T) {
	t.Parallel()

	repository, err := NewRepository(
		t.TempDir(),
		bytes.Repeat([]byte{0x54}, 32),
	)
	if err != nil {
		t.Fatal(err)
	}

	october := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	november := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

	if err := repository.CreateWithMonthlyQuota(
		"october-token",
		Check{
			ID:        "chk_month001",
			TenantID:  "tenant-a",
			Status:    StatusPendingDocument,
			CreatedAt: october,
			ExpiresAt: october.Add(time.Hour),
		},
		1,
	); err != nil {
		t.Fatal(err)
	}

	if err := repository.CreateWithMonthlyQuota(
		"november-token",
		Check{
			ID:        "chk_month002",
			TenantID:  "tenant-a",
			Status:    StatusPendingDocument,
			CreatedAt: november,
			ExpiresAt: november.Add(time.Hour),
		},
		1,
	); err != nil {
		t.Fatalf("quota should reset in a new month: %v", err)
	}
}
