package security

import (
	"sync"
	"time"
)

type WindowLimiter struct {
	mu         sync.Mutex
	entries    map[string]windowCounter
	maxEntries int
}

type windowCounter struct {
	Count     int
	ExpiresAt time.Time
}

func NewWindowLimiter(maxEntries int) *WindowLimiter {
	if maxEntries <= 0 {
		maxEntries = 4096
	}
	return &WindowLimiter{
		entries:    make(map[string]windowCounter),
		maxEntries: maxEntries,
	}
}

func (limiter *WindowLimiter) Allow(
	key string,
	limit int,
	window time.Duration,
	now time.Time,
) (bool, time.Duration) {
	if limiter == nil || key == "" || limit <= 0 || window <= 0 {
		return true, 0
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	if len(limiter.entries) >= limiter.maxEntries {
		limiter.cleanupExpired(now)
		if len(limiter.entries) >= limiter.maxEntries {
			for candidate := range limiter.entries {
				delete(limiter.entries, candidate)
				break
			}
		}
	}

	counter, exists := limiter.entries[key]
	if !exists || !now.Before(counter.ExpiresAt) {
		limiter.entries[key] = windowCounter{
			Count:     1,
			ExpiresAt: now.Add(window),
		}
		return true, 0
	}

	if counter.Count >= limit {
		retryAfter := time.Until(counter.ExpiresAt)
		if !now.Equal(time.Now()) {
			retryAfter = counter.ExpiresAt.Sub(now)
		}
		if retryAfter < 0 {
			retryAfter = 0
		}
		return false, retryAfter
	}

	counter.Count++
	limiter.entries[key] = counter
	return true, 0
}

func (limiter *WindowLimiter) cleanupExpired(now time.Time) {
	for key, counter := range limiter.entries {
		if !now.Before(counter.ExpiresAt) {
			delete(limiter.entries, key)
		}
	}
}
