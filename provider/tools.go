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
	Read(path string, start, end int) string
	Remember(content string) string
	Recall(filter string) string
	Forget(filter string) string
	Search(query, path string) string
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
		var res string
		var err error

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
			startLine, _ := args["start_line"].(float64)
			endLine, _ := args["end_line"].(float64)
			res, err = backend.Read(path, int(startLine), int(endLine)), nil
		case "remember":
			content, _ := args["content"].(string)
			res, err = backend.Remember(content), nil
		case "recall":
			filter, _ := args["filter"].(string)
			res, err = backend.Recall(filter), nil
		case "forget":
			filter, _ := args["filter"].(string)
			res, err = backend.Forget(filter), nil
		case "search":
			path, _ := args["path"].(string)
			if path == "" {
				path = "."
			}
			query, _ := args["query"].(string)
			if query == "" {
				res, err = "", fmt.Errorf("search requires query parameter")
			} else {
				res, err = backend.Search(query, path), nil
			}
		default:
			res, err = "", fmt.Errorf("unknown tool: %s", name)
		}
		return res, err
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
			Description: "Read the contents of a file in the workspace. Accepts optional start_line and end_line for huge files. 1-indexed.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "File path to read, e.g. 'editor/chat.go'",
					},
					"start_line": map[string]any{
						"type":        "number",
						"description": "Optional starting line number to read from (1-indexed).",
					},
					"end_line": map[string]any{
						"type":        "number",
						"description": "Optional ending line number to read until (1-indexed, inclusive).",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "search",
			Description: "Search for a text string or pattern across files in a directory.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "The exact text or pattern to search for"},
					"path":  map[string]any{"type": "string", "description": "The directory path to search in, defaults to '.'"},
				},
				"required": []string{"query"},
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
