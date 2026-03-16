package editor

import (
	"strings"

	"github.com/theapemachine/piaf/team"
)

/*
KanbanView renders a Kanban into display lines for a TUI.
Shows the roadmap plus split kanban columns for parallel tracking.
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

	lines := []string{
		"Roadmap",
		"-------",
	}
	lines = append(lines, view.kanban.RoadmapLines()...)
	lines = append(lines,
		"",
		"Kanban",
		"------",
		"Split view follows multiple active developer lanes at once.",
	)
	lines = append(lines, view.splitColumns()...)
	lines = append(lines, "", "---", ":board to toggle | Escape to return to chat")

	return lines
}

/*
SetKanban updates the kanban source.
*/
func (view *KanbanView) SetKanban(kanban *team.Kanban) {
	view.kanban = kanban
}

func (view *KanbanView) splitColumns() []string {
	statuses := []team.StoryStatus{
		team.StatusBacklog,
		team.StatusTodo,
		team.StatusInProgress,
		team.StatusReview,
		team.StatusDone,
	}
	columns := make([][]string, len(statuses))

	for _, epic := range view.kanban.Epics {
		for _, story := range epic.Stories {
			if len(story.Tasks) == 0 {
				index := statusColumnIndex(story.Status)
				columns[index] = append(columns[index], story.Title)
				continue
			}

			for _, task := range story.Tasks {
				index := statusColumnIndex(task.Status)
				columns[index] = append(columns[index], story.Title+": "+task.Title)
			}
		}
	}

	colWidth := view.width/len(statuses) - 1
	if colWidth < 14 {
		colWidth = 14
	}

	headers := make([]string, 0, len(statuses))
	separator := make([]string, 0, len(statuses))
	maxRows := 0
	for index, status := range statuses {
		headers = append(headers, padKanbanCell(string(status), colWidth))
		separator = append(separator, strings.Repeat("─", colWidth))
		if len(columns[index]) > maxRows {
			maxRows = len(columns[index])
		}
	}

	lines := []string{
		strings.Join(headers, " "),
		strings.Join(separator, " "),
	}

	for row := 0; row < maxRows; row++ {
		cells := make([]string, 0, len(columns))
		for _, column := range columns {
			cell := ""
			if row < len(column) {
				cell = column[row]
			}
			cells = append(cells, padKanbanCell(cell, colWidth))
		}
		lines = append(lines, strings.Join(cells, " "))
	}

	return lines
}

func statusColumnIndex(status team.StoryStatus) int {
	switch status {
	case team.StatusBacklog:
		return 0
	case team.StatusInProgress:
		return 2
	case team.StatusReview:
		return 3
	case team.StatusDone:
		return 4
	default:
		return 1
	}
}

func padKanbanCell(value string, width int) string {
	runes := []rune(value)
	if len(runes) > width {
		runes = append(runes[:width-1], '…')
	}

	if len(runes) < width {
		runes = append(runes, []rune(strings.Repeat(" ", width-len(runes)))...)
	}

	return string(runes)
}
