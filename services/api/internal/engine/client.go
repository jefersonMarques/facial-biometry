package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"faceproof/services/api/internal/domain"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
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

func (client *Client) ExtractReference(ctx context.Context, imageBase64 string) (domain.ReferenceResult, error) {
	var result domain.ReferenceResult
	if err := client.postJSON(ctx, "/reference", map[string]string{"imageBase64": imageBase64}, &result); err != nil {
		return domain.ReferenceResult{}, err
	}
	if len(result.Embedding) == 0 {
		return domain.ReferenceResult{}, errors.New("engine returned empty reference embedding")
	}
	return result, nil
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
