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
	}

	stub.requests = append(stub.requests, copyRequest)

	index := len(stub.requests) - 1
	if index < len(stub.responses) {
		return stub.responses[index], nil
	}

	return fmt.Sprintf("%s response %d", stub.name, index+1), nil
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
			ChatWithProviders(openai, claude, gemini),
		)

		convey.Convey("When a browse prompt is submitted", func() {
			chat.Submit("browse .")

			convey.Convey("It should add a full three-model pipeline to the transcript", func() {
				transcript := strings.Join(chat.history, "\n")

				convey.So(chat.history, convey.ShouldHaveLength, 5)
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
				transcript := strings.Join(chat.history, "\n")
				convey.So(transcript, convey.ShouldContainSubstring, "Accept with :accept or :reject.")
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
		chat.history = nil
		chat.Submit("read note.txt")
	}
}
