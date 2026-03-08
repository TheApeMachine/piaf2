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

const (
	projectManagerProviderOffset = 0
	teamLeadPosition             = 0
	teamLeadProviderOffset       = 1
	developerProviderOffset      = 2
	qaProviderOffset             = 3
	reviewProviderOffset         = 4
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
	workflow     *Workflow
	memory       *AgentMemory
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
		root:     root,
		mode:     "CHAT",
		random:   rand.New(rand.NewSource(time.Now().UnixNano())),
		timeout:  30 * time.Second,
		workflow: NewWorkflow(),
		memory:   NewAgentMemory(),
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
		if chat.workflow == nil {
			chat.workflow = NewWorkflow()
		}
		chat.history = append(chat.history,
			"System: development team engaged.",
			"System: project manager, team lead, QA, and developers are now coordinating the implementation.",
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

	chat.mu.Lock()
	chat.history = append(chat.history, "You: "+message)
	chat.mu.Unlock()

	if chat.mode == "IMPLEMENT" {
		chat.submitImplementation(message)
	} else {
		chat.submitDiscussion(message)
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
	case "remember", "memo":
		return chat.remember(target)
	case "recall", "memory":
		return chat.recall(target)
	case "forget":
		return chat.forget(target)
	default:
		return ""
	}
}

func (chat *Chat) submitDiscussion(message string) {
	order := chat.randomizedModels()
	baseToolOutput := chat.toolOutput(message)
	names := make([]string, 0, len(order))
	for _, current := range order {
		names = append(names, current.Name())
	}

	chat.appendHistory("Pipeline: " + strings.Join(names, " -> "))

	transcript := chat.snapshot()
	responses := make([]string, 0, len(order))
	for _, current := range order {
		agentLabel := "Discussion " + current.Name()
		toolOutput := chat.composeToolOutput(baseToolOutput, agentLabel)
		systemPrompt := chat.systemPrompt
		line, response := chat.runStage(current.Name(), current, &provider.Request{
			Mode:          chat.mode,
			Message:       message,
			ToolOutput:    toolOutput,
			Transcript:    transcript,
			PriorResponse: responses,
			SystemPrompt:  systemPrompt,
		})
		transcript = append(transcript, line)
		responses = append(responses, line)
		chat.memory.RememberAgent(agentLabel, response)
	}
}

func (chat *Chat) submitImplementation(message string) {
	if chat.workflow == nil {
		chat.workflow = NewWorkflow()
	}

	baseToolOutput := chat.toolOutput(message)
	board := chat.workflow.Begin(message)
	chat.appendHistory(board...)

	order := chat.randomizedModels()
	if len(order) == 0 {
		return
	}

	roles := []string{"Project Manager", "Team Lead"}
	for index := 0; index < chat.workflow.DeveloperCount(); index++ {
		roles = append(roles, fmt.Sprintf("Developer %d", index+1))
	}
	roles = append(roles, "QA", "Review")

	chat.appendHistory("Team: " + strings.Join(roles, " -> "))

	transcript := chat.snapshot()
	responses := []string{}

	projectManager := order[projectManagerProviderOffset]
	projectRequest := chat.implementationRequest("Project Manager", message, baseToolOutput, "Break the request into a concise project board, capture risks, and preserve context for the rest of the team.", transcript, responses)
	line, response := chat.runStage("Project Manager ["+projectManager.Name()+"]", projectManager, projectRequest)
	transcript = append(transcript, line)
	responses = append(responses, line)
	chat.memory.RememberAgent("Project Manager", response)
	chat.memory.RememberShared("Project board prepared for: " + message)

	teamLead := order[providerIndex(teamLeadPosition, teamLeadProviderOffset, len(order))]
	developerTasks := chat.workflow.DeveloperTasks()
	developerCount := len(developerTasks)
	if developerCount == 0 {
		developerTasks = []string{"the requested implementation"}
		developerCount = 1
	}

	assignments := []string{fmt.Sprintf("Deploy %d developer(s).", developerCount)}
	for index, task := range developerTasks {
		assignments = append(assignments, chat.workflow.AssignDeveloper(index+1, task))
	}
	chat.appendHistory(assignments...)

	leadRequest := chat.implementationRequest("Team Lead", message, baseToolOutput, "Oversee the team, explain the staffing decision, and coordinate the developer assignments.\n"+strings.Join(assignments, "\n"), transcript, responses)
	line, response = chat.runStage("Team Lead ["+teamLead.Name()+"]", teamLead, leadRequest)
	transcript = append(transcript, line)
	responses = append(responses, line)
	chat.memory.RememberAgent("Team Lead", response)
	chat.memory.RememberShared(response)
	chat.appendHistory(chat.workflow.ReportProgress("Team Lead", fmt.Sprintf("assigned %d developer(s) and published the current plan", developerCount)))

	for index, task := range developerTasks {
		for _, channel := range chat.workflow.AnnounceIntent(index+1, task) {
			chat.appendHistory(channel)
		}

		developer := order[providerIndex(index, developerProviderOffset, len(order))]
		label := fmt.Sprintf("Developer %d [%s]", index+1, developer.Name())
		detail := fmt.Sprintf("Implement %s. Report progress back to the chat and mention any files or tests that need review.", task)
		developerRequest := chat.implementationRequest(fmt.Sprintf("Developer %d", index+1), message, baseToolOutput, detail, transcript, responses)
		line, response = chat.runStage(label, developer, developerRequest)
		transcript = append(transcript, line)
		responses = append(responses, line)
		chat.memory.RememberAgent(fmt.Sprintf("Developer %d", index+1), response)
		chat.memory.RememberShared(response)
		chat.appendHistory(chat.workflow.ReportProgress(fmt.Sprintf("Developer %d", index+1), "reported implementation progress to the chat"))
	}

	qaProvider := order[providerIndex(developerCount, qaProviderOffset, len(order))]
	qaRequest := chat.implementationRequest("QA", message, baseToolOutput, "Write the unit and integration test strategy, review the implementation quality, and start with `Decision: PASS` or `Decision: REWORK`.", transcript, responses)
	line, response = chat.runStage("QA ["+qaProvider.Name()+"]", qaProvider, qaRequest)
	transcript = append(transcript, line)
	responses = append(responses, line)
	chat.memory.RememberAgent("QA", response)
	chat.appendHistory(chat.workflow.ReportProgress("QA", "reviewed the implementation and test plan"))

	decision := chat.qaDecision(response)
	if decision == "REWORK" {
		chat.appendHistory(chat.workflow.RequestRework(response))
		for index, task := range developerTasks {
			developer := order[providerIndex(index, developerProviderOffset, len(order))]
			label := fmt.Sprintf("Developer %d [%s]", index+1, developer.Name())
			detail := fmt.Sprintf("Address the QA findings while updating %s. Confirm the revised intent and the tests you touched.", task)
			developerRequest := chat.implementationRequest(fmt.Sprintf("Developer %d", index+1), message, baseToolOutput, detail, transcript, responses)
			line, response = chat.runStage(label, developer, developerRequest)
			transcript = append(transcript, line)
			responses = append(responses, line)
			chat.memory.RememberAgent(fmt.Sprintf("Developer %d", index+1), response)
			chat.memory.RememberShared(response)
			chat.appendHistory(chat.workflow.ReportProgress(fmt.Sprintf("Developer %d", index+1), "completed the QA follow-up pass"))
		}

		line, response = chat.runStage("QA ["+qaProvider.Name()+"]", qaProvider, chat.implementationRequest("QA", message, baseToolOutput, "Review the updated implementation and start with `Decision: PASS` or `Decision: REWORK`.", transcript, responses))
		transcript = append(transcript, line)
		responses = append(responses, line)
		decision = chat.qaDecision(response)
		chat.memory.RememberAgent("QA", response)
	}

	chat.appendHistory(chat.workflow.SetReview(decision))

	reviewProvider := order[providerIndex(developerCount, reviewProviderOffset, len(order))]
	reviewRequest := chat.implementationRequest("Review", message, baseToolOutput, "Summarize the project board, communication channel status, memory, and review outcome. End with `Accept with :accept or :reject.`", transcript, responses)
	line, response = chat.runStage("Review ["+reviewProvider.Name()+"]", reviewProvider, reviewRequest)
	if !strings.Contains(strings.ToLower(response), "accept") {
		chat.mu.Lock()
		last := len(chat.history) - 1
		if last >= 0 {
			chat.history[last] += "\nAccept with :accept or :reject."
		}
		chat.mu.Unlock()
	}
	chat.memory.RememberAgent("Review", response)
}

func (chat *Chat) implementationRequest(role string, message string, baseToolOutput string, instructions string, transcript []string, responses []string) *provider.Request {
	return &provider.Request{
		Mode:          "IMPLEMENT",
		Message:       message,
		ToolOutput:    chat.composeImplementationContext(baseToolOutput, role, instructions),
		Transcript:    transcript,
		PriorResponse: responses,
		SystemPrompt:  chat.implementationPrompt(role, instructions),
	}
}

func (chat *Chat) implementationPrompt(role string, instructions string) string {
	return strings.TrimSpace(strings.Join([]string{
		"You are the " + role + " in a coordinated implementation team.",
		"Keep the response concrete, minimal, and aligned with the current project board.",
		"Use the communication channel and memory context before proposing overlapping changes.",
		instructions,
	}, "\n"))
}

func (chat *Chat) composeImplementationContext(baseToolOutput string, role string, instructions string) string {
	lines := []string{}
	if baseToolOutput != "" {
		lines = append(lines, baseToolOutput)
	}

	if memory := chat.memory.Snapshot(role); len(memory) > 0 {
		lines = append(lines, strings.Join(memory, "\n"))
	}

	lines = append(lines, "Role instructions: "+instructions)

	return strings.Join(lines, "\n\n")
}

func (chat *Chat) composeToolOutput(baseToolOutput string, agent string) string {
	lines := []string{}
	if baseToolOutput != "" {
		lines = append(lines, baseToolOutput)
	}

	if memory := chat.memory.Snapshot(agent); len(memory) > 0 {
		lines = append(lines, strings.Join(memory, "\n"))
	}

	return strings.Join(lines, "\n\n")
}

func (chat *Chat) runStage(label string, current provider.Provider, request *provider.Request) (string, string) {
	chat.mu.Lock()
	chat.history = append(chat.history, label+": ")
	chat.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), chat.timeout)
	response, err := current.GenerateStream(ctx, request, func(chunk string) {
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
		chat.history[last] = label + ": \033[31mError:\033[0m " + err.Error()
	}
	line := chat.history[last]
	chat.mu.Unlock()

	if chat.onStream != nil {
		chat.onStream()
	}

	return line, response
}

