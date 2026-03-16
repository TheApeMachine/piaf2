package team

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	kanbanEpicRe  = regexp.MustCompile(`(?m)^##\s+Epic:\s*(.+)$`)
	kanbanStoryRe = regexp.MustCompile(`(?m)^###\s+Story:\s*(.+)$`)
)

/*
KanbanParser extracts epics and stories from PM output.
Expects ## Epic: Title and ### Story: Title or ### Story: Title (Status).
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
	var currentEpic *Epic
	epicIdx := 0
	storyIdx := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if kanbanEpicRe.MatchString(line) {
			matches := kanbanEpicRe.FindStringSubmatch(line)
			if len(matches) >= 2 {
				title := strings.TrimSpace(matches[1])
				if title == "" {
					continue
				}
				if currentEpic != nil && len(currentEpic.Stories) == 0 {
					currentEpic.Stories = append(currentEpic.Stories, Story{
						ID:     storyID(epicIdx, storyIdx),
						Title:  currentEpic.Title,
						Status: StatusTodo,
						EpicID: currentEpic.ID,
					})
					storyIdx++
				}
				epicIdx++
				storyIdx = 0
				id := fmt.Sprintf("epic-%d", epicIdx)
				currentEpic = &Epic{ID: id, Title: title}
				kanban.Epics = append(kanban.Epics, *currentEpic)
			}
		} else if kanbanStoryRe.MatchString(line) {
			matches := kanbanStoryRe.FindStringSubmatch(line)
			if len(matches) >= 2 {
				title := strings.TrimSpace(matches[1])
				if title == "" {
					continue
				}
				status := StatusTodo
				if idx := strings.Index(title, "("); idx > 0 {
					paren := title[idx:]
					title = strings.TrimSpace(title[:idx])
					paren = strings.Trim(paren, "()")
					switch strings.ToLower(paren) {
					case "done", "completed":
						status = StatusDone
					case "in progress", "inprogress":
						status = StatusInProgress
					case "backlog":
						status = StatusBacklog
					}
				}
				storyIdx++
				sid := storyID(epicIdx+1, storyIdx)
				if currentEpic != nil {
					sid = storyID(epicIdx, storyIdx)
				}
				story := Story{ID: sid, Title: title, Status: status}
				if currentEpic != nil {
					story.EpicID = currentEpic.ID
					currentEpic.Stories = append(currentEpic.Stories, story)
					kanban.Epics[len(kanban.Epics)-1] = *currentEpic
				} else {
					kanban.Epics = append(kanban.Epics, Epic{
						ID:      fmt.Sprintf("epic-%d", epicIdx+1),
						Title:   title,
						Stories: []Story{story},
					})
					currentEpic = &kanban.Epics[len(kanban.Epics)-1]
					epicIdx++
				}
			}
		}
	}

	if currentEpic != nil && len(currentEpic.Stories) == 0 {
		currentEpic.Stories = append(currentEpic.Stories, Story{
			ID:     storyID(epicIdx, storyIdx),
			Title:  currentEpic.Title,
			Status: StatusTodo,
			EpicID: currentEpic.ID,
		})
		if len(kanban.Epics) > 0 {
			kanban.Epics[len(kanban.Epics)-1] = *currentEpic
		}
	}

	return kanban
}
