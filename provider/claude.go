package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
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
*/
func (provider *ClaudeProvider) Generate(ctx context.Context, request *Request) (response string, err error) {
	if provider.apiKey == "" {
		return "", fmt.Errorf("%s is not configured: missing ANTHROPIC_API_KEY", provider.Name())
	}

	msg, err := provider.client.Messages.New(ctx, provider.buildParams(request))
	if err != nil {
		return "", provider.wrapErr(err)
	}

	text := strings.TrimSpace(provider.extractText(msg.Content))
	if text == "" {
		return "", fmt.Errorf("%s returned no content", provider.Name())
	}

	return text, nil
}

/*
GenerateStream performs a streaming Messages API request.
*/
func (provider *ClaudeProvider) GenerateStream(
	ctx context.Context,
	request *Request,
	onChunk func(string),
) (response string, err error) {
	if provider.apiKey == "" {
		return "", fmt.Errorf("%s is not configured: missing ANTHROPIC_API_KEY", provider.Name())
	}

	stream := provider.client.Messages.NewStreaming(ctx, provider.buildParams(request))
	var full strings.Builder

	for stream.Next() {
		event := stream.Current()
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
	if text == "" {
		return "", fmt.Errorf("%s returned no content", provider.Name())
	}

	return text, nil
}

func (provider *ClaudeProvider) buildParams(request *Request) anthropic.MessageNewParams {
	system := BuildSystemPrompt(request)
	user := BuildUserPrompt(request)

	params := anthropic.MessageNewParams{
		Model:     provider.model,
		MaxTokens: 2048,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(user)),
		},
	}

	if system != "" {
		params.System = []anthropic.TextBlockParam{{Text: system}}
	}

	return params
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
