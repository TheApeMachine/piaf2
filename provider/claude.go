package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
)

/*
ClaudeProvider calls the Anthropic Messages API via the official SDK.
Supports streaming and custom base URLs.
*/
type ClaudeProvider struct {
	client  anthropic.Client
	model   anthropic.Model
	apiKey  string
	baseURL string
}

type claudeOpts func(*ClaudeProvider)

/*
NewClaudeProvider instantiates a ClaudeProvider from config and environment.
*/
func NewClaudeProvider(opts ...claudeOpts) *ClaudeProvider {
	provider := &ClaudeProvider{
		model: anthropic.ModelClaudeOpus4_6,
	}

	if model := os.Getenv("CLAUDE_MODEL"); model != "" {
		provider.model = anthropic.Model(model)
	}

	for _, opt := range opts {
		opt(provider)
	}

	if provider.client.Options == nil {
		apiKey := provider.apiKey
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
			provider.apiKey = apiKey
		}
		clientOpts := []option.RequestOption{option.WithAPIKey(apiKey)}

		baseURL := provider.baseURL
		if baseURL == "" {
			baseURL = os.Getenv("CLAUDE_BASE_URL")
		}
		if baseURL != "" {
			clientOpts = append(clientOpts, option.WithBaseURL(strings.TrimRight(baseURL, "/")))
		}

		provider.client = anthropic.NewClient(clientOpts...)
	}

	return provider
}

/*
Name returns the provider display name.
*/
func (provider *ClaudeProvider) Name() string {
	return "Claude Opus 4.6"
}

/*
Generate performs a non-streaming Messages API request.
When Tools and ToolExecutor are set, loops until the model returns text.
*/
func (provider *ClaudeProvider) Generate(ctx context.Context, request *Request) (response string, err error) {
	if provider.apiKey == "" {
		return "", fmt.Errorf("%s is not configured: missing ANTHROPIC_API_KEY", provider.Name())
	}

	messages := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(BuildUserPrompt(request)))}

	for {
		params := provider.buildParamsWithMessages(request, messages)
		msg, err := provider.client.Messages.New(ctx, params)
		if err != nil {
			return "", provider.wrapErr(err)
		}

		text := strings.TrimSpace(provider.extractText(msg.Content))
		if request.ToolExecutor == nil {
			return text, nil
		}

		toolResults, hasCalls := provider.collectToolCalls(msg.Content, request.ToolExecutor, nil)
		if !hasCalls {
			return text, nil
		}

		messages = append(messages, msg.ToParam())
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}
}

/*
GenerateStream performs a streaming Messages API request.
When Tools and ToolExecutor are set, handles tool calls by continuing after each stream completes.
*/
func (provider *ClaudeProvider) GenerateStream(
	ctx context.Context,
	request *Request,
	onChunk func(string),
) (response string, err error) {
	if provider.apiKey == "" {
		return "", fmt.Errorf("%s is not configured: missing ANTHROPIC_API_KEY", provider.Name())
	}

	messages := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(BuildUserPrompt(request)))}

	for {
		params := provider.buildParamsWithMessages(request, messages)
		stream := provider.client.Messages.NewStreaming(ctx, params)
		var full strings.Builder
		finalMessage := &anthropic.Message{}

		for stream.Next() {
			event := stream.Current()
			_ = finalMessage.Accumulate(event)
			switch eventVariant := event.AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				switch deltaVariant := eventVariant.Delta.AsAny().(type) {
				case anthropic.TextDelta:
					if deltaVariant.Text != "" {
						full.WriteString(deltaVariant.Text)
						if onChunk != nil {
							onChunk(deltaVariant.Text)
						}
					}
				}
			}
		}

		if stream.Err() != nil {
			return full.String(), provider.wrapErr(stream.Err())
		}

		text := strings.TrimSpace(full.String())
		if request.ToolExecutor == nil {
			if text == "" {
				return "", fmt.Errorf("%s returned no content", provider.Name())
			}
			return text, nil
		}

		toolResults, hasCalls := provider.collectToolCalls(finalMessage.Content, request.ToolExecutor, onChunk)
		if !hasCalls {
			if text == "" {
				return "", fmt.Errorf("%s returned no content", provider.Name())
			}
			return text, nil
		}

		messages = append(messages, finalMessage.ToParam())
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}
}

