package editor

import (
	"github.com/theapemachine/piaf/team"
)

/*
KanbanView renders a Kanban into display lines for a TUI.
Shows columns: Backlog, Todo, InProgress, Done.
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

/*
Lines returns the kanban formatted for display.
*/
func (view *KanbanView) Lines() []string {
	if view.kanban == nil || len(view.kanban.Epics) == 0 {
		return []string{
			"Kanban",
			"------",
			"(No epics or stories yet)",
			"",
			"Run :implement and submit a task to populate the board.",
		}
	}

	lines := []string{"Kanban", "------", ""}

	colWidth := view.width/4 - 2
	if colWidth < 8 {
		colWidth = 8
	}

	for _, epic := range view.kanban.Epics {
		lines = append(lines, "## "+epic.Title)
		lines = append(lines, "")

		for _, story := range epic.Stories {
			title := story.Title
			if len(title) > colWidth*2 {
				title = title[:colWidth*2-3] + "..."
			}
			lines = append(lines, "  ["+string(story.Status)+"] "+title)
		}

		if len(epic.Stories) == 0 {
			lines = append(lines, "  (no stories)")
		}

		lines = append(lines, "")
	}

	lines = append(lines, "---", ":board to toggle | Escape to return to chat")

	return lines
}

/*
SetKanban updates the kanban source.
*/
func (view *KanbanView) SetKanban(kanban *team.Kanban) {
	view.kanban = kanban
}
