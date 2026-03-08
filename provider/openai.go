package provider

import (
	"bufio"
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
		return "", fmt.Errorf("%s: %s", provider.Name(), parseAPIError(responseBody))
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

/*
GenerateStream performs a streaming chat completion request.
*/
func (provider *OpenAIProvider) GenerateStream(ctx context.Context, request *Request, onChunk func(string)) (response string, err error) {
	if provider.apiKey == "" {
		return "", fmt.Errorf("%s is not configured: missing OPENAI_API_KEY", provider.Name())
	}

	payload := map[string]any{
		"stream": true,
		"model":  provider.model,
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

	if httpResponse.StatusCode >= http.StatusBadRequest {
		responseBody, _ := io.ReadAll(httpResponse.Body)
		return "", fmt.Errorf("%s: %s", provider.Name(), parseAPIError(responseBody))
	}

	var full strings.Builder
	scanner := bufio.NewScanner(httpResponse.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}

		var decoded struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
			continue
		}

		if len(decoded.Choices) > 0 && decoded.Choices[0].Delta.Content != "" {
			chunk := decoded.Choices[0].Delta.Content
			full.WriteString(chunk)
			if onChunk != nil {
				onChunk(chunk)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return full.String(), err
	}

	return strings.TrimSpace(full.String()), nil
}
