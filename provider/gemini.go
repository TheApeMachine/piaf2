package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"
)

/*
GeminiProvider calls the Gemini API via the official genai SDK.
Supports GenerateContent and GenerateContentStream.
*/
type GeminiProvider struct {
	client *genai.Client
	model  string
}

type geminiOpts func(*GeminiProvider)

/*
NewGeminiProvider instantiates a GeminiProvider from config and environment.
*/
func NewGeminiProvider(opts ...geminiOpts) *GeminiProvider {
	provider := &GeminiProvider{
		model: "gemini-2.5-flash",
	}

	if model := os.Getenv("GEMINI_MODEL"); model != "" {
		provider.model = model
	}

	for _, opt := range opts {
		opt(provider)
	}

	if provider.client == nil {
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("GOOGLE_API_KEY")
		}

		if apiKey == "" {
			return provider
		}

		config := &genai.ClientConfig{
			APIKey:  apiKey,
			Backend: genai.BackendGeminiAPI,
		}

		client, err := genai.NewClient(context.Background(), config)
		if err != nil {
			return provider
		}

		provider.client = client
	}

	return provider
}

/*
Name returns the provider display name.
*/
func (provider *GeminiProvider) Name() string {
	return "Gemini Pro 3.1"
}

/*
Generate performs a non-streaming GenerateContent request.
*/
func (provider *GeminiProvider) Generate(ctx context.Context, request *Request) (response string, err error) {
	if provider.client == nil {
		return "", fmt.Errorf("%s is not configured: missing GEMINI_API_KEY or GOOGLE_API_KEY", provider.Name())
	}

	system := BuildSystemPrompt(request)
	user := BuildUserPrompt(request)

	config := &genai.GenerateContentConfig{
		Temperature: genai.Ptr[float32](0),
	}

	if system != "" {
		config.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{{Text: system}},
		}
	}

	result, err := provider.client.Models.GenerateContent(ctx, provider.model, genai.Text(user), config)
	if err != nil {
		return "", fmt.Errorf("%s: %w", provider.Name(), err)
	}

	text := strings.TrimSpace(result.Text())
	if text == "" {
		return "", fmt.Errorf("%s returned no content", provider.Name())
	}

	return text, nil
}

/*
GenerateStream performs a streaming GenerateContent request.
*/
func (provider *GeminiProvider) GenerateStream(
	ctx context.Context,
	request *Request,
	onChunk func(string),
) (response string, err error) {
	if provider.client == nil {
		return "", fmt.Errorf("%s is not configured: missing GEMINI_API_KEY or GOOGLE_API_KEY", provider.Name())
	}

	system := BuildSystemPrompt(request)
	user := BuildUserPrompt(request)

	config := &genai.GenerateContentConfig{
		Temperature: genai.Ptr[float32](0),
	}

	if system != "" {
		config.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{{Text: system}},
		}
	}

	var full strings.Builder

	for result, streamErr := range provider.client.Models.GenerateContentStream(ctx, provider.model, genai.Text(user), config) {
		if streamErr != nil {
			return full.String(), fmt.Errorf("%s: %w", provider.Name(), streamErr)
		}

		chunk := result.Text()
		if chunk != "" {
			full.WriteString(chunk)
			if onChunk != nil {
				onChunk(chunk)
			}
		}
	}

	text := strings.TrimSpace(full.String())
	if text == "" {
		return "", fmt.Errorf("%s returned no content", provider.Name())
	}

	return text, nil
}

/*
GeminiWithClient configures GeminiProvider with a genai client.
*/
func GeminiWithClient(client *genai.Client) geminiOpts {
	return func(provider *GeminiProvider) {
		provider.client = client
	}
}

/*
GeminiWithModel configures GeminiProvider with a model name.
*/
func GeminiWithModel(model string) geminiOpts {
	return func(provider *GeminiProvider) {
		provider.model = model
	}
}

/*
GeminiWithBaseURL configures GeminiProvider with a custom base URL for testing.
*/
func GeminiWithBaseURL(baseURL string) geminiOpts {
	return func(provider *GeminiProvider) {
		if provider.client != nil {
			return
		}

		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("GOOGLE_API_KEY")
		}
		if apiKey == "" {
			apiKey = "test-key"
		}

		config := &genai.ClientConfig{
			APIKey:      apiKey,
			Backend:     genai.BackendGeminiAPI,
			HTTPOptions: genai.HTTPOptions{BaseURL: baseURL},
		}

		client, err := genai.NewClient(context.Background(), config)
		if err == nil {
			provider.client = client
		}
	}
}

