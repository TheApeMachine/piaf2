package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

/*
parseAPIError extracts a single-line message from API error JSON.
Handles OpenAI, Gemini, and Anthropic shapes. Falls back to truncated raw on parse failure.
*/
func parseAPIError(body []byte) string {
	raw := strings.TrimSpace(string(body))
	raw = strings.ReplaceAll(raw, "\n", " ")
	raw = strings.ReplaceAll(raw, "\r", "")

	var parsed struct {
		Error struct {
			Message string `json:"message"`
			Code    any    `json:"code"`
			Type    string `json:"type"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &parsed); err != nil {
		if len(raw) > 100 {
			return raw[:97] + "..."
		}

		return raw
	}

	message := strings.TrimSpace(parsed.Error.Message)
	if message == "" {
		if len(raw) > 100 {
			return raw[:97] + "..."
		}

		return raw
	}

	code := ""
	switch value := parsed.Error.Code.(type) {
	case float64:
		if value != 0 {
			code = fmt.Sprintf("%.0f", value)
		}
	case string:
		if value != "" {
			code = value
		}
	}

	if code != "" {
		return code + ": " + message
	}

	return message
}

/*
ToolDefinition describes a function tool for the model to call.
*/
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any
}

/*
ToolCallOutput is the result of executing a tool call, sent when continuing a conversation.
*/
type ToolCallOutput struct {
	CallID string
	Output string
}

/*
Request carries the running discussion context into a provider call.
SystemPrompt overrides the default when non-empty.
Tools and ToolExecutor enable function calling; when ToolExecutor is set, the provider
loops until the model returns text (no more tool calls).
*/
type Request struct {
	Mode              string
	Message           string
	ToolOutput        string
	Transcript        []string
	PriorResponse     []string
	SystemPrompt      string
	Tools             []ToolDefinition
	ToolExecutor      func(name string, args map[string]any) (string, error)
	PreviousResponseID string
	ToolCallOutputs   []ToolCallOutput
}

/*
Provider generates a response for a single model stage.
Generate streams chunks via onChunk and returns the full response.
*/
type Provider interface {
	Name() string
	Generate(ctx context.Context, request *Request) (response string, err error)
	GenerateStream(ctx context.Context, request *Request, onChunk func(string)) (response string, err error)
}

/*
BuildSystemPrompt returns the system prompt from the request.
The prompt is supplied by the caller (config-driven); the provider does not override it.
*/
func BuildSystemPrompt(request *Request) string {
	return request.SystemPrompt
}

/*
BuildUserPrompt creates the shared provider user prompt.
*/
func BuildUserPrompt(request *Request) string {
	lines := []string{
		"Mode: " + request.Mode,
		"User message:",
		request.Message,
	}

	if len(request.Transcript) > 0 {
		lines = append(lines, "", "Running transcript:")
		lines = append(lines, request.Transcript...)
	}

	if request.ToolOutput != "" {
		lines = append(lines, "", "Workspace tool output:")
		lines = append(lines, request.ToolOutput)
	}

	if len(request.PriorResponse) > 0 {
		lines = append(lines, "", "Responses already produced in this stage:")
		lines = append(lines, request.PriorResponse...)
	}

	if request.Mode == "IMPLEMENT" {
		lines = append(lines, "", "Return implementation-focused guidance. The last stage should mention accept or reject.")
	}

	if request.Mode == "DISCUSS" && len(request.Transcript) > 1 {
		lines = append(lines, "", "Your turn. Reply directly to what others have said. Do not repeat points already in the transcript; add something new, challenge a specific claim, or synthesize toward a conclusion.")
	}

	return strings.Join(lines, "\n")
}

/*
retryProvider wraps a Provider with exponential backoff on transient errors.
*/
type retryProvider struct {
	base     Provider
	attempts int
}

/*
WithRetry wraps a Provider to retry failed requests with exponential backoff.
*/
func WithRetry(base Provider, attempts int) Provider {
	return &retryProvider{
		base:     base,
		attempts: attempts,
	}
}

/*
Name delegates to the wrapped provider.
*/
func (r *retryProvider) Name() string { return r.base.Name() }

/*
Generate retries the base provider up to attempts times with exponential backoff.
*/
func (r *retryProvider) Generate(ctx context.Context, request *Request) (response string, err error) {
	for i := 0; i < r.attempts; i++ {
		response, err = r.base.Generate(ctx, request)
		if err == nil {
			return response, nil
		}

		if i < r.attempts-1 {
			backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
	}
	return response, err
}

/*
GenerateStream retries the base provider's stream up to attempts times with exponential backoff.
*/
func (r *retryProvider) GenerateStream(ctx context.Context, request *Request, onChunk func(string)) (response string, err error) {
	for i := 0; i < r.attempts; i++ {
		response, err = r.base.GenerateStream(ctx, request, onChunk)
		if err == nil {
			return response, nil
		}

		if i < r.attempts-1 {
			backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
	}
	return response, err
}
