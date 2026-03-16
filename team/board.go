package team

import (
	"fmt"
	"strings"
)

/*
StoryStatus represents the kanban column for a story.
*/
type StoryStatus string

const (
	StatusBacklog    StoryStatus = "Backlog"
	StatusTodo       StoryStatus = "Todo"
	StatusInProgress StoryStatus = "InProgress"
	StatusDone       StoryStatus = "Done"
	StatusReview     StoryStatus = "Review"
)

/*
Task is a concrete work item under a story.
*/
type Task struct {
	ID       string
	Title    string
	Status   StoryStatus
	StoryID  string
}

/*
Story is a single work item in the kanban; may contain tasks.
*/
type Story struct {
	ID     string
	Title  string
	Status StoryStatus
	EpicID string
	Tasks  []Task
}

/*
Epic groups related stories.
*/
type Epic struct {
	ID     string
	Title  string
	Stories []Story
}

/*
Kanban holds epics and stories derived from conversation.
*/
type Kanban struct {
	Epics []Epic
}

/*
Board returns a flat list of implementation tasks for backward compatibility.
Each story becomes "Implement <title>"; epics without stories become "Implement <epic title>".
*/
func (kanban *Kanban) Board() []string {
	if kanban == nil {
		return nil
	}

	var board []string
	for _, epic := range kanban.Epics {
		if len(epic.Stories) == 0 {
			board = append(board, "Implement "+epic.Title)
			continue
		}
		for _, story := range epic.Stories {
			board = append(board, "Implement "+story.Title)
		}
	}
	return board
}

/*
DeveloperTasks returns up to maxTasks stories suitable for developer assignment.
Skips QA and review-style items.
*/
func (kanban *Kanban) DeveloperTasks(maxTasks int) []string {
	if kanban == nil || maxTasks <= 0 {
		return nil
	}

	var tasks []string
	skip := map[string]bool{
		"write unit": true, "integration coverage": true, "unit and integration": true,
		"prepare the implementation review": true, "coordinate developer": true,
	}
	for _, epic := range kanban.Epics {
		for _, story := range epic.Stories {
			lower := strings.ToLower(story.Title)
			skipThis := false
			for k := range skip {
				if strings.Contains(lower, k) {
					skipThis = true
					break
				}
			}
			if skipThis {
				continue
			}
			tasks = append(tasks, story.Title)
			if len(tasks) >= maxTasks {
				return tasks
			}
		}
		if len(epic.Stories) == 0 {
			tasks = append(tasks, epic.Title)
			if len(tasks) >= maxTasks {
				return tasks
			}
		}
	}
	return tasks
}

func storyID(epic, story int) string {
	if epic <= 0 {
		epic = 1
	}
	if story <= 0 {
		story = 1
	}
	return fmt.Sprintf("story-%d-%d", epic, story)
}

/*
AddEpic appends a new epic. Initializes kanban if nil.
*/
func (kanban *Kanban) AddEpic(title string) {
	if kanban == nil {
		return
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return
	}
	epicIdx := len(kanban.Epics) + 1
	kanban.Epics = append(kanban.Epics, Epic{
		ID:      fmt.Sprintf("epic-%d", epicIdx),
		Title:   title,
		Stories: []Story{},
	})
}

/*
AddStory appends a story to the epic at epicIdx. Use -1 for last epic. Creates epic if none exist.
*/
func (kanban *Kanban) AddStory(epicIdx int, title string) {
	if kanban == nil {
		return
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return
	}
	if len(kanban.Epics) == 0 {
		kanban.AddEpic("General")
		epicIdx = 0
	}
	if epicIdx < 0 {
		epicIdx = len(kanban.Epics) - 1
	}
	if epicIdx >= len(kanban.Epics) {
		epicIdx = len(kanban.Epics) - 1
	}
	storyIdx := len(kanban.Epics[epicIdx].Stories) + 1
	sid := storyID(epicIdx+1, storyIdx)
	story := Story{
		ID:     sid,
		Title:  title,
		Status: StatusTodo,
		EpicID: kanban.Epics[epicIdx].ID,
	}
	kanban.Epics[epicIdx].Stories = append(kanban.Epics[epicIdx].Stories, story)
}

/*
AddTask appends a task to the story at epicIdx, storyIdx. Use -1 for last. Creates story if none exist.
*/
func (kanban *Kanban) AddTask(epicIdx, storyIdx int, title string) {
	if kanban == nil {
		return
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return
	}
	if len(kanban.Epics) == 0 {
		kanban.AddEpic("General")
		sid := storyID(1, 1)
		kanban.Epics[0].Stories = append(kanban.Epics[0].Stories, Story{
			ID: sid, Title: "General", Status: StatusTodo, EpicID: kanban.Epics[0].ID,
		})
		kanban.Epics[0].Stories[0].Tasks = append(kanban.Epics[0].Stories[0].Tasks, Task{
			ID: "task-1-1-1", Title: title, Status: StatusTodo, StoryID: sid,
		})
		return
	}
	if epicIdx < 0 {
		epicIdx = len(kanban.Epics) - 1
	}
	if epicIdx >= len(kanban.Epics) {
		epicIdx = len(kanban.Epics) - 1
	}
	epic := &kanban.Epics[epicIdx]
	if len(epic.Stories) == 0 {
		epic.Stories = append(epic.Stories, Story{
			ID: storyID(epicIdx+1, 1), Title: epic.Title, Status: StatusTodo, EpicID: epic.ID,
		})
	}
	if storyIdx < 0 {
		storyIdx = len(epic.Stories) - 1
	}
	if storyIdx >= len(epic.Stories) {
		storyIdx = len(epic.Stories) - 1
	}
	story := &epic.Stories[storyIdx]
	taskIdx := len(story.Tasks) + 1
	tid := fmt.Sprintf("task-%d-%d-%d", epicIdx+1, storyIdx+1, taskIdx)
	task := Task{ID: tid, Title: title, Status: StatusTodo, StoryID: story.ID}
	story.Tasks = append(story.Tasks, task)
}

/*
FormatForPM serializes the kanban to the PM input/output format.
*/
func (kanban *Kanban) FormatForPM() string {
	if kanban == nil || len(kanban.Epics) == 0 {
		return ""
	}
	var lines []string
	for _, epic := range kanban.Epics {
		lines = append(lines, "## Epic: "+epic.Title)
		for _, story := range epic.Stories {
			lines = append(lines, "### Story: "+story.Title)
			for _, task := range story.Tasks {
				lines = append(lines, "#### Task: "+task.Title)
			}
		}
	}
	return strings.Join(lines, "\n")
}
