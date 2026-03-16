package editor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/theapemachine/piaf/team"
)

var workflowMessageReplacer = strings.NewReplacer("\n", ",", ";", ",", " and ", ",")

const maxAssignedDevelopers = 2

/*
Workflow tracks the implementation board, communication channel, and progress.
*/
type Workflow struct {
	mu             sync.Mutex
	root           string
	kanban         *team.Kanban
	plan           *team.ImplementationPlan
	queue          *team.Queue
	qaReport       string
	board          []string
	developerTasks []string
	channels       []string
	progress       []string
	review         string
}

/*
NewWorkflow instantiates a new Workflow and attempts to load an existing saved state.
*/
func NewWorkflow(root string) *Workflow {
	w := &Workflow{root: root}
	w.LoadKanban()
	return w
}

/*
Begin resets workflow state for a new implementation run.
Accepts conversation history for PM to scan; the last user message is used as fallback when kanban is empty.
*/
func (workflow *Workflow) Begin(history []string) {
	workflow.mu.Lock()
	defer workflow.mu.Unlock()

	// Intentionally omitting kanban reset so previously loaded persistent board is active.
	workflow.plan = nil
	workflow.qaReport = ""
	workflow.queue = team.NewQueue(64)
	workflow.channels = nil
	workflow.progress = nil
	workflow.review = ""

	message := lastUserMessage(history)
	
	if workflow.kanban == nil {
		workflow.board = workflowBoard(message)
		workflow.developerTasks = workflowDeveloperTasks(workflow.board)
	}
}

func (workflow *Workflow) SaveKanban() {
	if workflow.kanban == nil || workflow.root == "" {
		return
	}
	
	path := filepath.Join(workflow.root, ".piaf", "kanban.json")
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	
	b, err := json.MarshalIndent(workflow.kanban, "", "  ")
	if err == nil {
		_ = os.WriteFile(path, b, 0644)
	}
}

