package team

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	kanbanEpicRe  = regexp.MustCompile(`(?m)^##\s+Epic:\s*(.+)$`)
	kanbanStoryRe = regexp.MustCompile(`(?m)^###\s+Story:\s*(.+)$`)
	kanbanTaskRe  = regexp.MustCompile(`(?m)^(?:####|[-*])\s+Task:\s*(.+)$`)
)

/*
KanbanParser extracts epics and stories from PM output.
Expects ## Epic: Title, ### Story: Title, and #### Task: Title.
*/
type KanbanParser struct{}

/*
NewKanbanParser instantiates a new KanbanParser.
*/
func NewKanbanParser() *KanbanParser {
	return &KanbanParser{}
}

/*
Parse extracts a Kanban from PM response text.
*/
func (parser *KanbanParser) Parse(text string) *Kanban {
	kanban := &Kanban{}

	lines := strings.Split(text, "\n")
	epicIdx := 0
	storyIdx := 0
	taskIdx := 0
	currentEpicIndex := -1
	currentStoryIndex := -1

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if kanbanEpicRe.MatchString(line) {
			matches := kanbanEpicRe.FindStringSubmatch(line)
			if len(matches) >= 2 {
				title := strings.TrimSpace(matches[1])
				if title == "" {
					continue
				}
				if currentEpicIndex >= 0 && len(kanban.Epics[currentEpicIndex].Stories) == 0 {
					storyIdx++
					kanban.Epics[currentEpicIndex].Stories = append(kanban.Epics[currentEpicIndex].Stories, Story{
						ID:     storyID(epicIdx, storyIdx),
						Title:  kanban.Epics[currentEpicIndex].Title,
						Status: StatusTodo,
						EpicID: kanban.Epics[currentEpicIndex].ID,
					})
				}
				epicIdx++
				storyIdx = 0
				taskIdx = 0
				id := fmt.Sprintf("epic-%d", epicIdx)
				kanban.Epics = append(kanban.Epics, Epic{ID: id, Title: title})
				currentEpicIndex = len(kanban.Epics) - 1
				currentStoryIndex = -1
			}
		} else if kanbanStoryRe.MatchString(line) {
			matches := kanbanStoryRe.FindStringSubmatch(line)
			if len(matches) >= 2 {
				title, status := parseStatusTitle(strings.TrimSpace(matches[1]))
				if title == "" {
					continue
				}
				storyIdx++
				taskIdx = 0
				sid := storyID(epicIdx+1, storyIdx)
				if currentEpicIndex >= 0 {
					sid = storyID(epicIdx, storyIdx)
				}
				story := Story{ID: sid, Title: title, Status: status}
				if currentEpicIndex >= 0 {
					story.EpicID = kanban.Epics[currentEpicIndex].ID
					kanban.Epics[currentEpicIndex].Stories = append(kanban.Epics[currentEpicIndex].Stories, story)
					currentStoryIndex = len(kanban.Epics[currentEpicIndex].Stories) - 1
				} else {
					kanban.Epics = append(kanban.Epics, Epic{
						ID:      fmt.Sprintf("epic-%d", epicIdx+1),
						Title:   title,
						Stories: []Story{story},
					})
					epicIdx++
					currentEpicIndex = len(kanban.Epics) - 1
					currentStoryIndex = 0
				}
			}
		} else if kanbanTaskRe.MatchString(line) {
			matches := kanbanTaskRe.FindStringSubmatch(line)
			if len(matches) < 2 {
				continue
			}

			title, status := parseStatusTitle(strings.TrimSpace(matches[1]))
			if title == "" {
				continue
			}

			if currentEpicIndex < 0 {
				epicIdx++
				kanban.Epics = append(kanban.Epics, Epic{
					ID:    fmt.Sprintf("epic-%d", epicIdx),
					Title: "Requested work",
				})
				currentEpicIndex = len(kanban.Epics) - 1
			}

			if currentStoryIndex < 0 {
				storyIdx++
				kanban.Epics[currentEpicIndex].Stories = append(kanban.Epics[currentEpicIndex].Stories, Story{
					ID:     storyID(epicIdx, storyIdx),
					Title:  kanban.Epics[currentEpicIndex].Title,
					Status: StatusTodo,
					EpicID: kanban.Epics[currentEpicIndex].ID,
				})
				currentStoryIndex = len(kanban.Epics[currentEpicIndex].Stories) - 1
			}

			taskIdx++
			task := Task{
				ID:      taskID(epicIdx, storyIdx, taskIdx),
				Title:   title,
				Status:  status,
				StoryID: kanban.Epics[currentEpicIndex].Stories[currentStoryIndex].ID,
			}
			kanban.Epics[currentEpicIndex].Stories[currentStoryIndex].Tasks = append(kanban.Epics[currentEpicIndex].Stories[currentStoryIndex].Tasks, task)
		}
	}

	if currentEpicIndex >= 0 && len(kanban.Epics[currentEpicIndex].Stories) == 0 {
		kanban.Epics[currentEpicIndex].Stories = append(kanban.Epics[currentEpicIndex].Stories, Story{
			ID:     storyID(epicIdx, storyIdx),
			Title:  kanban.Epics[currentEpicIndex].Title,
			Status: StatusTodo,
			EpicID: kanban.Epics[currentEpicIndex].ID,
		})
	}

	return kanban
}

func parseStatusTitle(title string) (string, StoryStatus) {
	status := StatusTodo
	if idx := strings.LastIndex(title, "("); idx >= 0 && strings.HasSuffix(title, ")") {
		paren := strings.TrimSpace(strings.TrimSuffix(title[idx+1:], ")"))
		title = strings.TrimSpace(title[:idx])
		switch strings.ToLower(paren) {
		case "done", "completed":
			status = StatusDone
		case "in progress", "inprogress":
			status = StatusInProgress
		case "backlog":
			status = StatusBacklog
		case "review", "in review":
			status = StatusReview
		}
	}

	return title, status
}
