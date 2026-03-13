package team

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/theapemachine/piaf/provider"
)

/*
SubAgentRunner runs a sub-agent with the given provider and returns the result.
*/
type SubAgentRunner struct {
	timeout time.Duration
}

/*
NewSubAgentRunner instantiates a new SubAgentRunner.
*/
func NewSubAgentRunner(timeout time.Duration) *SubAgentRunner {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &SubAgentRunner{timeout: timeout}
}

/*
Run executes a sub-agent with the given provider, system prompt, and user prompt.
Returns the sub-agent response or an error.
*/
func (runner *SubAgentRunner) Run(
	ctx context.Context,
	prov provider.Provider,
	systemPrompt string,
	userPrompt string,
) (string, error) {
	systemPrompt = strings.TrimSpace(systemPrompt)
	userPrompt = strings.TrimSpace(userPrompt)
	if systemPrompt == "" {
		systemPrompt = "You are a focused sub-agent. Complete the task concisely."
	}
	if userPrompt == "" {
		return "", fmt.Errorf("subagent: user prompt is required")
	}

	request := &provider.Request{
		Mode:         "IMPLEMENT",
		Message:      userPrompt,
		SystemPrompt: systemPrompt,
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, runner.timeout)
		defer cancel()
	}

	return prov.Generate(ctx, request)
}
