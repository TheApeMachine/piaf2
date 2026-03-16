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
	StatusReview     StoryStatus = "Review"
	StatusDone       StoryStatus = "Done"
)

/*
Task is a single executable item inside a story.
*/
type Task struct {
	ID      string
	Title   string
	Status  StoryStatus
	StoryID string
}

/*
Story is a single work item in the kanban.
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
	ID      string
	Title   string
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
			if len(story.Tasks) > 0 {
				for _, task := range story.Tasks {
					board = append(board, "Implement "+story.Title+": "+task.Title)
				}
				continue
			}
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
			if len(story.Tasks) > 0 {
				for _, task := range story.Tasks {
					label := story.Title + ": " + task.Title
					if shouldSkipDeveloperTask(label, skip) {
						continue
					}
					tasks = append(tasks, label)
					if len(tasks) >= maxTasks {
						return tasks
					}
				}
				continue
			}
			if shouldSkipDeveloperTask(story.Title, skip) {
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

	if len(tasks) == 0 {
		return []string{"Implement the requested change"}
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

func taskID(epic, story, task int) string {
	if task <= 0 {
		task = 1
	}

	return fmt.Sprintf("task-%d-%d-%d", epic, story, task)
}

/*
RoadmapLines returns roadmap lines for transcript and kanban views.
*/
func (kanban *Kanban) RoadmapLines() []string {
	if kanban == nil {
		return nil
	}

	var lines []string
	for _, epic := range kanban.Epics {
		lines = append(lines, "- Epic ["+string(kanban.epicStatus(epic))+"]: "+epic.Title)
		if len(epic.Stories) == 0 {
			continue
		}

		for _, story := range epic.Stories {
			lines = append(lines, "  - Story ["+string(kanban.storyStatus(story))+"]: "+story.Title)
			for _, task := range story.Tasks {
				lines = append(lines, "    - Task ["+string(task.Status)+"]: "+task.Title)
			}
		}
	}

	return lines
}

/*
MarkTaskInProgress marks a developer-owned item as active.
*/
func (kanban *Kanban) MarkTaskInProgress(label string) bool {
	return kanban.markTaskStatus(label, StatusInProgress)
}

/*
MoveAllToReview moves all roadmap items to the review column.
*/
func (kanban *Kanban) MoveAllToReview() {
	if kanban == nil {
		return
	}

	for epicIndex := range kanban.Epics {
		for storyIndex := range kanban.Epics[epicIndex].Stories {
			story := &kanban.Epics[epicIndex].Stories[storyIndex]
			if len(story.Tasks) == 0 {
				story.Status = StatusReview
				continue
			}

			for taskIndex := range story.Tasks {
				story.Tasks[taskIndex].Status = StatusReview
			}
		}
	}
}

func (kanban *Kanban) markTaskStatus(label string, status StoryStatus) bool {
	if kanban == nil {
		return false
	}

	needle := strings.ToLower(strings.TrimSpace(label))
	for epicIndex := range kanban.Epics {
		for storyIndex := range kanban.Epics[epicIndex].Stories {
			story := &kanban.Epics[epicIndex].Stories[storyIndex]
			if len(story.Tasks) == 0 {
				if strings.EqualFold(story.Title, needle) || strings.EqualFold("implement "+story.Title, needle) {
					story.Status = status
					return true
				}
				continue
			}

			for taskIndex := range story.Tasks {
				task := &story.Tasks[taskIndex]
				taskLabel := strings.TrimSpace(story.Title + ": " + task.Title)
				if strings.EqualFold(taskLabel, needle) || strings.EqualFold("implement "+taskLabel, needle) {
					task.Status = status
					return true
				}
			}
		}
	}

	return false
}

func (kanban *Kanban) storyStatus(story Story) StoryStatus {
	if len(story.Tasks) == 0 {
		if story.Status == "" {
			return StatusTodo
		}
		return story.Status
	}

	statuses := make([]StoryStatus, 0, len(story.Tasks))
	for _, task := range story.Tasks {
		if task.Status == "" {
			statuses = append(statuses, StatusTodo)
			continue
		}
		statuses = append(statuses, task.Status)
	}

	return aggregateStatus(statuses)
}

func (kanban *Kanban) epicStatus(epic Epic) StoryStatus {
	if len(epic.Stories) == 0 {
		return StatusTodo
	}

	statuses := make([]StoryStatus, 0, len(epic.Stories))
	for _, story := range epic.Stories {
		statuses = append(statuses, kanban.storyStatus(story))
	}

	return aggregateStatus(statuses)
}

func aggregateStatus(statuses []StoryStatus) StoryStatus {
	if len(statuses) == 0 {
		return StatusTodo
	}

	allReview := true
	allDone := true
	seenInProgress := false
	seenTodo := false
	seenBacklog := false

	for _, status := range statuses {
		switch status {
		case StatusReview:
			allDone = false
		case StatusDone:
			allReview = false
		case StatusInProgress:
			allReview = false
			allDone = false
			seenInProgress = true
		case StatusBacklog:
			allReview = false
			allDone = false
			seenBacklog = true
		default:
			allReview = false
			allDone = false
			seenTodo = true
		}
	}

	switch {
	case allReview:
		return StatusReview
	case allDone:
		return StatusDone
	case seenInProgress:
		return StatusInProgress
	case seenTodo:
		return StatusTodo
	case seenBacklog:
		return StatusBacklog
	default:
		return StatusTodo
	}
}

func shouldSkipDeveloperTask(label string, skip map[string]bool) bool {
	lower := strings.ToLower(label)
	for item := range skip {
		if strings.Contains(lower, item) {
			return true
		}
	}

	return false
}
