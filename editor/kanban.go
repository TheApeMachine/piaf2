package editor

import (
	"strings"

	"github.com/theapemachine/piaf/team"
)

const (
	roadmapHeader = "ROADMAP ─ EPICS → STORIES → TASKS"
	colBacklog    = "Backlog"
	colTodo       = "Todo"
	colInProgress = "InProgress"
	colDone       = "Done"
	colReview     = "Review"
)

/*
KanbanView renders a Kanban into display lines for a TUI.
Shows roadmap hierarchy and columns: Backlog, Todo, InProgress, Done, Review.
*/
type KanbanView struct {
	kanban *team.Kanban
	width  int
}

/*
NewKanbanView instantiates a KanbanView for the given kanban.
*/
func NewKanbanView(kanban *team.Kanban, width int) *KanbanView {
	if width <= 0 {
		width = 80
	}

	return &KanbanView{
		kanban: kanban,
		width:  width,
	}
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}

	return string(runes[:max-3]) + "..."
}

/*
Lines returns the kanban formatted for display with roadmap and column clarity.
*/
func (view *KanbanView) Lines() []string {
	if view.kanban == nil || len(view.kanban.Epics) == 0 {
		return []string{
			roadmapHeader,
			strings.Repeat("─", len(roadmapHeader)),
			"",
			"(No epics or stories yet)",
			"",
			":epic Title | :story Title | :task Title | Tab to prompt",
			"",
			":board to toggle | :refine to expand | Escape to return",
		}
	}

	colWidth := (view.width - 6) / 5
	if colWidth < 6 {
		colWidth = 6
	}

	lines := []string{
		roadmapHeader,
		strings.Repeat("─", len(roadmapHeader)),
		"",
	}

	for _, epic := range view.kanban.Epics {
		lines = append(lines, "## EPIC: "+epic.Title)
		lines = append(lines, "")

		for _, story := range epic.Stories {
			statusTag := "[" + string(story.Status) + "]"
			title := truncate(story.Title, view.width-len(statusTag)-4)
			lines = append(lines, "  ### "+statusTag+" "+title)

			for _, task := range story.Tasks {
				taskTag := "    ○"
				if task.Status == team.StatusDone || task.Status == team.StatusReview {
					taskTag = "    ✓"
				}
				taskTitle := truncate(task.Title, view.width-8)
				lines = append(lines, taskTag+" "+taskTitle)
			}
		}

		if len(epic.Stories) == 0 {
			lines = append(lines, "  (no stories)")
		}

		lines = append(lines, "")
	}

	lines = append(lines, "─", "Backlog → Todo → InProgress → Done → Review")
	lines = append(lines, ":epic/:story/:task Title | :refine | :board | :accept/:reject | Escape")

	return lines
}

/*
SetKanban updates the kanban source.
*/
func (view *KanbanView) SetKanban(kanban *team.Kanban) {
	view.kanban = kanban
}
