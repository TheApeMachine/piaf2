package editor

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/piaf/provider"
)

type stubProvider struct {
	name      string
	responses []string
	requests  []*provider.Request
	generate  func(*provider.Request, int) string
}

func (stub *stubProvider) Name() string {
	return stub.name
}

func (stub *stubProvider) Generate(_ context.Context, request *provider.Request) (response string, err error) {
	copyRequest := &provider.Request{
		Mode:          request.Mode,
		Message:       request.Message,
		ToolOutput:    request.ToolOutput,
		Transcript:    append([]string(nil), request.Transcript...),
		PriorResponse: append([]string(nil), request.PriorResponse...),
		SystemPrompt:  request.SystemPrompt,
	}

	stub.requests = append(stub.requests, copyRequest)

	index := len(stub.requests) - 1
	if stub.generate != nil {
		return stub.generate(copyRequest, index), nil
	}

	if index < len(stub.responses) {
		return stub.responses[index], nil
	}

	return fmt.Sprintf("%s response %d", stub.name, index+1), nil
}

func (stub *stubProvider) GenerateStream(ctx context.Context, request *provider.Request, onChunk func(string)) (response string, err error) {
	response, err = stub.Generate(ctx, request)
	if err != nil {
		return "", err
	}

	if onChunk != nil {
		onChunk(response)
	}

	return response, nil
}

func TestChatSubmit(t *testing.T) {
	convey.Convey("Given a Chat scoped to a workspace", t, func() {
		root := t.TempDir()
		os.WriteFile(filepath.Join(root, "note.txt"), []byte("hello\nworld"), 0o644)
		os.Mkdir(filepath.Join(root, "docs"), 0o755)

		openai := &stubProvider{name: "OpenAI GPT-5.4", responses: []string{"first response"}}
		claude := &stubProvider{name: "Claude Open 4.6", responses: []string{"second response"}}
		gemini := &stubProvider{name: "Gemini Pro 3.1", responses: []string{"third response"}}

		chat := NewChat(
			ChatWithRoot(root),
			ChatWithRandom(rand.New(rand.NewSource(7))),
			ChatWithSystemPrompt("Keep discussion concise."),
			ChatWithProviders(openai, claude, gemini),
		)

		convey.Convey("When a browse prompt is submitted", func() {
			chat.Submit("browse .")

			convey.Convey("It should add a full three-model pipeline to the transcript", func() {
				lines := chat.Lines()
				transcript := strings.Join(lines, "\n")

				convey.So(lines, convey.ShouldHaveLength, 10)
				convey.So(transcript, convey.ShouldContainSubstring, "Pipeline:")
				convey.So(transcript, convey.ShouldContainSubstring, "OpenAI GPT-5.4")
				convey.So(transcript, convey.ShouldContainSubstring, "Claude Open 4.6")
				convey.So(transcript, convey.ShouldContainSubstring, "Gemini Pro 3.1")
				convey.So(len(openai.requests), convey.ShouldEqual, 1)
				convey.So(len(claude.requests), convey.ShouldEqual, 1)
				convey.So(len(gemini.requests), convey.ShouldEqual, 1)

				recorded := []*provider.Request{
					openai.requests[0],
					claude.requests[0],
					gemini.requests[0],
				}
				priorCounts := []int{
					len(recorded[0].PriorResponse),
					len(recorded[1].PriorResponse),
					len(recorded[2].PriorResponse),
				}
				sort.Ints(priorCounts)

				convey.So(recorded[0].ToolOutput, convey.ShouldContainSubstring, "Tool browse .")
				convey.So(recorded[1].ToolOutput, convey.ShouldContainSubstring, "Tool browse .")
				convey.So(recorded[2].ToolOutput, convey.ShouldContainSubstring, "Tool browse .")
				convey.So(recorded[0].SystemPrompt, convey.ShouldEqual, "Keep discussion concise.")
				convey.So(recorded[0].ToolOutput, convey.ShouldContainSubstring, "- docs/")
				convey.So(recorded[0].ToolOutput, convey.ShouldContainSubstring, "- note.txt")
				convey.So(priorCounts, convey.ShouldResemble, []int{0, 1, 2})
			})
		})

		convey.Convey("When a read prompt targets a file outside the workspace", func() {
			outside := filepath.Join(filepath.Dir(root), "outside.txt")
			os.WriteFile(outside, []byte("secret"), 0o644)

			chat.Submit("read ../outside.txt")

			convey.Convey("It should block the tool access", func() {
				convey.So(len(openai.requests), convey.ShouldEqual, 1)
				convey.So(len(claude.requests), convey.ShouldEqual, 1)
				convey.So(len(gemini.requests), convey.ShouldEqual, 1)
				convey.So(openai.requests[0].ToolOutput, convey.ShouldContainSubstring, "Tool read blocked")
				convey.So(claude.requests[0].ToolOutput, convey.ShouldContainSubstring, "Tool read blocked")
				convey.So(gemini.requests[0].ToolOutput, convey.ShouldContainSubstring, "Tool read blocked")
			})
		})
	})
}

