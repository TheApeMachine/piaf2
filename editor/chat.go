package editor

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/theapemachine/piaf/provider"
	"github.com/theapemachine/piaf/team"
)

const (
	projectManagerProviderOffset = 0
	architectProviderOffset      = 1
	teamLeadPosition             = 0
	teamLeadProviderOffset       = 2
	developerProviderOffset      = 3
	qaProviderOffset             = 4
	reviewProviderOffset         = 5
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
	dumpPath     string
	scrollOffset int
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
		timeout:  180 * time.Second,
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
ChatWithTimeout sets the per-stage API timeout.
Default 180s to allow tool-heavy rounds (browse, read, recall).
*/
func ChatWithTimeout(timeout time.Duration) chatOpts {
	return func(chat *Chat) {
		if timeout > 0 {
			chat.timeout = timeout
		}
	}
}

/*
ChatWithDumpFile appends all model output to the given path.
Creates the file if missing; each runStage writes label and full response.
*/
func ChatWithDumpFile(path string) chatOpts {
	return func(chat *Chat) {
		chat.dumpPath = strings.TrimSpace(path)
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
Kanban returns the current kanban from the workflow when in IMPLEMENT mode.
*/
func (chat *Chat) Kanban() *team.Kanban {
	if chat.workflow == nil {
		return nil
	}

	return chat.workflow.Kanban()
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
	chat.scrollOffset = 0
	chat.history = append(chat.history, "", "You: "+message, "")
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

/*
ScrollUp moves the chat window back in history.
*/
func (chat *Chat) ScrollUp() {
	chat.mu.Lock()
	chat.scrollOffset++
	chat.mu.Unlock()
}

/*
ScrollDown moves the chat window forward in history.
*/
func (chat *Chat) ScrollDown() {
	chat.mu.Lock()
	if chat.scrollOffset > 0 {
		chat.scrollOffset--
	}
	chat.mu.Unlock()
}

/*
ScrollOffset returns the current scroll offset.
*/
func (chat *Chat) ScrollOffset() int {
	chat.mu.Lock()
	defer chat.mu.Unlock()
	return chat.scrollOffset
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

type chatToolBackend struct {
	chat *Chat
}

func (backend *chatToolBackend) Browse(path string) string   { return backend.chat.browse(path) }
func (backend *chatToolBackend) Read(path string) string      { return backend.chat.read(path) }
func (backend *chatToolBackend) Remember(content string) string { return backend.chat.remember(content) }
func (backend *chatToolBackend) Recall(filter string) string  { return backend.chat.recall(filter) }
func (backend *chatToolBackend) Forget(filter string) string  { return backend.chat.forget(filter) }

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
	toolBackend := &chatToolBackend{chat: chat}
	for _, current := range order {
		agentLabel := "Discussion " + current.Name()
		toolOutput := chat.composeToolOutput(baseToolOutput, agentLabel)
		systemPrompt := chat.systemPrompt
		if systemPrompt == "" {
			systemPrompt = "You are a specialized AI assistant. The root directory of the project is: " + chat.root
		}
		request := &provider.Request{
			Mode:          chat.mode,
			Message:       message,
			ToolOutput:    toolOutput,
			Transcript:    transcript,
			PriorResponse: responses,
			SystemPrompt:  systemPrompt,
			Tools:         provider.DiscussionTools(),
			ToolExecutor:  provider.NewDiscussionToolExecutor(toolBackend),
		}
		line, response := chat.runStage(current.Name(), current, request)
		transcript = append(transcript, line)
		responses = append(responses, line)
		chat.memory.RememberAgent(agentLabel, response)
	}
}

func (chat *Chat) submitImplementation(message string) {
	if chat.workflow == nil {
		chat.workflow = NewWorkflow()
	}

	chat.appendHistory("System: Securing current state via Git...")
	if err := chat.secureGitBranch(message); err != nil {
		chat.appendHistory("System: Git strategy skipped or failed: " + err.Error())
	} else {
		chat.appendHistory("System: Checked out new branch automatically.")
	}

	chat.workflow.Begin(chat.snapshot())

	baseToolOutput := chat.toolOutput(message)
	transcript := chat.snapshot()
	responses := []string{}

	order := chat.randomizedModels()
	if len(order) == 0 {
		return
	}

	projectManager := order[projectManagerProviderOffset]
	pmPrompt := "Scan the recent conversation. Extract epics and stories. Output a structured kanban using ## Epic: Title for each epic and ### Story: Title for each story. Capture risks and preserve context."
	projectRequest := chat.implementationRequest("Project Manager", message, baseToolOutput, pmPrompt, transcript, responses)
	line, response := chat.runStage("Project Manager ["+projectManager.Name()+"]", projectManager, projectRequest)
	transcript = append(transcript, line)
	responses = append(responses, line)
	chat.memory.RememberAgent("Project Manager", response)
	chat.memory.RememberShared("Project board prepared for: " + message)

	parser := team.NewKanbanParser()
	kanban := parser.Parse(response)
	if kanban != nil && len(kanban.Epics) > 0 {
		chat.workflow.SetKanban(kanban)
	}
	chat.appendHistory(chat.workflow.BoardLines()...)

	kanbanContext := chat.formatKanbanForArchitect()
	architect := order[architectProviderOffset]
	architectPrompt := "Given the kanban (epics/stories), produce an implementation plan: file-level changes, dependencies, ordering, and risks. Be concise."
	architectRequest := chat.implementationRequest("Architect", message, baseToolOutput, architectPrompt+"\n\n"+kanbanContext, transcript, responses)
	line, architectResponse := chat.runStage("Architect ["+architect.Name()+"]", architect, architectRequest)
	transcript = append(transcript, line)
	responses = append(responses, line)
	chat.memory.RememberAgent("Architect", architectResponse)
	chat.workflow.SetImplementationPlan(&team.ImplementationPlan{Raw: architectResponse})
	chat.appendHistory(chat.workflow.ReportProgress("Architect", "produced implementation plan"))

	roles := []string{"Project Manager", "Architect", "Team Lead"}
	for index := 0; index < chat.workflow.DeveloperCount(); index++ {
		roles = append(roles, fmt.Sprintf("Developer %d", index+1))
	}
	roles = append(roles, "QA", "Review")

	chat.appendHistory("Team: " + strings.Join(roles, " -> "))

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

	var wg sync.WaitGroup
	type devResult struct {
		index    int
		task     string
		line     string
		response string
	}
	resultsCh := make(chan devResult, developerCount)

	for index, task := range developerTasks {
		for _, channel := range chat.workflow.AnnounceIntent(index+1, task) {
			chat.appendHistory(channel)
		}

		wg.Add(1)
		go func(idx int, tsk string) {
			defer wg.Done()
			developer := order[providerIndex(idx, developerProviderOffset, len(order))]
			label := fmt.Sprintf("Developer %d [%s]", idx+1, developer.Name())
			detail := fmt.Sprintf("Implement %s. You may spawn sub-agents for focused tasks using the spawn_subagent tool. Use the coordination queue to announce file locks (FileLock) before editing. Report progress back to the chat and mention any files or tests that need review.", tsk)
			developerRequest := chat.implementationDeveloperRequest(fmt.Sprintf("Developer %d", idx+1), message, baseToolOutput, detail, transcript, responses, developer)
			lineOut, responseOut := chat.runStage(label, developer, developerRequest)

			resultsCh <- devResult{index: idx, task: tsk, line: lineOut, response: responseOut}
		}(index, task)
	}

	wg.Wait()
	close(resultsCh)

	var devLines []string
	var devResponses []string
	for res := range resultsCh {
		chat.memory.RememberAgent(fmt.Sprintf("Developer %d", res.index+1), res.response)
		chat.memory.RememberShared(res.response)
		chat.appendHistory(chat.workflow.ReportProgress(fmt.Sprintf("Developer %d", res.index+1), "reported implementation progress to the chat"))
		devLines = append(devLines, res.line)
		devResponses = append(devResponses, res.response)
	}
	transcript = append(transcript, devLines...)
	responses = append(responses, devResponses...)

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
		var qaWg sync.WaitGroup
		qaResults := make(chan devResult, developerCount)

		for index, task := range developerTasks {
			qaWg.Add(1)
			go func(idx int, tsk string) {
				defer qaWg.Done()
				developer := order[providerIndex(idx, developerProviderOffset, len(order))]
				label := fmt.Sprintf("Developer %d [%s]", idx+1, developer.Name())
				detail := fmt.Sprintf("Address the QA findings while updating %s. Confirm the revised intent and the tests you touched.", tsk)
				developerRequest := chat.implementationDeveloperRequest(fmt.Sprintf("Developer %d", idx+1), message, baseToolOutput, detail, transcript, responses, developer)
				lineOut, responseOut := chat.runStage(label, developer, developerRequest)
				qaResults <- devResult{index: idx, task: tsk, line: lineOut, response: responseOut}
			}(index, task)
		}

		qaWg.Wait()
		close(qaResults)

		var qaDevLines []string
		var qaDevResponses []string
		for res := range qaResults {
			chat.memory.RememberAgent(fmt.Sprintf("Developer %d", res.index+1), res.response)
			chat.memory.RememberShared(res.response)
			chat.appendHistory(chat.workflow.ReportProgress(fmt.Sprintf("Developer %d", res.index+1), "completed the QA follow-up pass"))
			qaDevLines = append(qaDevLines, res.line)
			qaDevResponses = append(qaDevResponses, res.response)
		}
		transcript = append(transcript, qaDevLines...)
		responses = append(responses, qaDevResponses...)

		line, response = chat.runStage("QA ["+qaProvider.Name()+"]", qaProvider, chat.implementationRequest("QA", message, baseToolOutput, "Review the updated implementation and start with `Decision: PASS` or `Decision: REWORK`.", transcript, responses))
		transcript = append(transcript, line)
		responses = append(responses, line)
		decision = chat.qaDecision(response)
		chat.memory.RememberAgent("QA", response)
	}

	chat.workflow.SetQAReport(response)

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

	pmSummary := order[projectManagerProviderOffset]
	pmSummaryPrompt := "Summarize the completed work for the user and the discussion. List epics/stories done, key changes, test coverage, and any risks. Be concise."
	summaryTranscript := chat.snapshot()
	summaryRequest := chat.implementationRequest("Project Manager", message, baseToolOutput, pmSummaryPrompt, summaryTranscript, responses)
	line, _ = chat.runStage("PM Summary ["+pmSummary.Name()+"]", pmSummary, summaryRequest)
	chat.appendHistory(line)
	defer func() {
		if r := recover(); r != nil {
			chat.appendHistory("---", "The team was interrupted. You may request another implementation or continue the discussion.", "---")
			return
		}
	}()
	chat.appendHistory("---", "Implementation complete. Review the summary above. Accept with :accept or :reject.", "---")

	chat.appendHistory("You: Review the implementation summary above. Share your assessment.")
	chat.submitDiscussion("Review the implementation summary above. Share your assessment.")
}

func (chat *Chat) formatKanbanForArchitect() string {
	kanban := chat.workflow.Kanban()
	if kanban == nil || len(kanban.Epics) == 0 {
		return "Kanban: (fallback board from user request)"
	}

	var lines []string
	lines = append(lines, "Kanban:")
	for _, epic := range kanban.Epics {
		lines = append(lines, "## Epic: "+epic.Title)
		for _, story := range epic.Stories {
			lines = append(lines, "### Story: "+story.Title)
		}
	}
	return strings.Join(lines, "\n")
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

func (chat *Chat) implementationDeveloperRequest(role string, message string, baseToolOutput string, instructions string, transcript []string, responses []string, devProvider provider.Provider) *provider.Request {
	queue := chat.workflow.Queue()
	queueContext := ""
	if queue != nil {
		for _, msg := range queue.Snapshot() {
			if msg.Kind == team.FileLock {
				queueContext += fmt.Sprintf("FileLock: %s by %s\n", msg.Path, msg.Agent)
			}
		}
		if queueContext != "" {
			queueContext = "Coordination queue (current locks):\n" + queueContext
		}
	}

	baseWithQueue := baseToolOutput
	if queueContext != "" {
		baseWithQueue = baseToolOutput + "\n\n" + queueContext
	}

	request := chat.implementationRequest(role, message, baseWithQueue, instructions, transcript, responses)
	request.Tools = append(provider.DiscussionTools(), provider.ToolDefinition{
		Name:        "spawn_subagent",
		Description: "Run a single sub-agent for a focused subtask. Use for isolated work that does not conflict with your main task.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"system_prompt": map[string]any{"type": "string", "description": "System prompt for the sub-agent"},
				"user_prompt":   map[string]any{"type": "string", "description": "User prompt describing the subtask"},
			},
			"required": []string{"user_prompt"},
		},
	}, provider.ToolDefinition{
		Name:        "spawn_subagents_parallel",
		Description: "Run multiple sub-agents in parallel. Provide an array of subtask objects. This is highly recommended for concurrent independent tasks.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"subagents": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"system_prompt": map[string]any{"type": "string", "description": "System config"},
							"user_prompt":   map[string]any{"type": "string", "description": "Task details"},
						},
						"required": []string{"user_prompt"},
					},
				},
			},
			"required": []string{"subagents"},
		},
	})
	runner := team.NewSubAgentRunner(chat.timeout)
	discussionExecutor := provider.NewDiscussionToolExecutor(&chatToolBackend{chat: chat})

	request.ToolExecutor = func(name string, args map[string]any) (string, error) {
		if name == "spawn_subagent" {
			userPrompt, _ := args["user_prompt"].(string)
			systemPrompt, _ := args["system_prompt"].(string)
			if userPrompt == "" {
				return "", fmt.Errorf("spawn_subagent requires user_prompt")
			}
			ctx, cancel := context.WithTimeout(context.Background(), chat.timeout)
			defer cancel()
			return runner.Run(ctx, devProvider, systemPrompt, userPrompt)
		}

		if name == "spawn_subagents_parallel" {
			subagentsList, ok := args["subagents"].([]any)
			if !ok {
				return "", fmt.Errorf("spawn_subagents_parallel requires a subagents array")
			}

			var wg sync.WaitGroup
			results := make([]string, len(subagentsList))
			errors := make([]error, len(subagentsList))

			for i, item := range subagentsList {
				config, ok := item.(map[string]any)
				if !ok {
					continue
				}

				userPrompt, _ := config["user_prompt"].(string)
				systemPrompt, _ := config["system_prompt"].(string)

				wg.Add(1)
				go func(idx int, sp, up string) {
					defer wg.Done()
					ctx, cancel := context.WithTimeout(context.Background(), chat.timeout)
					defer cancel()
					res, err := runner.Run(ctx, devProvider, sp, up)
					results[idx] = res
					errors[idx] = err
				}(i, systemPrompt, userPrompt)
			}
			wg.Wait()

			var combined strings.Builder
			for i, res := range results {
				if errors[i] != nil {
					combined.WriteString(fmt.Sprintf("Subagent %d error: %v\n", i+1, errors[i]))
				} else {
					combined.WriteString(fmt.Sprintf("Subagent %d result:\n%s\n", i+1, res))
				}
			}
			return combined.String(), nil
		}

		return discussionExecutor(name, args)
	}

	return request
}

