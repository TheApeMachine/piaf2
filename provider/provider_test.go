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

func TestParseAPIError(t *testing.T) {
	convey.Convey("Given API error JSON", t, func() {
		convey.Convey("When body has error.message and error.code", func() {
			body := []byte(`{"error":{"code":404,"message":"models/gemini-pro-3.1 is not found for API version v1beta","status":"NOT_FOUND"}}`)
			convey.Convey("It should return a single-line message with code prefix", func() {
				convey.So(parseAPIError(body), convey.ShouldEqual, "404: models/gemini-pro-3.1 is not found for API version v1beta")
			})
		})

		convey.Convey("When body has error.message and string code", func() {
			body := []byte(`{"error":{"message":"Project does not have access to model","code":"model_not_found"}}`)
			convey.Convey("It should return message with code prefix", func() {
				convey.So(parseAPIError(body), convey.ShouldEqual, "model_not_found: Project does not have access to model")
			})
		})

		convey.Convey("When body is not valid JSON", func() {
			body := []byte("plain text error")
			convey.Convey("It should return truncated raw", func() {
				convey.So(parseAPIError(body), convey.ShouldEqual, "plain text error")
			})
		})

		convey.Convey("When body has newlines in JSON", func() {
			body := []byte("{\"error\":{\"message\":\"not configured\"}}\n")
			convey.Convey("It should return clean message", func() {
				convey.So(parseAPIError(body), convey.ShouldEqual, "not configured")
			})
		})
	})
}

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
			writer.Write([]byte(`{"id":"resp_1","output":[{"content":[{"type":"output_text","text":"first response"}]}]}`))
		}))
		defer server.Close()

		provider := NewOpenAIProvider(
			OpenAIWithBaseURL(server.URL),
			OpenAIWithAPIKey("openai-key"),
			OpenAIWithModel("gpt-5.4"),
		)

		convey.Convey("When Generate is called", func() {
			response, err := provider.Generate(context.Background(), &Request{
				Mode:       "CHAT",
				Message:    "browse .",
				ToolOutput: "Tool browse .",
			})

			convey.Convey("It should call the responses endpoint", func() {
				convey.So(err, convey.ShouldBeNil)
				convey.So(response, convey.ShouldEqual, "first response")
				convey.So(path, convey.ShouldEqual, "/responses")
				convey.So(auth, convey.ShouldEqual, "Bearer openai-key")
				convey.So(body["model"], convey.ShouldEqual, "gpt-5.4")
			})
		})
	})
}

func TestClaudeProviderGenerate(t *testing.T) {
	convey.Convey("Given a ClaudeProvider with mock server", t, func() {
		var path string
		var apiKey string

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			path = request.URL.Path
			apiKey = request.Header.Get("x-api-key")

			writer.Header().Set("Content-Type", "application/json")
			writer.Write([]byte(`{"id":"msg_1","container":{},"content":[{"type":"text","text":"second response"}],"model":"claude-open-4.6","role":"assistant","stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":0,"output_tokens":1}}`))
		}))
		defer server.Close()

		provider := NewClaudeProvider(
			ClaudeWithBaseURL(server.URL),
			ClaudeWithAPIKey("claude-key"),
			ClaudeWithModel("claude-open-4.6"),
		)

		convey.Convey("When Generate is called", func() {
			response, err := provider.Generate(context.Background(), &Request{
				Mode:    "CHAT",
				Message: "hello",
			})

			convey.Convey("It should call the messages endpoint", func() {
				convey.So(err, convey.ShouldBeNil)
				convey.So(response, convey.ShouldEqual, "second response")
				convey.So(strings.Contains(path, "messages"), convey.ShouldBeTrue)
				convey.So(apiKey, convey.ShouldEqual, "claude-key")
			})
		})
	})
}

func TestGeminiProviderGenerate(t *testing.T) {
	convey.Convey("Given a GeminiProvider with mock server", t, func() {
		var path string

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			path = request.URL.Path

			writer.Header().Set("Content-Type", "application/json")
			writer.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"third response"}]}}]}`))
		}))
		defer server.Close()

		provider := NewGeminiProvider(
			GeminiWithBaseURL(server.URL),
			GeminiWithModel("gemini-pro-3.1"),
		)

		convey.Convey("When Generate is called", func() {
			response, err := provider.Generate(context.Background(), &Request{
				Mode:    "IMPLEMENT",
				Message: "add provider support",
			})

			convey.Convey("It should call generateContent and return the parsed response", func() {
				convey.So(err, convey.ShouldBeNil)
				convey.So(response, convey.ShouldEqual, "third response")
				convey.So(strings.Contains(path, "generateContent"), convey.ShouldBeTrue)
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
