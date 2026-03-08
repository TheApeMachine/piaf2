package editor

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

/*
Chat renders the multi-model discussion and implementation transcript.
It keeps a running context and can inspect files inside the workspace.
*/
type Chat struct {
	root    string
	mode    string
	history []string
	random  *rand.Rand
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
		root:   root,
		mode:   "CHAT",
		random: rand.New(rand.NewSource(time.Now().UnixNano())),
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
	chat.history = append(chat.history, "Pipeline: "+strings.Join(order, " -> "))

	firstResponse := chat.composeResponse(order[0], message, "", toolOutput, 0)
	secondResponse := chat.composeResponse(order[1], message, firstResponse, toolOutput, 1)
	thirdResponse := chat.composeResponse(order[2], message, secondResponse, toolOutput, 2)

	chat.history = append(chat.history,
		order[0]+": "+firstResponse,
		order[1]+": "+secondResponse,
		order[2]+": "+thirdResponse,
	)
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

func (chat *Chat) randomizedModels() []string {
	models := []string{
		"OpenAI GPT-5.4",
		"Claude Open 4.6",
		"Gemini Pro 3.1",
	}

	chat.random.Shuffle(len(models), func(left, right int) {
		models[left], models[right] = models[right], models[left]
	})

	return models
}

func (chat *Chat) composeResponse(model, message, previous, toolOutput string, stage int) string {
	if chat.mode == "IMPLEMENT" {
		switch stage {
		case 0:
			return "I scoped the implementation request and identified the main edit surface."
		case 1:
			if toolOutput != "" {
				return "Proposed diff plan:\n+ use the inspected files as the edit targets\n+ keep the change minimal\n" + toolOutput
			}

			return "Proposed diff plan:\n+ update the relevant editor flow\n+ keep the change minimal"
		default:
			return "Final implementation summary: " + previous + "\nAccept with :accept or :reject."
		}
	}

	if toolOutput != "" {
		switch stage {
		case 0:
			return "I inspected the workspace before answering.\n" + toolOutput
		case 1:
			return "Building on the previous answer, the relevant evidence is:\n" + toolOutput
		default:
			return "Final response: considering `" + message + "` and the earlier context, " + strings.ToLower(model) + " recommends continuing with the inspected evidence."
		}
	}

	if previous != "" {
		return "I considered the earlier response and kept the running context in view."
	}

	return "I considered the prompt and prepared the next stage of the discussion."
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
