package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

/*
ClaudeProvider calls the Anthropic messages API.
*/
type ClaudeProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

/*
NewClaudeProvider instantiates a ClaudeProvider from the environment.
*/
func NewClaudeProvider() *ClaudeProvider {
	baseURL := os.Getenv("CLAUDE_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}

	model := os.Getenv("CLAUDE_MODEL")
	if model == "" {
		model = "claude-open-4.6"
	}

	return &ClaudeProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  os.Getenv("CLAUDE_API_KEY"),
		model:   model,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

/*
Name returns the provider display name.
*/
func (provider *ClaudeProvider) Name() string {
	return "Claude Open 4.6"
}

/*
Generate performs an Anthropic messages request.
*/
func (provider *ClaudeProvider) Generate(ctx context.Context, request *Request) (response string, err error) {
	if provider.apiKey == "" {
		return "", fmt.Errorf("%s is not configured: missing CLAUDE_API_KEY", provider.Name())
	}

	payload := map[string]any{
		"model":      provider.model,
		"max_tokens": 2048,
		"system":     BuildSystemPrompt(request),
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": BuildUserPrompt(request),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	httpRequest.Header.Set("x-api-key", provider.apiKey)
	httpRequest.Header.Set("anthropic-version", "2023-06-01")
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := provider.client.Do(httpRequest)
	if err != nil {
		return "", err
	}
	defer httpResponse.Body.Close()

	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return "", err
	}

	if httpResponse.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("%s request failed: %s", provider.Name(), strings.TrimSpace(string(responseBody)))
	}

	var decoded struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err = json.Unmarshal(responseBody, &decoded); err != nil {
		return "", err
	}

	parts := make([]string, 0, len(decoded.Content))
	for _, part := range decoded.Content {
		if part.Text != "" {
			parts = append(parts, strings.TrimSpace(part.Text))
		}
	}

	if len(parts) == 0 {
		return "", fmt.Errorf("%s returned no content", provider.Name())
	}

	return strings.Join(parts, "\n"), nil
}
