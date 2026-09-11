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
	ErrSessionCompleted = errors.New("session already completed")
)

type Store struct {
	mu       sync.RWMutex
	sessions map[string]domain.CaptureSession
}

func NewStore() *Store {
	return &Store{sessions: make(map[string]domain.CaptureSession)}
}

func (store *Store) Put(session domain.CaptureSession) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.sessions[session.ID] = session
}

func (store *Store) Get(id string) (domain.CaptureSession, error) {
	store.mu.RLock()
	session, exists := store.sessions[id]
	store.mu.RUnlock()
	if !exists {
		return domain.CaptureSession{}, ErrSessionNotFound
	}
	if time.Now().After(session.ExpiresAt) {
		return domain.CaptureSession{}, ErrSessionExpired
	}
	if session.Completed {
		return domain.CaptureSession{}, ErrSessionCompleted
	}
	return session, nil
}

func (store *Store) MarkCompleted(id string) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	session, exists := store.sessions[id]
	if !exists {
		return ErrSessionNotFound
	}
	session.Completed = true
	store.sessions[id] = session
	return nil
}
