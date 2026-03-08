package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestOpenAIProviderGenerate(t *testing.T) {
	convey.Convey("Given an OpenAIProvider", t, func() {
		var path string
		var auth string
		var body map[string]any

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			path = request.URL.Path
			auth = request.Header.Get("Authorization")

			data, _ := io.ReadAll(request.Body)
			json.Unmarshal(data, &body)

			writer.Header().Set("Content-Type", "application/json")
			writer.Write([]byte(`{"choices":[{"message":{"content":"first response"}}]}`))
		}))
		defer server.Close()

		provider := &OpenAIProvider{
			baseURL: server.URL,
			apiKey:  "openai-key",
			model:   "gpt-5.4",
			client:  server.Client(),
		}

		convey.Convey("When Generate is called", func() {
			response, err := provider.Generate(context.Background(), &Request{
				Mode:       "CHAT",
				Message:    "browse .",
				ToolOutput: "Tool browse .",
			})

			convey.Convey("It should call the chat completions endpoint", func() {
				convey.So(err, convey.ShouldBeNil)
				convey.So(response, convey.ShouldEqual, "first response")
				convey.So(path, convey.ShouldEqual, "/chat/completions")
				convey.So(auth, convey.ShouldEqual, "Bearer openai-key")
				convey.So(body["model"], convey.ShouldEqual, "gpt-5.4")
			})
		})
	})
}

func TestClaudeProviderGenerate(t *testing.T) {
	convey.Convey("Given a ClaudeProvider", t, func() {
		var path string
		var apiKey string

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			path = request.URL.Path
			apiKey = request.Header.Get("x-api-key")

			writer.Header().Set("Content-Type", "application/json")
			writer.Write([]byte(`{"content":[{"type":"text","text":"second response"}]}`))
		}))
		defer server.Close()

		provider := &ClaudeProvider{
			baseURL: server.URL,
			apiKey:  "claude-key",
			model:   "claude-open-4.6",
			client:  server.Client(),
		}

		convey.Convey("When Generate is called", func() {
			response, err := provider.Generate(context.Background(), &Request{
				Mode:    "CHAT",
				Message: "hello",
			})

			convey.Convey("It should call the messages endpoint", func() {
				convey.So(err, convey.ShouldBeNil)
				convey.So(response, convey.ShouldEqual, "second response")
				convey.So(path, convey.ShouldEqual, "/messages")
				convey.So(apiKey, convey.ShouldEqual, "claude-key")
			})
		})
	})
}

func TestGeminiProviderGenerate(t *testing.T) {
	convey.Convey("Given a GeminiProvider", t, func() {
		var endpoint string

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			endpoint = request.URL.String()

			writer.Header().Set("Content-Type", "application/json")
			writer.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"third response"}]}}]}`))
		}))
		defer server.Close()

		provider := &GeminiProvider{
			baseURL: server.URL,
			apiKey:  "gemini-key",
			model:   "gemini-pro-3.1",
			client:  server.Client(),
		}

		convey.Convey("When Generate is called", func() {
			response, err := provider.Generate(context.Background(), &Request{
				Mode:    "IMPLEMENT",
				Message: "add provider support",
			})

			convey.Convey("It should call generateContent with the key in the query string", func() {
				convey.So(err, convey.ShouldBeNil)
				convey.So(response, convey.ShouldEqual, "third response")
				convey.So(strings.HasPrefix(endpoint, "/models/gemini-pro-3.1:generateContent?key=gemini-key"), convey.ShouldBeTrue)
			})
		})
	})
}

func BenchmarkBuildUserPrompt(b *testing.B) {
	request := &Request{
		Mode:       "IMPLEMENT",
		Message:    "add provider support",
		ToolOutput: "Tool read provider/openai.go",
		Transcript: []string{"You: add provider support", "Pipeline: OpenAI GPT-5.4 -> Claude Open 4.6 -> Gemini Pro 3.1"},
		PriorResponse: []string{
			"OpenAI GPT-5.4: scoped the change",
			"Claude Open 4.6: confirmed the affected files",
		},
	}

	for b.Loop() {
		_ = BuildUserPrompt(request)
	}
}
