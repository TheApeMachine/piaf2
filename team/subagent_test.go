package team

import (
	"context"
	"testing"
	"time"

	"github.com/theapemachine/piaf/provider"
)

type stubSubAgentProvider struct {
	response string
}

func (s *stubSubAgentProvider) Name() string {
	return "stub"
}

func (s *stubSubAgentProvider) Generate(_ context.Context, _ *provider.Request) (string, error) {
	return s.response, nil
}

func (s *stubSubAgentProvider) GenerateStream(_ context.Context, _ *provider.Request, onChunk func(string)) (string, error) {
	if onChunk != nil {
		onChunk(s.response)
	}
	return s.response, nil
}

func TestSubAgentRunnerRun(t *testing.T) {
	runner := NewSubAgentRunner(5 * time.Second)
	stub := &stubSubAgentProvider{response: "sub-agent result"}

	result, err := runner.Run(context.Background(), stub, "System", "User task")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result != "sub-agent result" {
		t.Errorf("got %q", result)
	}
}

func BenchmarkSubAgentRunnerRun(b *testing.B) {
	runner := NewSubAgentRunner(5 * time.Second)
	stub := &stubSubAgentProvider{response: "ok"}

	for index := 0; index < b.N; index++ {
		_, _ = runner.Run(context.Background(), stub, "sys", "task")
	}
}
