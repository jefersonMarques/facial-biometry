//go:build !linux || !cgo

package engine

import (
	"context"
	"errors"

	"faceproof/services/api/internal/domain"
)

type NativeClient struct{}

func NewNativeClient(configuration NativeConfig) (*NativeClient, error) {
	return nil, errors.New("FaceProof Secure Core requires Linux with cgo enabled")
}

func (client *NativeClient) Close() error {
	return nil
}

func (client *NativeClient) AnalyzeIdentity(
	ctx context.Context,
	request domain.EngineIdentityRequest,
) (domain.EngineResult, error) {
	return domain.EngineResult{}, errors.New("FaceProof Secure Core is unavailable")
}

func (client *NativeClient) ExtractReference(
	ctx context.Context,
	imagePayload string,
) (domain.ReferenceResult, error) {
	return domain.ReferenceResult{}, errors.New("FaceProof Secure Core is unavailable")
}

func (client *NativeClient) Guide(
	ctx context.Context,
	imagePayload string,
) (domain.EngineGuideResult, error) {
	return domain.EngineGuideResult{}, errors.New("FaceProof Secure Core is unavailable")
}