func (chat *Chat) implementationPrompt(role string, instructions string) string {
	return strings.TrimSpace(strings.Join([]string{
		"You are the " + role + " in a coordinated implementation team.",
		"The root directory of the project is: " + chat.root,
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
	chat.history = append(chat.history, "", label+": ")
	myIndex := len(chat.history) - 1
	chat.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), chat.timeout)
	response, err := current.GenerateStream(ctx, request, func(chunk string) {
		chat.mu.Lock()
		chat.scrollOffset = 0
		if myIndex >= 0 {
			chat.history[myIndex] += chunk
			if chat.onStream != nil {
				chat.onStream()
			}
		}
		chat.mu.Unlock()
	})
	cancel()

	chat.mu.Lock()
	if err != nil {
		chat.history[myIndex] = label + ": \033[31mError:\033[0m " + err.Error()
	}
	line := chat.history[myIndex]
	chat.mu.Unlock()

	if chat.onStream != nil {
		chat.onStream()
	}

	chat.dumpStage(label, line)
	return line, response
}

func (chat *Chat) dumpStage(label string, line string) {
	if chat.dumpPath == "" {
		return
	}

	path := chat.dumpPath
	if strings.HasPrefix(path, "~/") || path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	dir := filepath.Dir(path)
	_ = os.MkdirAll(dir, 0755)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	ts := time.Now().Format("2006-01-02 15:04:05")
	block := fmt.Sprintf("\n=== %s | %s ===\n%s\n", ts, label, line)
	_, _ = f.WriteString(block)
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
	if len(lines) > 2000 {
		truncated := len(lines) - 2000
		lines = lines[:2000]
		lines = append(lines, fmt.Sprintf("... (truncated %d lines)", truncated))
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

func sanitizeBranchName(message string) string {
	msg := strings.ToLower(message)
	reg := regexp.MustCompile("[^a-z0-9]+")
	name := reg.ReplaceAllString(msg, "-")
	name = strings.Trim(name, "-")
	if len(name) > 40 {
		name = name[:40]
		name = strings.Trim(name, "-")
	}
	if name == "" {
		name = "feature-implementation"
	}
	return "feature/" + name
}

func (chat *Chat) secureGitBranch(message string) error {
	cmdAdd := exec.Command("git", "add", ".")
	cmdAdd.Dir = chat.root
	if err := cmdAdd.Run(); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	cmdStatus := exec.Command("git", "status", "--porcelain")
	cmdStatus.Dir = chat.root
	out, _ := cmdStatus.Output()
	if len(strings.TrimSpace(string(out))) > 0 {
		cmdCommit := exec.Command("git", "commit", "-m", "Auto-commit before implementation phase: "+sanitizeBranchName(message))
		cmdCommit.Dir = chat.root
		if err := cmdCommit.Run(); err != nil {
			return fmt.Errorf("git commit failed: %w", err)
		}
	}

	branchName := sanitizeBranchName(message)

	cmdBranchCheck := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branchName)
	cmdBranchCheck.Dir = chat.root
	if err := cmdBranchCheck.Run(); err == nil {
		branchName = fmt.Sprintf("%s-%d", branchName, time.Now().Unix())
	}

	cmdCheckout := exec.Command("git", "checkout", "-b", branchName)
	cmdCheckout.Dir = chat.root
	if err := cmdCheckout.Run(); err != nil {
		return fmt.Errorf("git checkout failed: %w", err)
	}

	return nil
}
