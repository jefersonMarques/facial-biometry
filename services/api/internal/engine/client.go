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
	body, err := json.Marshal(request)
	if err != nil {
		return domain.EngineResult{}, err
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+"/analyze", bytes.NewReader(body))
	if err != nil {
		return domain.EngineResult{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return domain.EngineResult{}, err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return domain.EngineResult{}, err
	}
	if response.StatusCode != http.StatusOK {
		return domain.EngineResult{}, fmt.Errorf("engine returned %d: %s", response.StatusCode, string(responseBody))
	}

	var result domain.EngineResult
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return domain.EngineResult{}, err
	}
	if len(result.Embedding) == 0 {
		return domain.EngineResult{}, errors.New("engine returned empty embedding")
	}
	return result, nil
}
