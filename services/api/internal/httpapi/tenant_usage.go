package httpapi

import (
	"net/http"
	"time"

	"faceproof/services/api/internal/analytics"
)

type tenantUsageResponse struct {
	analytics.TenantUsage
	MonthlyCheckLimit int  `json:"monthlyCheckLimit"`
	RemainingChecks   *int `json:"remainingChecks,omitempty"`
	AnalyticsAvailable bool `json:"analyticsAvailable"`
}

func (handler *Handler) getTenantUsage(
	writer http.ResponseWriter,
	request *http.Request,
) {
	authenticatedTenant, authorized := handler.authorizeIdentityIssuer(request)
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

	authoritativeIssued, err := handler.identityChecks.CountTenantChecks(
		authenticatedTenant.ID,
		periodFrom,
		periodTo,
	)
	if err != nil {
		handler.writeError(
			writer,
			http.StatusInternalServerError,
			"failed to load authoritative tenant usage",
		)
		return
	}

	usage := analytics.TenantUsage{
		TenantID:   authenticatedTenant.ID,
		PeriodFrom: periodFrom,
		PeriodTo:   periodTo,
		Issued:     int64(authoritativeIssued),
	}
	analyticsAvailable := handler.analytics != nil
	if analyticsAvailable {
		operationContext, cancel := analyticsOperationContext(request.Context())
		analyticsUsage, analyticsErr := handler.analytics.TenantUsage(
			operationContext,
			authenticatedTenant.ID,
			periodFrom,
			periodTo,
		)
		cancel()
		if analyticsErr != nil {
			handler.logAnalyticsError("load tenant usage", analyticsErr)
			analyticsAvailable = false
		} else {
			analyticsUsage.Issued = int64(authoritativeIssued)
			usage = analyticsUsage
		}
	}

	var remaining *int
	if authenticatedTenant.MonthlyCheckLimit > 0 {
		value := authenticatedTenant.MonthlyCheckLimit - authoritativeIssued
		if value < 0 {
			value = 0
		}
		remaining = &value
	}

	handler.writeJSON(writer, http.StatusOK, tenantUsageResponse{
		TenantUsage:       usage,
		MonthlyCheckLimit: authenticatedTenant.MonthlyCheckLimit,
		RemainingChecks:   remaining,
		AnalyticsAvailable: analyticsAvailable,
	})
}
