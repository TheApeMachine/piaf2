package editor

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/theapemachine/piaf/provider"
)

/*
Chat renders the multi-model discussion and implementation transcript.
It keeps a running context and can inspect files inside the workspace.
*/
type Chat struct {
	root      string
	mode      string
	history   []string
	random    *rand.Rand
	timeout   time.Duration
	providers []provider.Provider
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
}

/*
Submit adds a user message and the three randomized model responses.
*/
func (chat *Chat) Submit(message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	order := chat.randomizedModels()
	toolOutput := chat.toolOutput(message)

	chat.history = append(chat.history, "You: "+message)
	names := make([]string, 0, len(order))
	for _, current := range order {
		names = append(names, current.Name())
	}

	chat.history = append(chat.history, "Pipeline: "+strings.Join(names, " -> "))

	transcript := append([]string(nil), chat.history...)
	responses := make([]string, 0, len(order))

	for index, current := range order {
		ctx, cancel := context.WithTimeout(context.Background(), chat.timeout)
		response, err := current.Generate(ctx, &provider.Request{
			Mode:          chat.mode,
			Message:       message,
			ToolOutput:    toolOutput,
			Transcript:    transcript,
			PriorResponse: responses,
		})
		cancel()

		if err != nil {
			response = "Provider error: " + err.Error()
		}

		if chat.mode == "IMPLEMENT" && index == len(order)-1 && !strings.Contains(strings.ToLower(response), "accept") {
			response += "\nAccept with :accept or :reject."
		}

		line := current.Name() + ": " + response
		chat.history = append(chat.history, line)
		transcript = append(transcript, line)
		responses = append(responses, line)
	}
}

/*
Accept marks the current implementation proposal as accepted.
*/
func (chat *Chat) Accept() {
	chat.history = append(chat.history, "System: implementation proposal accepted.")
}

/*
Reject marks the current implementation proposal as rejected.
*/
func (chat *Chat) Reject() {
	chat.history = append(chat.history, "System: implementation proposal rejected.")
}

/*
Lines returns the transcript for rendering.
*/
func (chat *Chat) Lines() []string {
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
