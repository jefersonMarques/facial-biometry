package httpapi

import (
	"net/http"
)

const (
	minConfirmedViewMS = 3000
	maxConfirmedViewMS = 5 * 60 * 1000
)

type identityViewRequest struct {
	VisibleMS int `json:"visibleMs"`
}

func (handler *Handler) confirmIdentityView(
	writer http.ResponseWriter,
	request *http.Request,
) {
	check, _, ok := handler.loadPublicIdentityCheck(writer, request)
	if !ok {
		return
	}

	var payload identityViewRequest
	if err := decodeJSON(request, &payload, 4<<10); err != nil {
		handler.writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	if payload.VisibleMS < minConfirmedViewMS || payload.VisibleMS > maxConfirmedViewMS {
		handler.writeError(
			writer,
			http.StatusBadRequest,
			"visibleMs is outside the accepted range",
		)
		return
	}

	handler.recordAnalyticsViewConfirmed(
		request.Context(),
		check.ID,
		payload.VisibleMS,
	)

	handler.writeJSON(writer, http.StatusOK, map[string]string{
		"status": "confirmed",
	})
}
