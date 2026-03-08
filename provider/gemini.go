package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

/*
GeminiProvider calls the Gemini generateContent API.
*/
type GeminiProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

/*
NewGeminiProvider instantiates a GeminiProvider from the environment.
*/
func NewGeminiProvider() *GeminiProvider {
	baseURL := os.Getenv("GEMINI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}

	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-3.1-pro-preview"
	}

	return &GeminiProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  os.Getenv("GEMINI_API_KEY"),
		model:   model,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

/*
Name returns the provider display name.
*/
func (provider *GeminiProvider) Name() string {
	return "Gemini Pro 3.1"
}

/*
Generate performs a Gemini generateContent request.
*/
func (provider *GeminiProvider) Generate(ctx context.Context, request *Request) (response string, err error) {
	if provider.apiKey == "" {
		return "", fmt.Errorf("%s is not configured: missing GEMINI_API_KEY", provider.Name())
	}

	payload := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []map[string]string{
				{
					"text": BuildSystemPrompt(request),
				},
			},
		},
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]string{
					{
						"text": BuildUserPrompt(request),
					},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	endpoint := provider.baseURL + "/models/" + provider.model + ":generateContent?key=" + url.QueryEscape(provider.apiKey)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

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
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err = json.Unmarshal(responseBody, &decoded); err != nil {
		return "", err
	}

	if len(decoded.Candidates) == 0 {
		return "", fmt.Errorf("%s returned no candidates", provider.Name())
	}

	parts := make([]string, 0, len(decoded.Candidates[0].Content.Parts))
	for _, part := range decoded.Candidates[0].Content.Parts {
		if part.Text != "" {
			parts = append(parts, strings.TrimSpace(part.Text))
		}
	}

	if len(parts) == 0 {
		return "", fmt.Errorf("%s returned no content", provider.Name())
	}

	return strings.Join(parts, "\n"), nil
}

/*
GenerateStream performs a streaming Gemini generateContent request.
*/
func (provider *GeminiProvider) GenerateStream(ctx context.Context, request *Request, onChunk func(string)) (response string, err error) {
	if provider.apiKey == "" {
		return "", fmt.Errorf("%s is not configured: missing GEMINI_API_KEY", provider.Name())
	}

	payload := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []map[string]string{
				{
					"text": BuildSystemPrompt(request),
				},
			},
		},
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]string{
					{
						"text": BuildUserPrompt(request),
					},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	endpoint := provider.baseURL + "/models/" + provider.model + ":streamGenerateContent?key=" + url.QueryEscape(provider.apiKey)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

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
		if line == "" {
			continue
		}

		var decoded struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}

		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			continue
		}

		if len(decoded.Candidates) > 0 {
			for _, part := range decoded.Candidates[0].Content.Parts {
				if part.Text != "" {
					chunk := part.Text
					full.WriteString(chunk)
					if onChunk != nil {
						onChunk(chunk)
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return full.String(), err
	}

	return strings.TrimSpace(full.String()), nil
}
