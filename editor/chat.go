package editor

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/theapemachine/piaf/provider"
)

/*
Chat renders the multi-model discussion and implementation transcript.
It keeps a running context and can inspect files inside the workspace.
*/
type Chat struct {
	root         string
	mode         string
	history      []string
	mu           sync.Mutex
	onStream     func()
	onComplete   func()
	random       *rand.Rand
	systemPrompt string
	timeout      time.Duration
	providers    []provider.Provider
}

/*
chatOpts configures Chat with options.
*/
type chatOpts func(*Chat)

/*
NewChat instantiates a new Chat with workspace-scoped tooling.
*/
func NewChat(opts ...chatOpts) *Chat {
	root, err := os.Getwd()
	if err != nil {
		root = "."
	}

	chat := &Chat{
		root:    root,
		mode:    "CHAT",
		random:  rand.New(rand.NewSource(time.Now().UnixNano())),
		timeout: 30 * time.Second,
		providers: []provider.Provider{
			provider.NewOpenAIProvider(),
			provider.NewClaudeProvider(),
			provider.NewGeminiProvider(),
		},
	}

	for _, opt := range opts {
		opt(chat)
	}

	return chat
}

/*
ChatWithRoot scopes file browsing and reading to a workspace root.
*/
func ChatWithRoot(root string) chatOpts {
	return func(chat *Chat) {
		if root == "" {
			return
		}

		absolute, err := filepath.Abs(root)
		if err != nil {
			return
		}

		chat.root = absolute
	}
}

/*
ChatWithRandom configures Chat with a deterministic random source.
*/
func ChatWithRandom(random *rand.Rand) chatOpts {
	return func(chat *Chat) {
		if random != nil {
			chat.random = random
		}
	}
}

/*
ChatWithProviders configures Chat with provider implementations.
*/
func ChatWithProviders(providers ...provider.Provider) chatOpts {
	return func(chat *Chat) {
		if len(providers) == 0 {
			return
		}

		chat.providers = append([]provider.Provider(nil), providers...)
	}
}

/*
ChatWithOnStream registers a callback invoked when streaming produces new content.
*/
func ChatWithOnStream(onStream func()) chatOpts {
	return func(chat *Chat) {
		chat.onStream = onStream
	}
}

/*
ChatWithOnComplete registers a callback invoked when Submit finishes all providers.
*/
func ChatWithOnComplete(onComplete func()) chatOpts {
	return func(chat *Chat) {
		chat.onComplete = onComplete
	}
}

/*
ChatWithSystemPrompt sets the system prompt for provider requests.
When non-empty, overrides the default BuildSystemPrompt for the current mode.
*/
func ChatWithSystemPrompt(prompt string) chatOpts {
	return func(chat *Chat) {
		chat.systemPrompt = strings.TrimSpace(prompt)
	}
}

/*
Mode returns the current chat mode name.
*/
func (chat *Chat) Mode() string {
	return chat.mode
}

/*
SetMode switches between discussion and implementation flows.
*/
func (chat *Chat) SetMode(mode string) {
	if mode == "" {
		return
	}

	chat.mu.Lock()
	chat.mode = mode

	switch mode {
	case "IMPLEMENT":
		chat.history = append(chat.history,
			"System: development team engaged.",
			"System: send implementation prompts and review the generated diff before :accept or :reject.",
		)
	default:
		chat.history = append(chat.history, "System: discussion mode engaged.")
	}
	chat.mu.Unlock()
}

/*
Submit adds a user message and streams the three randomized model responses.
Runs asynchronously; onStream is invoked as each chunk arrives.
*/
func (chat *Chat) Submit(message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	order := chat.randomizedModels()
	toolOutput := chat.toolOutput(message)

	chat.mu.Lock()
	chat.history = append(chat.history, "You: "+message)
	names := make([]string, 0, len(order))
	for _, current := range order {
		names = append(names, current.Name())
	}

	chat.history = append(chat.history, "Pipeline: "+strings.Join(names, " -> "))
	transcript := append([]string(nil), chat.history...)
	chat.mu.Unlock()

	responses := make([]string, 0, len(order))

	for index, current := range order {
		chat.mu.Lock()
		chat.history = append(chat.history, current.Name()+": ")
		transcript = append(transcript, chat.history[len(chat.history)-1])
		chat.mu.Unlock()

		systemPrompt := ""
		if chat.mode == "CHAT" {
			systemPrompt = chat.systemPrompt
		}

		ctx, cancel := context.WithTimeout(context.Background(), chat.timeout)
		response, err := current.GenerateStream(ctx, &provider.Request{
			Mode:          chat.mode,
			Message:       message,
			ToolOutput:    toolOutput,
			Transcript:    transcript,
			PriorResponse: responses,
			SystemPrompt:  systemPrompt,
		}, func(chunk string) {
			chat.mu.Lock()
			last := len(chat.history) - 1
			if last >= 0 {
				chat.history[last] += chunk
				if chat.onStream != nil {
					chat.onStream()
				}
			}
			chat.mu.Unlock()
		})
		cancel()

		chat.mu.Lock()
		last := len(chat.history) - 1
		if err != nil {
			chat.history[last] = current.Name() + ": \033[31mError:\033[0m " + err.Error()
		} else {
			if chat.mode == "IMPLEMENT" && index == len(order)-1 && !strings.Contains(strings.ToLower(response), "accept") {
				chat.history[last] += "\nAccept with :accept or :reject."
			}
		}
		line := chat.history[last]
		transcript = append(transcript, line)
		chat.mu.Unlock()

		responses = append(responses, line)
		if chat.onStream != nil {
			chat.onStream()
		}
	}

	if chat.onComplete != nil {
		chat.onComplete()
	}
}