func (chat *Chat) appendHistory(lines ...string) {
	if len(lines) == 0 {
		return
	}

	chat.mu.Lock()
	chat.history = append(chat.history, lines...)
	chat.mu.Unlock()
}

func (chat *Chat) snapshot() []string {
	chat.mu.Lock()
	defer chat.mu.Unlock()

	return append([]string(nil), chat.history...)
}

func (chat *Chat) qaDecision(response string) string {
	upper := strings.ToUpper(response)
	if strings.Contains(upper, "DECISION: REWORK") {
		return "REWORK"
	}

	if strings.Contains(upper, "DECISION: PASS") {
		return "PASS"
	}

	if strings.Contains(upper, "REWORK") {
		return "REWORK"
	}

	return "PASS"
}

func providerIndex(position int, offset int, total int) int {
	return (position + offset) % total
}

func (chat *Chat) remember(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return "Memory store skipped: no content provided."
	}

	chat.memory.RememberShared(target)
	chat.appendHistory("System: memory stored -> " + target)

	return "Memory stored: " + target
}

func (chat *Chat) recall(target string) string {
	lines := chat.memory.Recall(target)
	if len(lines) == 0 {
		chat.appendHistory("System: memory recall returned no matches.")
		return "Memory recall: no matches."
	}

	chat.appendHistory(append([]string{"System: memory recall."}, lines...)...)

	return "Memory recall:\n" + strings.Join(lines, "\n")
}

func (chat *Chat) forget(target string) string {
	removed := chat.memory.Forget(target)
	if removed == 0 {
		chat.appendHistory("System: memory forget removed nothing.")
		return "Memory forget: no matches."
	}

	line := fmt.Sprintf("System: memory forget removed %d entrie(s).", removed)
	chat.appendHistory(line)

	return fmt.Sprintf("Memory forget removed %d entrie(s).", removed)
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
