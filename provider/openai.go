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
OpenAIProvider calls the OpenAI chat completions API.
*/
type OpenAIProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

/*
NewOpenAIProvider instantiates an OpenAIProvider from the environment.
*/
func NewOpenAIProvider() *OpenAIProvider {
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-5.4"
	}

	return &OpenAIProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  os.Getenv("OPENAI_API_KEY"),
		model:   model,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

/*
Name returns the provider display name.
*/
func (provider *OpenAIProvider) Name() string {
	return "OpenAI GPT-5.4"
}

/*
Generate performs a chat completion request.
*/
func (provider *OpenAIProvider) Generate(ctx context.Context, request *Request) (response string, err error) {
	if provider.apiKey == "" {
		return "", fmt.Errorf("%s is not configured: missing OPENAI_API_KEY", provider.Name())
	}

	payload := map[string]any{
		"model": provider.model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": BuildSystemPrompt(request),
			},
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

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	httpRequest.Header.Set("Authorization", "Bearer "+provider.apiKey)
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
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err = json.Unmarshal(responseBody, &decoded); err != nil {
		return "", err
	}

	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("%s returned no choices", provider.Name())
	}

	return strings.TrimSpace(decoded.Choices[0].Message.Content), nil
}