/*
Accept marks the current implementation proposal as accepted.
*/
func (chat *Chat) Accept() {
	chat.mu.Lock()
	chat.history = append(chat.history, "System: implementation proposal accepted.")
	chat.mu.Unlock()
}

/*
Reject marks the current implementation proposal as rejected.
*/
func (chat *Chat) Reject() {
	chat.mu.Lock()
	chat.history = append(chat.history, "System: implementation proposal rejected.")
	chat.mu.Unlock()
}

/*
Lines returns the transcript for rendering.
*/
func (chat *Chat) Lines() []string {
	chat.mu.Lock()
	defer chat.mu.Unlock()

	if len(chat.history) > 0 {
		return append([]string(nil), chat.history...)
	}

	if chat.mode == "IMPLEMENT" {
		return []string{
			"Implementation window ready.",
			"Press i to send an implementation prompt.",
			"Use :accept or :reject after reviewing the proposal.",
		}
	}

	return []string{
		"Discussion window ready.",
		"Press i to send a message to the three-model discussion.",
		"Use prompts like `browse .` or `read editor/editor.go` to inspect files.",
	}
}

func (chat *Chat) randomizedModels() []provider.Provider {
	models := append([]provider.Provider(nil), chat.providers...)

	chat.random.Shuffle(len(models), func(left, right int) {
		models[left], models[right] = models[right], models[left]
	})

	return models
}

func (chat *Chat) toolOutput(message string) string {
	fields := strings.Fields(message)
	if len(fields) == 0 {
		return ""
	}

	action := strings.ToLower(fields[0])
	target := "."
	if len(fields) > 1 {
		target = strings.Join(fields[1:], " ")
	}

	switch action {
	case "browse", "list", "ls":
		return chat.browse(target)
	case "read", "open", "cat":
		return chat.read(target)
	default:
		return ""
	}
}

func (chat *Chat) browse(target string) string {
	resolved, allowed := chat.resolve(target)
	if !allowed {
		return "Tool browse blocked: path escapes the workspace."
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return "Tool browse failed: " + err.Error()
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}

		names = append(names, name)
	}

	sort.Strings(names)

	lines := []string{"Tool browse " + chat.relative(resolved)}
	for _, name := range names {
		lines = append(lines, "- "+name)
	}

	return strings.Join(lines, "\n")
}

func (chat *Chat) read(target string) string {
	resolved, allowed := chat.resolve(target)
	if !allowed {
		return "Tool read blocked: path escapes the workspace."
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "Tool read failed: " + err.Error()
	}

	if info.IsDir() {
		return "Tool read failed: target is a directory."
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return "Tool read failed: " + err.Error()
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > 12 {
		lines = lines[:12]
	}

	result := []string{"Tool read " + chat.relative(resolved)}
	for index, line := range lines {
		result = append(result, fmt.Sprintf("%d %s", index+1, line))
	}

	return strings.Join(result, "\n")
}

func (chat *Chat) resolve(target string) (string, bool) {
	if target == "" {
		target = "."
	}

	resolved := target
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(chat.root, resolved)
	}

	resolved = filepath.Clean(resolved)
	root := filepath.Clean(chat.root)

	if resolved == root {
		return resolved, true
	}

	if strings.HasPrefix(resolved, root+string(filepath.Separator)) {
		return resolved, true
	}

	return "", false
}

func (chat *Chat) relative(path string) string {
	relative, err := filepath.Rel(chat.root, path)
	if err != nil {
		return path
	}

	if relative == "." {
		return "."
	}

	return relative
}
