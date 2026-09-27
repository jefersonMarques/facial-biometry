package httpapi

import (
	"errors"
	"time"

	"faceproof/services/api/internal/domain"
	"faceproof/services/api/internal/security"
)

func (handler *Handler) issueCaptureSession(subjectID string, kind domain.SessionKind) (createSessionResponse, error) {
	sessionID, err := security.RandomID(18)
	if err != nil {
		return createSessionResponse{}, err
	}
	illuminationPattern, err := randomIlluminationPattern(illuminationSteps)
	if err != nil {
		return createSessionResponse{}, err
	}

	now := time.Now().UTC()
	captureSession := domain.CaptureSession{
		ID:                   sessionID,
		SubjectID:            subjectID,
		Kind:                 kind,
		IlluminationPattern:  illuminationPattern,
		CaptureDurationMS:    captureDurationMS,
		SampleIntervalMS:     sampleIntervalMS,
		IlluminationSettleMS: illuminationSettleMS,
		CreatedAt:            now,
		ExpiresAt:            now.Add(handler.config.SessionTTL),
	}

	token, err := handler.signer.Sign(captureSession.ID, captureSession.ExpiresAt)
	if err != nil {
		return createSessionResponse{}, err
	}

	handler.sessions.Put(captureSession)
	return createSessionResponse{
		SessionID:            captureSession.ID,
		SessionToken:         token,
		Kind:                 string(captureSession.Kind),
		ExpiresAt:            captureSession.ExpiresAt,
		CaptureDurationMS:    captureSession.CaptureDurationMS,
		SampleIntervalMS:     captureSession.SampleIntervalMS,
		IlluminationSettleMS: captureSession.IlluminationSettleMS,
		IlluminationPattern:  captureSession.IlluminationPattern,
	}, nil
}

func validateIdentityCaptureSession(captureSession domain.CaptureSession, checkID string) error {
	if captureSession.Kind != domain.SessionKindIdentity {
		return errors.New("identity capture session kind mismatch")
	}
	if captureSession.SubjectID != checkID {
		return errors.New("identity capture session subject mismatch")
	}
	return nil
}
