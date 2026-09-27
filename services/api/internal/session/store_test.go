package session

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"faceproof/services/api/internal/domain"
)

func TestConsumeAllowsSingleConsumer(t *testing.T) {
	store := NewStore()
	store.Put(domain.CaptureSession{
		ID:        "session-1",
		ExpiresAt: time.Now().Add(time.Minute),
	})

	var successfulConsumes int32
	var waitGroup sync.WaitGroup
	for index := 0; index < 16; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			if _, err := store.Consume("session-1"); err == nil {
				atomic.AddInt32(&successfulConsumes, 1)
			}
		}()
	}
	waitGroup.Wait()

	if successfulConsumes != 1 {
		t.Fatalf("expected exactly one successful consume, got %d", successfulConsumes)
	}
}

func TestConsumeRejectsAlreadyConsumedSession(t *testing.T) {
	store := NewStore()
	store.Put(domain.CaptureSession{
		ID:        "session-1",
		ExpiresAt: time.Now().Add(time.Minute),
	})

	if _, err := store.Consume("session-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Consume("session-1"); !errors.Is(err, ErrSessionCompleted) {
		t.Fatalf("expected ErrSessionCompleted, got %v", err)
	}
}
