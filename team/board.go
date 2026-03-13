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
)

/*
Story is a single work item in the kanban.
*/
type Story struct {
	ID     string
	Title  string
	Status StoryStatus
	EpicID string
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