func (workflow *Workflow) LoadKanban() {
	if workflow.root == "" {
		return
	}
	
	path := filepath.Join(workflow.root, ".piaf", "kanban.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	
	var kanban team.Kanban
	if err := json.Unmarshal(b, &kanban); err == nil {
		workflow.kanban = &kanban
		workflow.board = kanban.Board()
		workflow.developerTasks = kanban.DeveloperTasks(maxAssignedDevelopers)
	}
}

/*
BoardLines returns the project board as display lines for the transcript.
*/
func (workflow *Workflow) BoardLines() []string {
	workflow.mu.Lock()
	defer workflow.mu.Unlock()

	lines := []string{"Project board:"}
	for _, item := range workflow.board {
		lines = append(lines, "- [ ] "+item)
	}
	return lines
}

/*
AddEpic appends a new epic, creating the kanban if needed. Saves and refreshes board.
*/
func (workflow *Workflow) AddEpic(title string) {
	workflow.mu.Lock()
	defer workflow.mu.Unlock()

	if workflow.kanban == nil {
		workflow.kanban = &team.Kanban{}
	}

	workflow.kanban.AddEpic(title)
	workflow.board = workflow.kanban.Board()
	workflow.developerTasks = workflow.kanban.DeveloperTasks(maxAssignedDevelopers)
	workflow.SaveKanban()
}

/*
AddStory appends a story to the last epic. Creates General epic if none exist. Saves and refreshes board.
*/
func (workflow *Workflow) AddStory(title string) {
	workflow.mu.Lock()
	defer workflow.mu.Unlock()

	if workflow.kanban == nil {
		workflow.kanban = &team.Kanban{}
	}

	workflow.kanban.AddStory(-1, title)
	workflow.board = workflow.kanban.Board()
	workflow.developerTasks = workflow.kanban.DeveloperTasks(maxAssignedDevelopers)
	workflow.SaveKanban()
}

/*
AddTask appends a task to the last story. Creates General epic/story if none exist. Saves and refreshes board.
*/
func (workflow *Workflow) AddTask(title string) {
	workflow.mu.Lock()
	defer workflow.mu.Unlock()

	if workflow.kanban == nil {
		workflow.kanban = &team.Kanban{}
	}

	workflow.kanban.AddTask(-1, -1, title)
	workflow.board = workflow.kanban.Board()
	workflow.developerTasks = workflow.kanban.DeveloperTasks(maxAssignedDevelopers)
	workflow.SaveKanban()
}

/*
SetKanban stores the PM-derived kanban and updates board and developer tasks.
*/
func (workflow *Workflow) SetKanban(kanban *team.Kanban) {
	workflow.mu.Lock()
	defer workflow.mu.Unlock()

	if kanban == nil {
		return
	}

	workflow.kanban = kanban
	workflow.board = kanban.Board()

	maxDevs := maxAssignedDevelopers
	if workflow.plan != nil && workflow.plan.DeveloperCount > 0 {
		maxDevs = workflow.plan.DeveloperCount
	}

	workflow.developerTasks = kanban.DeveloperTasks(maxDevs)
	workflow.SaveKanban()
	workflow.board = append(workflow.board,
		"Coordinate developer intents and unblock overlapping changes",
		"Write unit and integration coverage",
		"Prepare the implementation review for :accept or :reject",
	)
}

/*
Kanban returns the current kanban for Architect and Developers.
*/
func (workflow *Workflow) Kanban() *team.Kanban {
	workflow.mu.Lock()
	defer workflow.mu.Unlock()

	return workflow.kanban
}

/*
ImplementationPlan holds the Architect-produced plan.
*/
func (workflow *Workflow) ImplementationPlan() *team.ImplementationPlan {
	workflow.mu.Lock()
	defer workflow.mu.Unlock()

	return workflow.plan
}

/*
SetImplementationPlan stores the Architect-produced plan and refreshes developer tasks
using plan.DeveloperCount when set (Architect-driven parallelizability).
*/
func (workflow *Workflow) SetImplementationPlan(plan *team.ImplementationPlan) {
	workflow.mu.Lock()
	defer workflow.mu.Unlock()

	workflow.plan = plan

	if workflow.kanban == nil {
		return
	}

	maxDevs := maxAssignedDevelopers
	if plan != nil && plan.DeveloperCount > 0 {
		maxDevs = plan.DeveloperCount
	}

	workflow.developerTasks = workflow.kanban.DeveloperTasks(maxDevs)
}

/*
SetQAReport stores the QA completion report for PM summary.
*/
func (workflow *Workflow) SetQAReport(report string) {
	workflow.mu.Lock()
	defer workflow.mu.Unlock()

	workflow.qaReport = report
}

/*
QAReport returns the stored QA report.
*/
func (workflow *Workflow) QAReport() string {
	workflow.mu.Lock()
	defer workflow.mu.Unlock()

	return workflow.qaReport
}

/*
Queue returns the coordination queue for Developers and sub-agents.
*/
func (workflow *Workflow) Queue() *team.Queue {
	workflow.mu.Lock()
	defer workflow.mu.Unlock()

	return workflow.queue
}

func lastUserMessage(history []string) string {
	for index := len(history) - 1; index >= 0; index-- {
		line := strings.TrimSpace(history[index])
		if strings.HasPrefix(line, "You: ") {
			return strings.TrimPrefix(line, "You: ")
		}
	}
	return ""
}

/*
DeveloperTasks returns the current developer assignments.
*/
func (workflow *Workflow) DeveloperTasks() []string {
	workflow.mu.Lock()
	defer workflow.mu.Unlock()

	return append([]string(nil), workflow.developerTasks...)
}

/*
DeveloperCount returns how many developers the team lead should assign.
*/
func (workflow *Workflow) DeveloperCount() int {
	workflow.mu.Lock()
	defer workflow.mu.Unlock()

	return len(workflow.developerTasks)
}

/*
AssignDeveloper records a team lead assignment line.
*/
func (workflow *Workflow) AssignDeveloper(index int, task string) string {
	return fmt.Sprintf("Assignment: Developer %d owns %s.", index, task)
}

/*
AnnounceIntent records a communication channel message for a developer change.
*/
func (workflow *Workflow) AnnounceIntent(index int, task string) []string {
	workflow.mu.Lock()
	defer workflow.mu.Unlock()

	intent := fmt.Sprintf("Channel coordination: Developer %d intends to change %s.", index, task)
	confirm := fmt.Sprintf("Channel coordination: Team Lead confirms Developer %d is clear to proceed without blocking the rest of the team.", index)
	workflow.channels = append(workflow.channels, intent, confirm)

	return []string{intent, confirm}
}

/*
ReportProgress records an agent status update.
*/
func (workflow *Workflow) ReportProgress(agent string, status string) string {
	workflow.mu.Lock()
	defer workflow.mu.Unlock()

	status = strings.TrimSpace(strings.TrimSuffix(status, "."))
	line := fmt.Sprintf("Progress: %s %s.", agent, status)
	workflow.progress = append(workflow.progress, line)

	return line
}

/*
RequestRework records that QA requested another implementation pass.
*/
func (workflow *Workflow) RequestRework(summary string) string {
	workflow.mu.Lock()
	defer workflow.mu.Unlock()

	workflow.review = "REWORK"

	return fmt.Sprintf("Review: QA requested improvements. %s", strings.TrimSpace(summary))
}

/*
SetReview records the final QA decision.
*/
func (workflow *Workflow) SetReview(decision string) string {
	workflow.mu.Lock()
	defer workflow.mu.Unlock()

	workflow.review = strings.ToUpper(strings.TrimSpace(decision))
	if workflow.review == "" {
		workflow.review = "PASS"
	}

	return "Review: QA final decision " + workflow.review + "."
}

func workflowBoard(message string) []string {
	parts := splitWorkflowMessage(message)
	board := make([]string, 0, len(parts)+3)

	for _, part := range parts {
		board = append(board, "Implement "+part)
	}

	board = append(board,
		"Coordinate developer intents and unblock overlapping changes",
		"Write unit and integration coverage",
		"Prepare the implementation review for :accept or :reject",
	)

	return board
}

func workflowDeveloperTasks(board []string) []string {
	tasks := make([]string, 0, maxAssignedDevelopers)
	for _, item := range board {
		lower := strings.ToLower(item)
		if strings.Contains(lower, "write unit and integration coverage") {
			continue
		}

		if strings.Contains(lower, "prepare the implementation review") {
			continue
		}

		tasks = append(tasks, item)
		if len(tasks) == maxAssignedDevelopers {
			break
		}
	}

	if len(tasks) == 0 {
		return []string{"Implement the requested change"}
	}

	return tasks
}

func splitWorkflowMessage(message string) []string {
	parts := strings.Split(workflowMessageReplacer.Replace(strings.TrimSpace(message)), ",")
	seen := map[string]struct{}{}
	tasks := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.TrimSuffix(part, ".")
		if part == "" {
			continue
		}

		key := strings.ToLower(part)
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		tasks = append(tasks, part)
	}

	if len(tasks) == 0 {
		return []string{"the requested implementation work"}
	}

	return tasks
}

