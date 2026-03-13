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
When Tools and ToolExecutor are set, loops until the model returns text.
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
	if len(request.Tools) > 0 {
		config.Tools = []*genai.Tool{{FunctionDeclarations: provider.buildFunctionDeclarations(request.Tools)}}
		config.ToolConfig = &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingConfigModeAny,
			},
		}
	}

	contents := genai.Text(user)
	for {
		result, err := provider.client.Models.GenerateContent(ctx, provider.model, contents, config)
		if err != nil {
			return "", fmt.Errorf("%s: %w", provider.Name(), err)
		}

		text := strings.TrimSpace(geminiExtractText(result))
		if request.ToolExecutor == nil {
			if text == "" {
				return "", fmt.Errorf("%s returned no content", provider.Name())
			}
			return text, nil
		}

		calls := result.FunctionCalls()
		if len(calls) == 0 {
			if text == "" {
				return "(no text produced)", nil
			}
			return text, nil
		}

		if len(result.Candidates) > 0 && result.Candidates[0].Content != nil {
			contents = append(contents, result.Candidates[0].Content)
		}

		responseParts := make([]*genai.Part, 0, len(calls))
		for _, call := range calls {
			output, execErr := request.ToolExecutor(call.Name, call.Args)
			resp := map[string]any{"output": output}
			if execErr != nil {
				resp = map[string]any{"error": execErr.Error()}
			}
			responseParts = append(responseParts, genai.NewPartFromFunctionResponse(call.Name, resp))
		}
		contents = append(contents, genai.NewContentFromParts(responseParts, genai.RoleUser))
	}
}

/*
geminiExtractText returns concatenated text from response parts.
Avoids calling result.Text() which logs a warning when the response contains FunctionCall parts.
*/
func geminiExtractText(resp *genai.GenerateContentResponse) string {
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return ""
	}

	var parts []string
	for _, part := range resp.Candidates[0].Content.Parts {
		if part != nil && part.Text != "" {
			parts = append(parts, part.Text)
		}
	}
	return strings.Join(parts, "")
}

func (provider *GeminiProvider) buildFunctionDeclarations(tools []ToolDefinition) []*genai.FunctionDeclaration {
	decls := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		decl := &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
		}
		if tool.Parameters != nil {
			decl.ParametersJsonSchema = tool.Parameters
		}
		decls = append(decls, decl)
	}
	return decls
}

/*
GenerateStream performs a streaming GenerateContent request.
When Tools and ToolExecutor are set, handles tool calls by continuing after each stream completes.
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
	if len(request.Tools) > 0 {
		config.Tools = []*genai.Tool{{FunctionDeclarations: provider.buildFunctionDeclarations(request.Tools)}}
		config.ToolConfig = &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingConfigModeAny,
			},
		}
	}

	contents := genai.Text(user)
	for {
		var full strings.Builder
		var lastResult *genai.GenerateContentResponse

		for result, streamErr := range provider.client.Models.GenerateContentStream(ctx, provider.model, contents, config) {
			if streamErr != nil {
				return full.String(), fmt.Errorf("%s: %w", provider.Name(), streamErr)
			}
			lastResult = result
			chunk := geminiExtractText(result)
			if chunk != "" {
				full.WriteString(chunk)
				if onChunk != nil {
					onChunk(chunk)
				}
			}
		}

		text := strings.TrimSpace(full.String())
		if request.ToolExecutor == nil {
			if text == "" {
				return "", fmt.Errorf("%s returned no content", provider.Name())
			}
			return text, nil
		}

		if lastResult == nil {
			if text == "" {
				return "", fmt.Errorf("%s returned no content", provider.Name())
			}
			return text, nil
		}

		calls := lastResult.FunctionCalls()
		if len(calls) == 0 {
			if text == "" {
				return "(no text produced)", nil
			}
			return text, nil
		}

		if len(lastResult.Candidates) > 0 && lastResult.Candidates[0].Content != nil {
			contents = append(contents, lastResult.Candidates[0].Content)
		}

		responseParts := make([]*genai.Part, 0, len(calls))
		for _, call := range calls {
			if onChunk != nil {
				onChunk(fmt.Sprintf("\n[Tool call: %s]\n", call.Name))
			}
			output, execErr := request.ToolExecutor(call.Name, call.Args)
			resp := map[string]any{"output": output}
			if execErr != nil {
				resp = map[string]any{"error": execErr.Error()}
			}
			responseParts = append(responseParts, genai.NewPartFromFunctionResponse(call.Name, resp))
		}
		contents = append(contents, genai.NewContentFromParts(responseParts, genai.RoleUser))
	}
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

