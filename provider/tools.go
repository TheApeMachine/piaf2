package provider

import (
	"fmt"
	"sync"
)

/*
DiscussionToolBackend provides workspace and memory operations for tool execution.
Implement this interface to wire DiscussionTools to a chat or similar context.
*/
type DiscussionToolBackend interface {
	Browse(path string) string
	Read(path string) string
	Remember(content string) string
	Recall(filter string) string
	Forget(filter string) string
}

/*
WithToolLimit wraps a ToolExecutor with a hard limit on executions.
Informs the LLM inside the tool output about remaining operations.
*/
func WithToolLimit(executor func(string, map[string]any) (string, error), limit int) func(string, map[string]any) (string, error) {
	if executor == nil {
		return nil
	}

	var mu sync.Mutex
	return func(name string, args map[string]any) (string, error) {
		mu.Lock()
		if limit <= 0 {
			mu.Unlock()
			return "Tool execution rejected: You have exhausted your tool call limit for this turn. You MUST output your final answer now without using any further tools.", nil
		}
		limit--
		rem := limit
		mu.Unlock()

		res, err := executor(name, args)
		if err == nil {
			if rem > 0 {
				res = fmt.Sprintf("%s\n\n[System: You have %d tool call(s) remaining for this turn.]", res, rem)
			} else {
				res = fmt.Sprintf("%s\n\n[System: You have exhausted your tool calls. You MUST output your final answer now.]", res)
			}
		}
		return res, err
	}
}

/*
NewDiscussionToolExecutor returns a ToolExecutor that dispatches to the backend.
*/
func NewDiscussionToolExecutor(backend DiscussionToolBackend) func(string, map[string]any) (string, error) {
	return func(name string, args map[string]any) (string, error) {
		switch name {
		case "browse":
			path, _ := args["path"].(string)
			if path == "" {
				path = "."
			}
			return backend.Browse(path), nil
		case "read":
			path, _ := args["path"].(string)
			if path == "" {
				return "", fmt.Errorf("read requires path")
			}
			return backend.Read(path), nil
		case "remember":
			content, _ := args["content"].(string)
			return backend.Remember(content), nil
		case "recall":
			filter, _ := args["filter"].(string)
			return backend.Recall(filter), nil
		case "forget":
			filter, _ := args["filter"].(string)
			return backend.Forget(filter), nil
		default:
			return "", fmt.Errorf("unknown tool: %s", name)
		}
	}
}

/*
DiscussionTools returns tool definitions for browse, read, remember, recall, and forget.
Shared across OpenAI, Claude, and Gemini providers so chat agents can inspect the workspace.
*/
func DiscussionTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "browse",
			Description: "List files and directories in the workspace. Use to explore the project structure.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Directory path to list, e.g. '.' or 'editor/'",
					},
				},
			},
		},
		{
			Name:        "read",
			Description: "Read the contents of a file in the workspace. Use to inspect source code or config.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "File path to read, e.g. 'editor/chat.go'",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "remember",
			Description: "Store a fact or note in shared team memory for later recall.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"content": map[string]any{
						"type":        "string",
						"description": "The fact or note to store",
					},
				},
				"required": []string{"content"},
			},
		},
		{
			Name:        "recall",
			Description: "Search shared and agent memory for facts matching a filter.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filter": map[string]any{
						"type":        "string",
						"description": "Substring to search for in stored memories",
					},
				},
			},
		},
		{
			Name:        "forget",
			Description: "Remove memories that match the given filter from shared and agent memory.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filter": map[string]any{
						"type":        "string",
						"description": "Substring to match for removal",
					},
				},
				"required": []string{"filter"},
			},
		},
	}
}
