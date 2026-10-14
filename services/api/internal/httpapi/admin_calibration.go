package httpapi

import (
    "errors"
    "net/http"
    "strconv"
    "strings"
    "time"

    "faceproof/services/api/internal/analytics"
)

type calibrationTruthRequest struct {
    Label string `json:"label"`
    Source string `json:"source"`
}

func (handler *Handler) getAdminCalibration(writer http.ResponseWriter, request *http.Request) {
    days:=30
    if raw:=strings.TrimSpace(request.URL.Query().Get("days"));raw!="" {
        parsed,err:=strconv.Atoi(raw)
        if err!=nil || parsed<1 || parsed>90 {
            handler.writeError(writer,http.StatusBadRequest,"days must be 1 to 90")
            return
        }
        days=parsed
    }
    runtime:=strings.TrimSpace(request.URL.Query().Get("runtime"))
    if len(runtime)>128 {
        handler.writeError(writer,http.StatusBadRequest,"runtime filter is too long")
        return
    }
    until:=time.Now().UTC()
    since:=until.Add(-time.Duration(days)*24*time.Hour)
    report,err:=handler.analytics.Calibration(request.Context(),since,until,runtime)
    if err!=nil {
        handler.logAnalyticsError("load calibration aggregates",err)
        handler.writeError(writer,http.StatusInternalServerError,"failed to load calibration")
        return
    }
    handler.writeJSON(writer,http.StatusOK,report)
}

func (handler *Handler) updateAdminCalibrationTruth(
    writer http.ResponseWriter,request *http.Request,id string,
) {
    var payload calibrationTruthRequest
    if err:=decodeJSON(request,&payload,2048);err!=nil {
        handler.writeError(writer,http.StatusBadRequest,"invalid classification request")
        return
    }
    payload.Label=strings.TrimSpace(payload.Label)
    payload.Source=strings.TrimSpace(payload.Source)
    if err:=analytics.ValidateCalibrationTruth(payload.Label,payload.Source);err!=nil {
        handler.writeError(writer,http.StatusBadRequest,err.Error())
        return
    }
    oldLabel,oldSource,err:=handler.analytics.CalibrationTruth(request.Context(),id)
    if errors.Is(err,analytics.ErrNotFound) {
        handler.writeError(writer,http.StatusNotFound,"CNH verification not found")
        return
    }
    if err!=nil {
        handler.writeError(writer,http.StatusInternalServerError,"failed to load classification")
        return
    }
    if oldLabel != payload.Label || oldSource != payload.Source {
        if err=handler.analytics.UpdateCalibrationTruth(request.Context(),id,payload.Label,payload.Source);err!=nil {
            handler.writeError(writer,http.StatusInternalServerError,"failed to update classification")
            return
        }
        // UpdateCalibrationTruth writes the audit event in the same transaction.
    }
    handler.writeJSON(writer,http.StatusOK,map[string]string{
        "truthLabel":payload.Label,"truthSource":payload.Source,
    })
}