/*
AgentMemory stores shared and per-agent memory entries.
*/
type AgentMemory struct {
	mu     sync.Mutex
	root   string
	shared []string
	agents map[string][]string
}

/*
NewAgentMemory instantiates a new AgentMemory, attempting to load from disk.
*/
func NewAgentMemory(root string) *AgentMemory {
	m := &AgentMemory{
		root:   root,
		agents: map[string][]string{},
	}
	m.Load()
	return m
}

type memoryState struct {
	Shared []string            `json:"shared"`
	Agents map[string][]string `json:"agents"`
}

func (memory *AgentMemory) Save() {
	if memory.root == "" {
		return
	}
	
	path := filepath.Join(memory.root, ".piaf", "memory.json")
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	
	state := memoryState{
		Shared: memory.shared,
		Agents: memory.agents,
	}
	
	b, err := json.MarshalIndent(state, "", "  ")
	if err == nil {
		_ = os.WriteFile(path, b, 0644)
	}
}

func (memory *AgentMemory) Load() {
	if memory.root == "" {
		return
	}
	
	path := filepath.Join(memory.root, ".piaf", "memory.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	
	var state memoryState
	if err := json.Unmarshal(b, &state); err == nil {
		memory.shared = state.Shared
		if state.Agents != nil {
			memory.agents = state.Agents
		}
	}
}

/*
RememberShared stores an item in the shared team memory.
*/
func (memory *AgentMemory) RememberShared(entry string) {
	memory.mu.Lock()
	defer memory.mu.Unlock()

	entry = strings.TrimSpace(entry)
	if entry == "" {
		return
	}

	for _, current := range memory.shared {
		if current == entry {
			return
		}
	}

	memory.shared = append(memory.shared, entry)
	memory.Save()
}

/*
RememberAgent stores an item in a specific agent memory.
*/
func (memory *AgentMemory) RememberAgent(agent string, entry string) {
	memory.mu.Lock()
	defer memory.mu.Unlock()

	agent = strings.TrimSpace(agent)
	entry = strings.TrimSpace(entry)
	if agent == "" || entry == "" {
		return
	}

	current := memory.agents[agent]
	for _, line := range current {
		if line == entry {
			return
		}
	}

	memory.agents[agent] = append(current, entry)
	memory.Save()
}

/*
Recall returns matching shared and agent memories.
*/
func (memory *AgentMemory) Recall(filter string) []string {
	memory.mu.Lock()
	defer memory.mu.Unlock()

	filter = strings.ToLower(strings.TrimSpace(filter))
	lines := make([]string, 0, len(memory.shared)+len(memory.agents))

	for _, entry := range memory.shared {
		if filter == "" || strings.Contains(strings.ToLower(entry), filter) {
			lines = append(lines, "Shared: "+entry)
		}
	}

	for agent, entries := range memory.agents {
		for _, entry := range entries {
			if filter == "" || strings.Contains(strings.ToLower(entry), filter) {
				lines = append(lines, agent+": "+entry)
			}
		}
	}

	return lines
}

/*
Forget removes matching shared and agent memories.
*/
func (memory *AgentMemory) Forget(filter string) int {
	memory.mu.Lock()
	defer memory.mu.Unlock()

	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return 0
	}

	removed := 0

	nextShared := make([]string, 0, len(memory.shared))
	for _, entry := range memory.shared {
		if strings.Contains(strings.ToLower(entry), filter) {
			removed++
			continue
		}

		nextShared = append(nextShared, entry)
	}
	memory.shared = nextShared

	for agent, entries := range memory.agents {
		nextEntries := make([]string, 0, len(entries))
		for _, entry := range entries {
			if strings.Contains(strings.ToLower(entry), filter) {
				removed++
				continue
			}

			nextEntries = append(nextEntries, entry)
		}

		if len(nextEntries) == 0 {
			delete(memory.agents, agent)
			continue
		}

		memory.agents[agent] = nextEntries
	}

	return removed
}

/*
Snapshot returns the shared and agent-specific context for a stage.
*/
func (memory *AgentMemory) Snapshot(agent string) []string {
	memory.mu.Lock()
	defer memory.mu.Unlock()

	lines := make([]string, 0, len(memory.shared)+len(memory.agents[agent])+2)
	if len(memory.shared) > 0 {
		lines = append(lines, "Shared memory:")
		for _, entry := range memory.shared {
			lines = append(lines, "- "+entry)
		}
	}

	if entries := memory.agents[agent]; len(entries) > 0 {
		lines = append(lines, agent+" memory:")
		for _, entry := range entries {
			lines = append(lines, "- "+entry)
		}
	}

	return lines
}
