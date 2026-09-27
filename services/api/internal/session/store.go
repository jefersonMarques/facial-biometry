package session

import (
	"errors"
	"sync"
	"time"

	"faceproof/services/api/internal/domain"
)

var (
	ErrSessionNotFound  = errors.New("session not found")
	ErrSessionExpired   = errors.New("session expired")
	ErrSessionCompleted = errors.New("session already consumed")
)

type Store struct {
	mu       sync.RWMutex
	sessions map[string]domain.CaptureSession
}

func NewStore() *Store {
	return &Store{sessions: make(map[string]domain.CaptureSession)}
}

func (store *Store) Put(captureSession domain.CaptureSession) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.sessions[captureSession.ID] = captureSession
}

func (store *Store) Get(id string) (domain.CaptureSession, error) {
	store.mu.RLock()
	captureSession, exists := store.sessions[id]
	store.mu.RUnlock()
	return validateSession(captureSession, exists)
}

func (store *Store) Consume(id string) (domain.CaptureSession, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	captureSession, exists := store.sessions[id]
	captureSession, err := validateSession(captureSession, exists)
	if err != nil {
		return domain.CaptureSession{}, err
	}

	captureSession.Completed = true
	store.sessions[id] = captureSession
	return captureSession, nil
}

func validateSession(captureSession domain.CaptureSession, exists bool) (domain.CaptureSession, error) {
	if !exists {
		return domain.CaptureSession{}, ErrSessionNotFound
	}
	if time.Now().After(captureSession.ExpiresAt) {
		return domain.CaptureSession{}, ErrSessionExpired
	}
	if captureSession.Completed {
		return domain.CaptureSession{}, ErrSessionCompleted
	}
	return captureSession, nil
}