func TestChatImplementMode(t *testing.T) {
	convey.Convey("Given a Chat in implementation mode", t, func() {
		chat := NewChat(
			ChatWithRandom(rand.New(rand.NewSource(11))),
			ChatWithProviders(
				&stubProvider{name: "OpenAI GPT-5.4", responses: []string{"scoped the request"}},
				&stubProvider{name: "Claude Open 4.6", responses: []string{"prepared the diff"}},
				&stubProvider{name: "Gemini Pro 3.1", responses: []string{"final implementation summary"}},
			),
		)
		chat.SetMode("IMPLEMENT")

		convey.Convey("When an implementation prompt is submitted", func() {
			chat.Submit("add a command palette")

			convey.Convey("It should end with accept and reject guidance", func() {
				transcript := strings.Join(chat.Lines(), "\n")
				convey.So(transcript, convey.ShouldContainSubstring, "Accept with :accept or :reject.")
			})
		})
	})
}

func TestChatMemoryTools(t *testing.T) {
	convey.Convey("Given a Chat with memory-aware agents", t, func() {
		openai := &stubProvider{name: "OpenAI GPT-5.4", responses: []string{"remembered", "recalled"}}
		claude := &stubProvider{name: "Claude Open 4.6", responses: []string{"tracked", "confirmed"}}
		gemini := &stubProvider{name: "Gemini Pro 3.1", responses: []string{"stored", "shared"}}

		chat := NewChat(
			ChatWithRandom(rand.New(rand.NewSource(17))),
			ChatWithProviders(openai, claude, gemini),
		)

		convey.Convey("When a memory entry is stored and recalled", func() {
			chat.Submit("remember keep tests focused")
			chat.Submit("recall focused")

			convey.Convey("It should expose memory management to the agent pipeline", func() {
				convey.So(openai.requests[1].ToolOutput, convey.ShouldContainSubstring, "Memory recall:")
				convey.So(openai.requests[1].ToolOutput, convey.ShouldContainSubstring, "Shared: keep tests focused")
			})
		})
	})
}

