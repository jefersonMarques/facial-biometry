package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"faceproof/services/api/internal/analytics"
	"faceproof/services/api/internal/security"
)

type createAnalyticsCampaignRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func (handler *Handler) handleAdmin(writer http.ResponseWriter, request *http.Request) bool {
	if request.URL.Path != "/v1/admin" && !strings.HasPrefix(request.URL.Path, "/v1/admin/") {
		return false
	}
	if handler.analytics == nil {
		handler.writeError(writer, http.StatusServiceUnavailable, "analytics is unavailable")
		return true
	}
	if !handler.authorizeAdmin(request) {
		handler.writeError(writer, http.StatusUnauthorized, "invalid admin credentials")
		return true
	}

	switch {
	case request.URL.Path == "/v1/admin/summary" && request.Method == http.MethodGet:
		handler.getAdminSummary(writer, request)
	case request.URL.Path == "/v1/admin/checks" && request.Method == http.MethodGet:
		handler.listAdminChecks(writer, request)
	case request.URL.Path == "/v1/admin/campaigns" && request.Method == http.MethodGet:
		handler.listAdminCampaigns(writer, request)
	case request.URL.Path == "/v1/admin/campaigns" && request.Method == http.MethodPost:
		handler.createAdminCampaign(writer, request)
	default:
		segments := splitPath(request.URL.Path)
		if len(segments) == 4 &&
			segments[0] == "v1" &&
			segments[1] == "admin" &&
			segments[2] == "checks" &&
			request.Method == http.MethodGet {
			handler.getAdminCheck(writer, request, segments[3])
			break
		}
		handler.writeError(writer, http.StatusNotFound, "admin route not found")
	}
	return true
}

func (handler *Handler) authorizeAdmin(request *http.Request) bool {
	if len(handler.config.AdminKey) == 0 {
		return false
	}
	value := strings.TrimSpace(request.Header.Get("Authorization"))
	const prefix = "Bearer "
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	provided := []byte(strings.TrimSpace(strings.TrimPrefix(value, prefix)))
	return constantTimeEqual(provided, handler.config.AdminKey)
}

func constantTimeEqual(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var difference byte
	for index := range left {
		difference |= left[index] ^ right[index]
	}
	return difference == 0
}

func (handler *Handler) getAdminSummary(writer http.ResponseWriter, request *http.Request) {
	summary, err := handler.analytics.Summary(request.Context())
	if err != nil {
		handler.writeError(writer, http.StatusInternalServerError, "failed to load analytics summary")
		return
	}
	handler.writeJSON(writer, http.StatusOK, summary)
}

func (handler *Handler) listAdminChecks(writer http.ResponseWriter, request *http.Request) {
	limit := 100
	if value := strings.TrimSpace(request.URL.Query().Get("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			handler.writeError(writer, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = parsed
	}

	items, err := handler.analytics.ListChecks(request.Context(), analytics.CheckFilter{
		Status:     strings.TrimSpace(request.URL.Query().Get("status")),
		Scenario:   strings.TrimSpace(request.URL.Query().Get("scenario")),
		CampaignID: strings.TrimSpace(request.URL.Query().Get("campaignId")),
		Limit:      limit,
	})
	if err != nil {
		handler.writeError(writer, http.StatusInternalServerError, "failed to load analytics checks")
		return
	}
	handler.writeJSON(writer, http.StatusOK, map[string]any{"items": items})
}

func (handler *Handler) getAdminCheck(
	writer http.ResponseWriter,
	request *http.Request,
	checkID string,
) {
	detail, err := handler.analytics.GetCheck(request.Context(), checkID)
	if errors.Is(err, analytics.ErrNotFound) {
		handler.writeError(writer, http.StatusNotFound, "analytics check not found")
		return
	}
	if err != nil {
		handler.writeError(writer, http.StatusInternalServerError, "failed to load analytics check")
		return
	}
	handler.writeJSON(writer, http.StatusOK, detail)
}

func (handler *Handler) listAdminCampaigns(writer http.ResponseWriter, request *http.Request) {
	campaigns, err := handler.analytics.ListCampaigns(request.Context())
	if err != nil {
		handler.writeError(writer, http.StatusInternalServerError, "failed to load campaigns")
		return
	}
	handler.writeJSON(writer, http.StatusOK, map[string]any{"items": campaigns})
}

func (handler *Handler) createAdminCampaign(writer http.ResponseWriter, request *http.Request) {
	var payload createAnalyticsCampaignRequest
	if err := decodeJSON(request, &payload, 1<<20); err != nil {
		handler.writeError(writer, http.StatusBadRequest, err.Error())
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	payload.Description = strings.TrimSpace(payload.Description)
	if len(payload.Name) < 2 || len(payload.Name) > 120 {
		handler.writeError(writer, http.StatusBadRequest, "campaign name must contain between 2 and 120 characters")
		return
	}
	if len(payload.Description) > 500 {
		handler.writeError(writer, http.StatusBadRequest, "campaign description is too long")
		return
	}

	randomID, err := security.RandomID(10)
	if err != nil {
		handler.writeError(writer, http.StatusInternalServerError, "failed to create campaign")
		return
	}
	campaign := analytics.Campaign{
		ID:          "cmp_" + randomID,
		Name:        payload.Name,
		Description: payload.Description,
		CreatedAt:   time.Now().UTC(),
	}
	if err := handler.analytics.CreateCampaign(request.Context(), campaign); err != nil {
		handler.writeError(writer, http.StatusInternalServerError, "failed to persist campaign")
		return
	}

	handler.writeJSON(writer, http.StatusCreated, campaign)
}
