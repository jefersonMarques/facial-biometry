package security

import (
	"testing"
	"time"
)

func TestWindowLimiterBlocksAfterLimitAndResets(t *testing.T) {
	t.Parallel()

	limiter := NewWindowLimiter(16)
	now := time.Unix(1_700_000_000, 0)

	for attempt := 0; attempt < 3; attempt++ {
		allowed, retryAfter := limiter.Allow("identity:abc", 3, time.Minute, now)
		if !allowed {
			t.Fatalf("attempt %d unexpectedly blocked", attempt+1)
		}
		if retryAfter != 0 {
			t.Fatalf("attempt %d retryAfter = %s", attempt+1, retryAfter)
		}
	}

	allowed, retryAfter := limiter.Allow("identity:abc", 3, time.Minute, now)
	if allowed {
		t.Fatal("fourth attempt should be blocked")
	}
	if retryAfter != time.Minute {
		t.Fatalf("retryAfter = %s", retryAfter)
	}

	allowed, retryAfter = limiter.Allow(
		"identity:abc",
		3,
		time.Minute,
		now.Add(time.Minute),
	)
	if !allowed {
		t.Fatal("window should reset after expiry")
	}
	if retryAfter != 0 {
		t.Fatalf("retryAfter after reset = %s", retryAfter)
	}
}

func TestWindowLimiterSeparatesKeys(t *testing.T) {
	t.Parallel()

	limiter := NewWindowLimiter(16)
	now := time.Unix(1_700_000_000, 0)

	if allowed, _ := limiter.Allow("a", 1, time.Minute, now); !allowed {
		t.Fatal("first key should be allowed")
	}
	if allowed, _ := limiter.Allow("b", 1, time.Minute, now); !allowed {
		t.Fatal("second key should be independent")
	}
	if allowed, _ := limiter.Allow("a", 1, time.Minute, now); allowed {
		t.Fatal("first key should now be blocked")
	}
}
