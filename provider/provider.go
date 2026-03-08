package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
Request carries the running discussion context into a provider call.
SystemPrompt overrides the default when non-empty.
*/
type Request struct {
	Mode          string
	Message       string
	ToolOutput    string
	Transcript    []string
	PriorResponse []string
	SystemPrompt  string
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
BuildSystemPrompt creates the shared provider system prompt.
Uses Request.SystemPrompt when set, otherwise returns the default for the mode.
*/
func BuildSystemPrompt(request *Request) string {
	if request.SystemPrompt != "" {
		return request.SystemPrompt
	}

	if request.Mode == "IMPLEMENT" {
		return "You are part of a three-model development team. Keep the implementation plan concrete, minimal, and reviewable. The final agent must leave the user with a decision they can accept or reject."
	}

	return "You are part of a three-model discussion chain. Build on the prior context, keep the answer concise, and use any provided workspace tool output as evidence."
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

	return strings.Join(lines, "\n")
}
