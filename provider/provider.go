package provider

import (
	"context"
	"strings"
)

/*
Request carries the running discussion context into a provider call.
*/
type Request struct {
	Mode          string
	Message       string
	ToolOutput    string
	Transcript    []string
	PriorResponse []string
}

/*
Provider generates a response for a single model stage.
*/
type Provider interface {
	Name() string
	Generate(ctx context.Context, request *Request) (response string, err error)
}

/*
BuildSystemPrompt creates the shared provider system prompt.
*/
func BuildSystemPrompt(request *Request) string {
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
