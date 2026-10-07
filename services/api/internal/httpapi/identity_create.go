package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"faceproof/services/api/internal/document/cnh"
	"faceproof/services/api/internal/identity"
	"faceproof/services/api/internal/security"
)

type identityCreateError struct {
	Status  int
	Message string
}

func (failure *identityCreateError) Error() string {
	return failure.Message
}

func (handler *Handler) createIdentityCheckRecord(
	ctx context.Context,
	tenantID string,
	payload createIdentityCheckRequest,
) (createIdentityCheckResponse, error) {
	if handler.identityChecks == nil || handler.cnhDocuments == nil {
		return createIdentityCheckResponse{}, &identityCreateError{
			Status:  http.StatusServiceUnavailable,
			Message: "identity verification is unavailable",
		}
	}

	expectedCPF := cnh.NormalizeCPF(payload.CPF)
	if !cnh.ValidCPF(expectedCPF) {
		return createIdentityCheckResponse{}, &identityCreateError{
			Status:  http.StatusBadRequest,
			Message: "cpf is invalid",
		}
	}

	scenario, ok := normalizeAnalyticsScenario(payload.Scenario)
	if !ok {
		return createIdentityCheckResponse{}, &identityCreateError{
			Status:  http.StatusBadRequest,
			Message: "scenario is invalid",
		}
	}
	expectedDecision, ok := normalizeExpectedDecision(payload.ExpectedDecision, scenario)
	if !ok {
		return createIdentityCheckResponse{}, &identityCreateError{
			Status:  http.StatusBadRequest,
			Message: "expectedDecision is invalid",
		}
	}

	campaignID := strings.TrimSpace(payload.CampaignID)
	campaignExists, err := handler.analyticsCampaignExists(ctx, campaignID)
	if err != nil {
		return createIdentityCheckResponse{}, &identityCreateError{
			Status:  http.StatusServiceUnavailable,
			Message: "analytics campaign lookup failed",
		}
	}
	if !campaignExists {
		return createIdentityCheckResponse{}, &identityCreateError{
			Status:  http.StatusBadRequest,
			Message: "campaignId is invalid or analytics is unavailable",
		}
	}

	location := time.FixedZone("America/Sao_Paulo", -3*60*60)
	minimumDate, err := time.ParseInLocation(
		"2006-01-02",
		strings.TrimSpace(payload.MinimumDocumentDate),
		location,
	)
	if err != nil {
		return createIdentityCheckResponse{}, &identityCreateError{
			Status:  http.StatusBadRequest,
			Message: "minimumDocumentDate must use YYYY-MM-DD",
		}
	}
	today := time.Now().In(location)
	todayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, location)
	if minimumDate.After(todayStart) {
		return createIdentityCheckResponse{}, &identityCreateError{
			Status:  http.StatusBadRequest,
			Message: "minimumDocumentDate cannot be in the future",
		}
	}

	linkTTL := handler.config.IdentityLinkTTL
	if payload.ExpiresInMinutes != 0 {
		if payload.ExpiresInMinutes < 5 || payload.ExpiresInMinutes > 24*60 {
			return createIdentityCheckResponse{}, &identityCreateError{
				Status:  http.StatusBadRequest,
				Message: "expiresInMinutes must be between 5 and 1440",
			}
		}
		linkTTL = time.Duration(payload.ExpiresInMinutes) * time.Minute
	}

	idValue, err := security.RandomID(12)
	if err != nil {
		return createIdentityCheckResponse{}, errors.New("failed to create identity check")
	}
	token, err := security.RandomID(32)
	if err != nil {
		return createIdentityCheckResponse{}, errors.New("failed to create identity check")
	}

	now := time.Now().UTC()
	check := identity.Check{
		ID:                  "chk_" + idValue,
		TenantID:            strings.TrimSpace(strings.ToLower(tenantID)),
		ExpectedCPF:         expectedCPF,
		MinimumDocumentDate: minimumDate.UTC(),
		CampaignID:          campaignID,
		Scenario:            scenario,
		ExpectedDecision:    expectedDecision,
		Status:              identity.StatusPendingDocument,
		CreatedAt:           now,
		ExpiresAt:           now.Add(linkTTL),
	}
	if err := handler.identityChecks.Create(token, check); err != nil {
		return createIdentityCheckResponse{}, errors.New("failed to persist identity check")
	}
	handler.recordAnalyticsCheckCreated(ctx, check)

	baseURL := strings.Split(strings.TrimSpace(handler.config.IdentityVerifyURL), "#")[0]
	return createIdentityCheckResponse{
		ID:               check.ID,
		VerificationURL:  baseURL + "#identity=" + token,
		ExpiresAt:        check.ExpiresAt,
		CampaignID:       check.CampaignID,
		Scenario:         check.Scenario,
		ExpectedDecision: check.ExpectedDecision,
	}, nil
}

func writeIdentityCreateError(
	handler *Handler,
	writer http.ResponseWriter,
	err error,
) {
	var createError *identityCreateError
	if errors.As(err, &createError) {
		handler.writeError(writer, createError.Status, createError.Message)
		return
	}
	handler.writeError(writer, http.StatusInternalServerError, err.Error())
}
