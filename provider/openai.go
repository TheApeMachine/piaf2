package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
	"github.com/theapemachine/piaf/core"
)

/*
OpenAIProvider calls the OpenAI Responses API via the official SDK.
Supports streaming, tool calling, and custom base URLs.
*/
type OpenAIProvider struct {
	ctx     context.Context
	cancel  context.CancelFunc
	baseURL string
	apiKey  string
	model   string
	client  openai.Client
}

type openaiOpts func(*OpenAIProvider)

/*
NewOpenAIProvider instantiates an OpenAIProvider from config and environment.
*/
func NewOpenAIProvider(opts ...openaiOpts) *OpenAIProvider {
	provider := &OpenAIProvider{
		baseURL: "https://api.openai.com/v1",
		model:   "gpt-4o",
		apiKey:  os.Getenv("OPENAI_API_KEY"),
	}

	if core.Cfg != nil {
		if url := core.Cfg.AI.Provider.OpenAI.BaseURL; url != "" {
			provider.baseURL = url
		}
		if model := core.Cfg.AI.Provider.OpenAI.Model; model != "" {
			provider.model = model
		}
	}

	for _, opt := range opts {
		opt(provider)
	}

	clientOpts := []option.RequestOption{option.WithAPIKey(provider.apiKey)}
	if provider.baseURL != "" {
		base := provider.baseURL
		if !strings.HasSuffix(base, "/") {
			base += "/"
		}
		clientOpts = append(clientOpts, option.WithBaseURL(base))
	}

	provider.client = openai.NewClient(clientOpts...)

	return provider
}

/*
Name returns the provider display name.
*/
func (provider *OpenAIProvider) Name() string {
	return "OpenAI"
}

/*
Generate performs a Responses API request using the official SDK.
When Tools and ToolExecutor are set, loops until the model returns text.
*/
func (provider *OpenAIProvider) Generate(ctx context.Context, request *Request) (response string, err error) {
	if provider.apiKey == "" {
		return "", fmt.Errorf("%s is not configured: missing OPENAI_API_KEY", provider.Name())
	}

	req := request
	for {
		params := provider.buildResponseParams(req, nil)
		resp, err := provider.client.Responses.New(ctx, params)
		if err != nil {
			return "", provider.wrapErr(err)
		}

		text := strings.TrimSpace(resp.OutputText())
		if req.ToolExecutor == nil {
			return text, nil
		}

		callOutputs, hasCalls := provider.collectToolCallsFromOutput(resp.Output, nil)
		if !hasCalls {
			return text, nil
		}

		toolOutputs := make([]ToolCallOutput, 0, len(callOutputs))
		for _, call := range callOutputs {
			output, execErr := req.ToolExecutor(call.Name, call.Args)
			if execErr != nil {
				return "", fmt.Errorf("%s tool %s: %w", provider.Name(), call.Name, execErr)
			}
			toolOutputs = append(toolOutputs, ToolCallOutput{CallID: call.CallID, Output: output})
		}

		req = &Request{
			Mode:              req.Mode,
			Message:           req.Message,
			ToolOutput:        req.ToolOutput,
			Transcript:        req.Transcript,
			PriorResponse:     req.PriorResponse,
			SystemPrompt:      req.SystemPrompt,
			Tools:             req.Tools,
			ToolExecutor:      req.ToolExecutor,
			PreviousResponseID: resp.ID,
			ToolCallOutputs:   toolOutputs,
		}
	}
}

/*
GenerateStream performs a streaming Responses API request using 
the official SDK. When Tools and ToolExecutor are set, handles 
tool calls by continuing after each stream completes.
*/
func (provider *OpenAIProvider) GenerateStream(
	ctx context.Context,
	request *Request,
	onChunk func(string),
) (response string, err error) {
	if provider.apiKey == "" {
		return "", fmt.Errorf("%s is not configured: missing OPENAI_API_KEY", provider.Name())
	}

	req := request
	for {
		params := provider.buildResponseParams(req, nil)
		stream := provider.client.Responses.NewStreaming(ctx, params)
		var full strings.Builder
		var completed *responses.Response

		for stream.Next() {
			event := stream.Current()
			if event.Delta != "" {
				full.WriteString(event.Delta)
				if onChunk != nil {
					onChunk(event.Delta)
				}
			}
		
			if event.Type == "response.completed" {
				c := event.AsResponseCompleted()
				completed = &c.Response
			}
		}

		if err := stream.Err(); err != nil {
			return full.String(), provider.wrapErr(err)
		}

		if req.ToolExecutor == nil || completed == nil {
			return strings.TrimSpace(full.String()), nil
		}

		callOutputs, hasCalls := provider.collectToolCallsFromOutput(completed.Output, nil)

		if !hasCalls {
			return strings.TrimSpace(completed.OutputText()), nil
		}

		toolOutputs := make([]ToolCallOutput, 0, len(callOutputs))
		for _, call := range callOutputs {
			if onChunk != nil {
				onChunk(fmt.Sprintf("\n[Tool call: %s]\n", call.Name))
			}
			output, execErr := req.ToolExecutor(call.Name, call.Args)
		
			if execErr != nil {
				return full.String(), fmt.Errorf("%s tool %s: %w", provider.Name(), call.Name, execErr)
			}
		
			toolOutputs = append(toolOutputs, ToolCallOutput{CallID: call.CallID, Output: output})
		}

		req = &Request{
			Mode:              req.Mode,
			Message:           req.Message,
			ToolOutput:        req.ToolOutput,
			Transcript:        req.Transcript,
			PriorResponse:     req.PriorResponse,
			SystemPrompt:      req.SystemPrompt,
			Tools:             req.Tools,
			ToolExecutor:      req.ToolExecutor,
			PreviousResponseID: completed.ID,
			ToolCallOutputs:   toolOutputs,
		}
	}
}

