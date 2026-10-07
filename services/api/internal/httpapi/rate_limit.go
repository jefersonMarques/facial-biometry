package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type identityRateLimitPolicy struct {
	Limit  int
	Window time.Duration
}

var publicIdentityRateLimits = map[string]identityRateLimitPolicy{
	http.MethodGet + " /v1/identity/check": {
		Limit:  120,
		Window: time.Minute,
	},
	http.MethodPost + " /v1/identity/document": {
		Limit:  8,
		Window: 10 * time.Minute,
	},
	http.MethodPost + " /v1/identity/guide": {
		Limit:  240,
		Window: time.Minute,
	},
	http.MethodPost + " /v1/identity/session": {
		Limit:  12,
		Window: 10 * time.Minute,
	},
	http.MethodPost + " /v1/identity/complete": {
		Limit:  12,
		Window: 10 * time.Minute,
	},
}

func (handler *Handler) enforcePublicIdentityRateLimit(
	writer http.ResponseWriter,
	request *http.Request,
) bool {
	if handler.publicLimiter == nil {
		return true
	}

	routeKey := request.Method + " " + request.URL.Path
	policy, exists := publicIdentityRateLimits[routeKey]
	if !exists {
		return true
	}

	token := strings.TrimSpace(
		request.Header.Get("X-FaceProof-Identity-Token"),
	)
	if token == "" {
		return true
	}

	sum := sha256.Sum256([]byte(token))
	key := routeKey + ":" + hex.EncodeToString(sum[:])

	allowed, retryAfter := handler.publicLimiter.Allow(
		key,
		policy.Limit,
		policy.Window,
		time.Now().UTC(),
	)
	if allowed {
		return true
	}

	retrySeconds := int(math.Ceil(retryAfter.Seconds()))
	if retrySeconds < 1 {
		retrySeconds = 1
	}
	writer.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
	handler.writeError(
		writer,
		http.StatusTooManyRequests,
		"too many requests for this identity verification",
	)
	return false
}