func (provider *ClaudeProvider) buildParams(request *Request) anthropic.MessageNewParams {
	return provider.buildParamsWithMessages(request, []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(BuildUserPrompt(request))),
	})
}

func (provider *ClaudeProvider) buildParamsWithMessages(request *Request, messages []anthropic.MessageParam) anthropic.MessageNewParams {
	system := BuildSystemPrompt(request)
	params := anthropic.MessageNewParams{
		Model:     provider.model,
		MaxTokens: 2048,
		Messages:  messages,
	}
	if system != "" {
		params.System = []anthropic.TextBlockParam{{Text: system}}
	}
	if len(request.Tools) > 0 {
		params.Tools = provider.buildTools(request.Tools)
	}
	return params
}

func (provider *ClaudeProvider) buildTools(tools []ToolDefinition) []anthropic.ToolUnionParam {
	out := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		schema := anthropic.ToolInputSchemaParam{Type: constant.Object("object")}
		if tool.Parameters != nil {
			if props, ok := tool.Parameters["properties"].(map[string]any); ok {
				schema.Properties = props
			}
			if req, ok := tool.Parameters["required"].([]any); ok {
				for _, r := range req {
					if s, ok := r.(string); ok {
						schema.Required = append(schema.Required, s)
					}
				}
			}
		}
		out = append(out, anthropic.ToolUnionParam{OfTool: &anthropic.ToolParam{
			Name:         tool.Name,
			Description: anthropic.String(tool.Description),
			InputSchema: schema,
		}})
	}
	return out
}

func (provider *ClaudeProvider) collectToolCalls(content []anthropic.ContentBlockUnion, executor func(string, map[string]any) (string, error), onChunk func(string)) ([]anthropic.ContentBlockParamUnion, bool) {
	var results []anthropic.ContentBlockParamUnion
	for _, block := range content {
		tu, ok := block.AsAny().(anthropic.ToolUseBlock)
		if !ok {
			continue
		}
		args := make(map[string]any)
		if len(tu.Input) > 0 {
			_ = json.Unmarshal(tu.Input, &args)
		}
		if onChunk != nil {
			onChunk(fmt.Sprintf("\n[Tool call: %s]\n", tu.Name))
		}
		output, err := executor(tu.Name, args)
		if err != nil {
			output = "Error: " + err.Error()
		}
		results = append(results, anthropic.NewToolResultBlock(tu.ID, output, err != nil))
	}
	return results, len(results) > 0
}

func (provider *ClaudeProvider) extractText(content []anthropic.ContentBlockUnion) string {
	var parts []string
	for _, block := range content {
		switch b := block.AsAny().(type) {
		case anthropic.TextBlock:
			if b.Text != "" {
				parts = append(parts, strings.TrimSpace(b.Text))
			}
		}
	}
	return strings.Join(parts, "\n")
}

func (provider *ClaudeProvider) wrapErr(err error) error {
	var apierr *anthropic.Error
	if errors.As(err, &apierr) {
		return fmt.Errorf("%s: %s", provider.Name(), parseAPIError(apierr.DumpResponse(false)))
	}
	return err
}

/*
ClaudeWithClient configures ClaudeProvider with an anthropic client.
*/
func ClaudeWithClient(client anthropic.Client) claudeOpts {
	return func(provider *ClaudeProvider) {
		provider.client = client
	}
}

/*
ClaudeWithBaseURL configures ClaudeProvider with a custom base URL.
*/
func ClaudeWithBaseURL(baseURL string) claudeOpts {
	return func(provider *ClaudeProvider) {
		provider.baseURL = strings.TrimRight(baseURL, "/")
	}
}

/*
ClaudeWithAPIKey configures ClaudeProvider with an API key.
*/
func ClaudeWithAPIKey(apiKey string) claudeOpts {
	return func(provider *ClaudeProvider) {
		provider.apiKey = apiKey
	}
}

/*
ClaudeWithModel configures ClaudeProvider with a model name.
*/
func ClaudeWithModel(model string) claudeOpts {
	return func(provider *ClaudeProvider) {
		provider.model = anthropic.Model(model)
	}
}