func (provider *OpenAIProvider) wrapErr(err error) error {
	var apierr *openai.Error
	
	if errors.As(err, &apierr) {
		return fmt.Errorf(
			"%s: %s",
			provider.Name(),
			parseAPIError(apierr.DumpResponse(false)),
		)
	}

	return err
}

type toolCall struct {
	CallID string
	Name   string
	Args   map[string]any
}

func (provider *OpenAIProvider) collectToolCallsFromOutput(
	output []responses.ResponseOutputItemUnion,
	onToolCall func(string),
) ([]toolCall, bool) {
	var calls []toolCall
	
	for _, item := range output {
		if item.Type != "function_call" {
			continue
		}
	
		fc := item.AsFunctionCall()
		var args map[string]any
	
		if fc.Arguments != "" {
			_ = json.Unmarshal([]byte(fc.Arguments), &args)
		}

		if args == nil {
			args = make(map[string]any)
		}

		calls = append(calls, toolCall{CallID: fc.CallID, Name: fc.Name, Args: args})
	}
	return calls, len(calls) > 0
}

func (provider *OpenAIProvider) buildResponseParams(
	request *Request,
	onToolCall func(string),
) responses.ResponseNewParams {
	system := BuildSystemPrompt(request)
	user := BuildUserPrompt(request)

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(provider.model),
	}

	if request.PreviousResponseID != "" && len(request.ToolCallOutputs) > 0 {
		items := make([]responses.ResponseInputItemUnionParam, 0, len(request.ToolCallOutputs))
		
		for _, out := range request.ToolCallOutputs {
			items = append(items, responses.ResponseInputItemParamOfFunctionCallOutput(out.CallID, out.Output))
		}
		
		params.PreviousResponseID = openai.String(request.PreviousResponseID)
		params.Input = responses.ResponseNewParamsInputUnion{
			OfInputItemList: items,
		}
	} else {
		params.Input = responses.ResponseNewParamsInputUnion{
			OfString: openai.String(user),
		}
	}

	if system != "" {
		params.Instructions = openai.String(system)
	}

	if len(request.Tools) > 0 {
		params.Tools = make([]responses.ToolUnionParam, 0, len(request.Tools))
		
		for _, tool := range request.Tools {
			toolParams := tool.Parameters
		
			if toolParams == nil {
				toolParams = map[string]any{"type": "object"}
			}
		
			params.Tools = append(params.Tools, responses.ToolUnionParam{
				OfFunction: &responses.FunctionToolParam{
					Name:        tool.Name,
					Description: openai.String(tool.Description),
					Parameters:  toolParams,
				},
			})
		}
	}

	return params
}

/*
OpenAIWithContext configures OpenAIProvider with a context.
*/
func OpenAIWithContext(ctx context.Context) openaiOpts {
	return func(provider *OpenAIProvider) {
		provider.ctx, provider.cancel = context.WithCancel(ctx)
	}
}

/*
OpenAIWithBaseURL configures OpenAIProvider with a custom base URL.
*/
func OpenAIWithBaseURL(baseURL string) openaiOpts {
	return func(provider *OpenAIProvider) {
		provider.baseURL = baseURL
	}
}

/*
OpenAIWithModel configures OpenAIProvider with a model name.
*/
func OpenAIWithModel(model string) openaiOpts {
	return func(provider *OpenAIProvider) {
		provider.model = model
	}
}

/*
OpenAIWithAPIKey configures OpenAIProvider with an API key.
*/
func OpenAIWithAPIKey(apiKey string) openaiOpts {
	return func(provider *OpenAIProvider) {
		provider.apiKey = apiKey
	}
}
