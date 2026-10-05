package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sort"
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

type createAdminIdentityCheckRequest struct {
	CPF                 string `json:"cpf"`
	MinimumDocumentDate string `json:"minimumDocumentDate"`
	ExpiresInMinutes    int    `json:"expiresInMinutes,omitempty"`
	CampaignID          string `json:"campaignId,omitempty"`
	Scenario            string `json:"scenario,omitempty"`
	ExpectedDecision    string `json:"expectedDecision,omitempty"`
	PublicBaseURL       string `json:"publicBaseUrl,omitempty"`
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
	case request.URL.Path == "/v1/admin/checks" && request.Method == http.MethodPost:
		handler.createAdminIdentityCheck(writer, request)
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

func (handler *Handler) reconcileAdminAnalytics(request *http.Request) {
	if handler.analytics == nil || handler.identityChecks == nil {
		return
	}

	states, err := handler.analytics.ListOpenCheckStates(request.Context(), 100)
	if err != nil {
		handler.logAnalyticsError("list open checks for reconciliation", err)
		return
	}

	for _, state := range states {
		check, err := handler.identityChecks.LoadByID(state.CheckID)
		if err != nil {
			continue
		}
		if string(check.Status) == state.Status && check.CompletedAt == nil {
			continue
		}

		operationContext, cancel := analyticsOperationContext(request.Context())
		err = handler.analytics.RecordAuthoritativeState(
			operationContext,
			check.ID,
			string(check.Status),
			check.Decision,
			check.CompletedAt,
			check.LivenessScore,
			check.FaceSimilarity,
			check.BiometricSessions,
		)
		cancel()
		if err != nil {
			handler.logAnalyticsError("reconcile authoritative identity state", err)
		}
	}
}

func (handler *Handler) applyAuthoritativeCheckItem(
	item *analytics.CheckListItem,
) {
	if handler.identityChecks == nil || item == nil {
		return
	}
	check, err := handler.identityChecks.LoadByID(item.CheckID)
	if err != nil {
		return
	}
	item.Status = string(check.Status)
	item.Decision = check.Decision
	item.CompletedAt = check.CompletedAt
	if check.FaceSimilarity != 0 {
		value := check.FaceSimilarity
		item.FaceSimilarity = &value
	}
	if check.LivenessScore != 0 {
		value := check.LivenessScore
		item.LivenessScore = &value
	}
}

func (handler *Handler) authoritativeSummary(
	ctx context.Context,
	base analytics.DashboardSummary,
) analytics.DashboardSummary {
	if handler.analytics == nil || handler.identityChecks == nil {
		return base
	}

	rows, err := handler.analytics.ListSummaryCheckRows(ctx)
	if err != nil {
		handler.logAnalyticsError("load summary rows", err)
		return base
	}

	base.Total = int64(len(rows))
	base.Approved = 0
	base.Review = 0
	base.Rejected = 0
	base.Expired = 0
	base.Pending = 0
	base.Completed = 0
	base.AvgFaceSimilarity = 0
	base.AvgLiveness = 0

	scenarios := map[string]*analytics.ScenarioSummary{}
	var faceSum, livenessSum float64
	var faceCount, livenessCount int64

	for index := range rows {
		row := &rows[index]
		if check, loadErr := handler.identityChecks.LoadByID(row.CheckID); loadErr == nil {
			row.Status = string(check.Status)
			row.Decision = check.Decision
			row.CompletedAt = check.CompletedAt
			if check.FaceSimilarity != 0 {
				value := check.FaceSimilarity
				row.FaceSimilarity = &value
			}
			if check.LivenessScore != 0 {
				value := check.LivenessScore
				row.LivenessScore = &value
			}
		}

		switch row.Status {
		case "approved":
			base.Approved++
		case "review":
			base.Review++
		case "rejected":
			base.Rejected++
		case "expired":
			base.Expired++
		default:
			base.Pending++
		}
		if row.CompletedAt != nil {
			base.Completed++
		}
		if row.FaceSimilarity != nil {
			faceSum += *row.FaceSimilarity
			faceCount++
		}
		if row.LivenessScore != nil {
			livenessSum += *row.LivenessScore
			livenessCount++
		}

		scenario := scenarios[row.Scenario]
		if scenario == nil {
			scenario = &analytics.ScenarioSummary{Scenario: row.Scenario}
			scenarios[row.Scenario] = scenario
		}
		scenario.Total++
		switch row.Decision {
		case "approved":
			scenario.Approved++
		case "review":
			scenario.Review++
		case "rejected":
			scenario.Rejected++
		}
		if row.ExpectedDecision != "" && row.Decision != "" {
			scenario.ExpectedEvaluated++
			if row.ExpectedDecision == row.Decision {
				scenario.ExpectedCorrect++
			}
		}
	}

	if base.Total > 0 {
		base.CompletionRate = float64(base.Completed) / float64(base.Total)
	} else {
		base.CompletionRate = 0
	}
	if faceCount > 0 {
		base.AvgFaceSimilarity = faceSum / float64(faceCount)
	}
	if livenessCount > 0 {
		base.AvgLiveness = livenessSum / float64(livenessCount)
	}

	base.Scenarios = base.Scenarios[:0]
	for _, scenario := range scenarios {
		base.Scenarios = append(base.Scenarios, *scenario)
	}
	sort.Slice(base.Scenarios, func(i, j int) bool {
		if base.Scenarios[i].Total == base.Scenarios[j].Total {
			return base.Scenarios[i].Scenario < base.Scenarios[j].Scenario
		}
		return base.Scenarios[i].Total > base.Scenarios[j].Total
	})

	return base
}

func (handler *Handler) getAdminSummary(writer http.ResponseWriter, request *http.Request) {
	handler.reconcileAdminAnalytics(request)
	summary, err := handler.analytics.Summary(request.Context())
	if err != nil {
		handler.writeError(writer, http.StatusInternalServerError, "failed to load analytics summary")
		return
	}
	summary = handler.authoritativeSummary(request.Context(), summary)
	handler.writeJSON(writer, http.StatusOK, summary)
}

func (handler *Handler) createAdminIdentityCheck(writer http.ResponseWriter, request *http.Request) {
	var payload createAdminIdentityCheckRequest
	if err := decodeJSON(request, &payload, 1<<20); err != nil {
		handler.writeError(writer, http.StatusBadRequest, err.Error())
		return
	}

	publicBaseURL := strings.TrimSpace(payload.PublicBaseURL)
	if publicBaseURL != "" {
		if _, err := parsePublicBaseURL(publicBaseURL); err != nil {
			handler.writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
	}

	response, err := handler.createIdentityCheckRecord(request.Context(), createIdentityCheckRequest{
		CPF:                 payload.CPF,
		MinimumDocumentDate: payload.MinimumDocumentDate,
		ExpiresInMinutes:    payload.ExpiresInMinutes,
		CampaignID:          payload.CampaignID,
		Scenario:            payload.Scenario,
		ExpectedDecision:    payload.ExpectedDecision,
	})
	if err != nil {
		writeIdentityCreateError(handler, writer, err)
		return
	}

	if publicBaseURL != "" {
		publicURL, err := publicVerificationURL(publicBaseURL, response.VerificationURL)
		if err != nil {
			handler.writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		response.VerificationURL = publicURL
	}

	handler.writeJSON(writer, http.StatusCreated, response)
}

func publicVerificationURL(publicBaseURL, verificationURL string) (string, error) {
	parsed, err := parsePublicBaseURL(publicBaseURL)
	if err != nil {
		return "", err
	}

	fragmentIndex := strings.IndexByte(verificationURL, '#')
	if fragmentIndex < 0 || fragmentIndex == len(verificationURL)-1 {
		return "", errors.New("verification URL does not contain an identity token")
	}

	basePath := strings.TrimRight(parsed.Path, "/")
	if basePath == "/verify.html" || strings.HasSuffix(basePath, "/verify.html") {
		parsed.Path = basePath
	} else {
		parsed.Path = basePath + "/verify.html"
	}
	parsed.RawPath = ""
	parsed.Fragment = verificationURL[fragmentIndex+1:]
	return parsed.String(), nil
}

func parsePublicBaseURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, errors.New("publicBaseUrl must be an absolute http or https URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("publicBaseUrl must not contain credentials, query or fragment")
	}
	return parsed, nil
}

func (handler *Handler) listAdminChecks(writer http.ResponseWriter, request *http.Request) {
	handler.reconcileAdminAnalytics(request)
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
		Status:     "",
		Scenario:   strings.TrimSpace(request.URL.Query().Get("scenario")),
		CampaignID: strings.TrimSpace(request.URL.Query().Get("campaignId")),
		Limit:      limit,
	})
	if err != nil {
		handler.writeError(writer, http.StatusInternalServerError, "failed to load analytics checks")
		return
	}

	statusFilter := strings.TrimSpace(request.URL.Query().Get("status"))
	filtered := items[:0]
	for index := range items {
		handler.applyAuthoritativeCheckItem(&items[index])
		if statusFilter != "" && items[index].Status != statusFilter {
			continue
		}
		filtered = append(filtered, items[index])
	}

	handler.writeJSON(writer, http.StatusOK, map[string]any{"items": filtered})
}

func (handler *Handler) getAdminCheck(
	writer http.ResponseWriter,
	request *http.Request,
	checkID string,
) {
	handler.reconcileAdminAnalytics(request)
	detail, err := handler.analytics.GetCheck(request.Context(), checkID)
	if errors.Is(err, analytics.ErrNotFound) {
		handler.writeError(writer, http.StatusNotFound, "analytics check not found")
		return
	}
	if err != nil {
		handler.writeError(writer, http.StatusInternalServerError, "failed to load analytics check")
		return
	}
	item := detail.CheckListItem
	handler.applyAuthoritativeCheckItem(&item)
	detail.CheckListItem = item
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
