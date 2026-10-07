package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"faceproof/services/api/internal/domain"
)

type Client struct {
	baseURL        string
	httpClient     *http.Client
	nativeIdentity *NativeClient
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 90 * time.Second},
	}
}

func (client *Client) EnableNativeIdentity(configuration NativeConfig) error {
	nativeClient, err := NewNativeClient(configuration)
	if err != nil {
		return err
	}
	if client.nativeIdentity != nil {
		_ = client.nativeIdentity.Close()
	}
	client.nativeIdentity = nativeClient
	return nil
}

func (client *Client) NativeIdentityEnabled() bool {
	return client != nil && client.nativeIdentity != nil
}

func (client *Client) Close() error {
	if client == nil || client.nativeIdentity == nil {
		return nil
	}
	return client.nativeIdentity.Close()
}

func (client *Client) AnalyzeIdentity(ctx context.Context, request domain.EngineIdentityRequest) (domain.EngineResult, error) {
	if client.nativeIdentity != nil {
		return client.nativeIdentity.AnalyzeIdentity(ctx, request)
	}

	var result domain.EngineResult

	hasBinary := false
	for _, frame := range request.GuidedFrames {
		if len(frame.ImageBytes) > 0 {
			hasBinary = true
			break
		}
	}

	var err error
	if hasBinary {
		err = client.postIdentityMultipart(ctx, request, &result)
	} else {
		err = client.postJSON(ctx, "/identity-analyze", request, &result)
	}
	if err != nil {
		return domain.EngineResult{}, err
	}
	if len(result.Embedding) == 0 {
		return domain.EngineResult{}, errors.New("engine returned empty identity embedding")
	}
	return result, nil
}

func (client *Client) Analyze(ctx context.Context, request domain.EngineRequest) (domain.EngineResult, error) {
	var result domain.EngineResult
	if err := client.postJSON(ctx, "/analyze", request, &result); err != nil {
		return domain.EngineResult{}, err
	}
	if len(result.Embedding) == 0 {
		return domain.EngineResult{}, errors.New("engine returned empty embedding")
	}
	return result, nil
}

func (client *Client) Guide(ctx context.Context, imageBase64 string) (domain.EngineGuideResult, error) {
	if client.nativeIdentity != nil {
		return client.nativeIdentity.Guide(ctx, imageBase64)
	}

	var result domain.EngineGuideResult
	if err := client.postJSON(ctx, "/guide", map[string]string{"imageBase64": imageBase64}, &result); err != nil {
		return domain.EngineGuideResult{}, err
	}
	return result, nil
}

func (client *Client) ExtractReference(ctx context.Context, imageBase64 string) (domain.ReferenceResult, error) {
	if client.nativeIdentity != nil {
		return client.nativeIdentity.ExtractReference(ctx, imageBase64)
	}

	var result domain.ReferenceResult
	if err := client.postJSON(ctx, "/reference", map[string]string{"imageBase64": imageBase64}, &result); err != nil {
		return domain.ReferenceResult{}, err
	}
	if len(result.Embedding) == 0 {
		return domain.ReferenceResult{}, errors.New("engine returned empty reference embedding")
	}
	return result, nil
}

func (client *Client) postIdentityMultipart(
	ctx context.Context,
	request domain.EngineIdentityRequest,
	target *domain.EngineResult,
) error {
	if len(request.GuidedFrames) == 0 {
		return errors.New("identity capture has no guided frames")
	}

	type manifestFrame struct {
		Phase         string                     `json:"phase"`
		ClientQuality *domain.ClientFrameQuality `json:"clientQuality,omitempty"`
	}
	type manifestPayload struct {
		GuidedFrames []manifestFrame `json:"guidedFrames"`
	}

	manifest := manifestPayload{
		GuidedFrames: make([]manifestFrame, len(request.GuidedFrames)),
	}
	for index, frame := range request.GuidedFrames {
		if len(frame.ImageBytes) == 0 {
			return errors.New("identity capture mixes binary and base64 frames")
		}
		manifest.GuidedFrames[index] = manifestFrame{
			Phase:         frame.Phase,
			ClientQuality: frame.ClientQuality,
		}
	}

	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("manifest", string(manifestJSON)); err != nil {
		return err
	}
	for index, frame := range request.GuidedFrames {
		header := make(textproto.MIMEHeader)
		header.Set(
			"Content-Disposition",
			fmt.Sprintf(`form-data; name="frame"; filename="%02d-%s.jpg"`, index, frame.Phase),
		)
		header.Set("Content-Type", "image/jpeg")
		part, err := writer.CreatePart(header)
		if err != nil {
			return err
		}
		if _, err := part.Write(frame.ImageBytes); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}

	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		client.baseURL+"/identity-analyze",
		bytes.NewReader(body.Bytes()),
	)
	if err != nil {
		return err
	}
	httpRequest.Header.Set("Content-Type", writer.FormDataContentType())

	return client.doJSONRequest(httpRequest, target)
}

func (client *Client) postJSON(ctx context.Context, path string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	return client.doJSONRequest(httpRequest, target)
}

func (client *Client) doJSONRequest(httpRequest *http.Request, target any) error {
	response, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("engine returned %d", response.StatusCode)
	}
	if err := json.Unmarshal(responseBody, target); err != nil {
		return err
	}
	return nil
}
