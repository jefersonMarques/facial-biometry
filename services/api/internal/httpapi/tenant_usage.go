package httpapi

import (
	"net/http"
	"time"
)

func (handler *Handler) getTenantUsage(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if handler.analytics == nil {
		handler.writeError(
			writer,
			http.StatusServiceUnavailable,
			"tenant usage analytics is unavailable",
		)
		return
	}

	tenantID, authorized := handler.authorizeIdentityIssuer(request)
	if !authorized {
		handler.writeError(
			writer,
			http.StatusUnauthorized,
			"invalid issuer credentials",
		)
		return
	}

	now := time.Now().UTC()
	periodFrom := time.Date(
		now.Year(),
		now.Month(),
		1,
		0,
		0,
		0,
		0,
		time.UTC,
	)
	periodTo := periodFrom.AddDate(0, 1, 0)

	operationContext, cancel := analyticsOperationContext(request.Context())
	defer cancel()

	usage, err := handler.analytics.TenantUsage(
		operationContext,
		tenantID,
		periodFrom,
		periodTo,
	)
	if err != nil {
		handler.writeError(
			writer,
			http.StatusInternalServerError,
			"failed to load tenant usage",
		)
		return
	}

	handler.writeJSON(writer, http.StatusOK, usage)
}
