package httpapi

import (
	"net"
	"net/http"
	"strings"

	"faceproof/services/api/internal/domain"
)

func (handler *Handler) technicalResponseAllowed(request *http.Request) bool {
	if !handler.config.Debug {
		return false
	}

	host := strings.TrimSpace(request.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(request.Host)
	}
	if comma := strings.IndexByte(host, ','); comma >= 0 {
		host = strings.TrimSpace(host[:comma])
	}

	hostname := host
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		hostname = parsed
	} else {
		hostname = strings.Trim(host, "[]")
	}

	switch strings.ToLower(hostname) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func (handler *Handler) publicDiagnostics(
	request *http.Request,
	diagnostics []string,
) []string {
	if !handler.technicalResponseAllowed(request) {
		return []string{}
	}
	return append([]string(nil), diagnostics...)
}

func (handler *Handler) publicNativeShadow(
	request *http.Request,
	shadow *domain.NativeShadowComparison,
) *domain.NativeShadowComparison {
	if !handler.technicalResponseAllowed(request) {
		return nil
	}
	return shadow
}