func TestChatImplementWorkflow(t *testing.T) {
	convey.Convey("Given a Chat in implementation mode with a QA rework pass", t, func() {
		qaReviews := 0
		generate := func(request *provider.Request, _ int) string {
			switch {
			case strings.Contains(request.SystemPrompt, "Summarize the completed work"):
				return "Summary: Epics completed. Key changes applied. Tests added. Accept with :accept or :reject."
			case strings.Contains(request.SystemPrompt, "Project Manager"):
				return "Project board captured with scope and risks."
			case strings.Contains(request.SystemPrompt, "Architect"):
				return "Implementation plan: editor/editor.go, editor/chat.go; order: editor first; risks: minimal."
			case strings.Contains(request.SystemPrompt, "Team Lead"):
				return "Team staffed and assignments published."
			case strings.Contains(request.SystemPrompt, "Developer"):
				return "Developer implementation progress shared."
			case strings.Contains(request.SystemPrompt, "QA"):
				qaReviews++
				if qaReviews == 1 {
					return "Decision: REWORK\nUnit coverage is incomplete."
				}

				return "Decision: PASS\nCoverage now looks good."
			default:
				return "Review summary ready. Accept with :accept or :reject."
			}
		}

		projectManager := &stubProvider{name: "OpenAI GPT-5.4", generate: generate}
		teamLead := &stubProvider{name: "Claude Open 4.6", generate: generate}
		qa := &stubProvider{name: "Gemini Pro 3.1", generate: generate}

		chat := NewChat(
			ChatWithRandom(rand.New(rand.NewSource(7))),
			ChatWithProviders(projectManager, teamLead, qa),
		)
		chat.SetMode("IMPLEMENT")

		convey.Convey("When an implementation request is submitted", func() {
			chat.Submit("add a command palette and integration tests")

			convey.Convey("It should run the team workflow with board, channels, progress, QA, and review", func() {
				transcript := strings.Join(chat.Lines(), "\n")
				systemPrompts := []string{}
				for _, current := range []*stubProvider{projectManager, teamLead, qa} {
					for _, request := range current.requests {
						systemPrompts = append(systemPrompts, request.SystemPrompt)
					}
				}

				convey.So(transcript, convey.ShouldContainSubstring, "Project board:")
				convey.So(transcript, convey.ShouldContainSubstring, "Team: Project Manager -> Architect -> Team Lead -> Developer 1 -> Developer 2 -> QA -> Review")
				convey.So(transcript, convey.ShouldContainSubstring, "Assignment: Developer 1 owns")
				convey.So(transcript, convey.ShouldContainSubstring, "Assignment: Developer 2 owns")
				convey.So(transcript, convey.ShouldContainSubstring, "Channel coordination: Developer 1 intends to change")
				convey.So(transcript, convey.ShouldContainSubstring, "Channel coordination: Team Lead confirms Developer 1 is clear to proceed")
				convey.So(transcript, convey.ShouldContainSubstring, "Progress: Architect produced implementation plan.")
				convey.So(transcript, convey.ShouldContainSubstring, "Progress: Team Lead assigned 2 developer(s) and published the current plan.")
				convey.So(transcript, convey.ShouldContainSubstring, "Progress: Developer 1 reported implementation progress to the chat.")
				convey.So(transcript, convey.ShouldContainSubstring, "Progress: QA reviewed the implementation and test plan.")
				convey.So(transcript, convey.ShouldContainSubstring, "Review: QA requested improvements.")
				convey.So(transcript, convey.ShouldContainSubstring, "Review: QA final decision PASS.")
				convey.So(transcript, convey.ShouldContainSubstring, "Accept with :accept or :reject.")
				convey.So(strings.Join(systemPrompts, "\n"), convey.ShouldContainSubstring, "You are the Project Manager")
				convey.So(strings.Join(systemPrompts, "\n"), convey.ShouldContainSubstring, "You are the Architect")
				convey.So(strings.Join(systemPrompts, "\n"), convey.ShouldContainSubstring, "You are the Team Lead")
				convey.So(strings.Join(systemPrompts, "\n"), convey.ShouldContainSubstring, "You are the Developer 1")
				convey.So(strings.Join(systemPrompts, "\n"), convey.ShouldContainSubstring, "You are the QA")
				convey.So(strings.Join(systemPrompts, "\n"), convey.ShouldContainSubstring, "You are the Review")
			})
		})
	})
}

func BenchmarkChatSubmit(b *testing.B) {
	root := b.TempDir()
	os.WriteFile(filepath.Join(root, "note.txt"), []byte("hello\nworld"), 0o644)

	chat := NewChat(
		ChatWithRoot(root),
		ChatWithRandom(rand.New(rand.NewSource(13))),
		ChatWithProviders(
			&stubProvider{name: "OpenAI GPT-5.4"},
			&stubProvider{name: "Claude Open 4.6"},
			&stubProvider{name: "Gemini Pro 3.1"},
		),
	)

	for b.Loop() {
		chat.mu.Lock()
		chat.history = nil
		chat.mu.Unlock()
		chat.Submit("read note.txt")
	}
}
