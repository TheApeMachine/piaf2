package editor

import (
	"fmt"
	"strings"

	"github.com/theapemachine/piaf/team"
)

const (
	kanbanBulletOpen = "○"
	kanbanBulletDone = "✓"
	kanbanArrow      = "→"
	kanbanDash       = "─"
	kanbanDoubleDash = "═"
	kanbanDot        = "•"
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

func kanbanStatusStyle(status team.StoryStatus) string {
	switch status {
	case team.StatusBacklog:
		return styleFgGray
	case team.StatusTodo:
		return styleFgBlue
	case team.StatusInProgress:
		return styleFgYellow
	case team.StatusDone:
		return styleFgGreen
	case team.StatusReview:
		return styleFgMagenta
	default:
		return styleFgGray
	}
}

func kanbanStatusIcon(status team.StoryStatus) string {
	switch status {
	case team.StatusBacklog:
		return "░"
	case team.StatusTodo:
		return "▪"
	case team.StatusInProgress:
		return "▶"
	case team.StatusDone:
		return "✓"
	case team.StatusReview:
		return "◆"
	default:
		return "▪"
	}
}

func kanbanStyledBadge(status team.StoryStatus) string {
	icon := kanbanStatusIcon(status)
	color := kanbanStatusStyle(status)
	label := string(status)

	return color + styleBold + " " + icon + " " + label + " " + styleReset
}

func kanbanProgressBar(counts [5]int, total int, barWidth int) string {
	if total == 0 || barWidth < 10 {
		return ""
	}

	var out strings.Builder
	out.Grow(barWidth * 8)

	statuses := [5]team.StoryStatus{
		team.StatusBacklog, team.StatusTodo, team.StatusInProgress,
		team.StatusDone, team.StatusReview,
	}

	out.WriteString(" ")

	for statusIndex, count := range counts {
		segWidth := (count * (barWidth - 2)) / total
		if count > 0 && segWidth == 0 {
			segWidth = 1
		}

		color := kanbanStatusStyle(statuses[statusIndex])
		out.WriteString(color)
		out.WriteString(strings.Repeat("█", segWidth))
	}

	out.WriteString(styleReset)
	out.WriteString(" ")

	return out.String()
}

func kanbanCountStories(kanban *team.Kanban) (counts [5]int, total int) {
	for _, epic := range kanban.Epics {
		for _, story := range epic.Stories {
			total++

			switch story.Status {
			case team.StatusBacklog:
				counts[0]++
			case team.StatusTodo:
				counts[1]++
			case team.StatusInProgress:
				counts[2]++
			case team.StatusDone:
				counts[3]++
			case team.StatusReview:
				counts[4]++
			}
		}
	}

	return counts, total
}

/*
Lines returns the kanban formatted for display with roadmap and column clarity.
*/
func (view *KanbanView) Lines() []string {
	if view.kanban == nil || len(view.kanban.Epics) == 0 {
		return kanbanEmptyLines(view.width)
	}

	lines := kanbanHeader(view.width)
	counts, total := kanbanCountStories(view.kanban)

	if total > 0 {
		lines = append(lines, kanbanSummaryLine(counts, total))
		lines = append(lines, kanbanProgressBar(counts, total, view.width-4))
		lines = append(lines, "")
	}

	for epicIndex, epic := range view.kanban.Epics {
		lines = append(lines, kanbanEpicLine(epic.Title, epicIndex+1, len(view.kanban.Epics)))
		lines = append(lines, kanbanThinSeparator(view.width/2))

		for _, story := range epic.Stories {
			lines = append(lines, kanbanStoryLine(story, view.width))

			for _, task := range story.Tasks {
				lines = append(lines, kanbanTaskLine(task, view.width))
			}
		}

		if len(epic.Stories) == 0 {
			lines = append(lines, styleDim+"    (no stories)"+styleReset)
		}

		lines = append(lines, "")
	}

	lines = append(lines, kanbanColumnFooter(view.width))
	lines = append(lines, kanbanHelpLine())

	return lines
}

/*
SetKanban updates the kanban source.
*/
func (view *KanbanView) SetKanban(kanban *team.Kanban) {
	view.kanban = kanban
}

func kanbanEmptyLines(width int) []string {
	header := kanbanHeader(width)

	return append(header,
		"",
		styleDim+"  (No epics or stories yet)"+styleReset,
		"",
		styleFgBrand()+styleBold+"  :epic"+styleReset+styleDim+" Title"+styleReset+
			styleFgGray+" │ "+styleReset+
			styleFgBrand()+styleBold+":story"+styleReset+styleDim+" Title"+styleReset+
			styleFgGray+" │ "+styleReset+
			styleFgBrand()+styleBold+":task"+styleReset+styleDim+" Title"+styleReset,
		"",
		styleDim+"  :board "+styleFgGray+"toggle"+styleReset+styleDim+" │ :refine "+styleFgGray+"expand"+styleReset+styleDim+" │ Esc "+styleFgGray+"return"+styleReset,
	)
}

func kanbanHeader(width int) []string {
	title := styleBold + styleFgBrand() + " ROADMAP" + styleReset +
		styleFgGray + " " + kanbanDash + " " + styleReset +
		styleFgHighlight() + "EPICS" + styleReset +
		styleDim + " " + kanbanArrow + " " + styleReset +
		styleFgHighlight() + "STORIES" + styleReset +
		styleDim + " " + kanbanArrow + " " + styleReset +
		styleFgHighlight() + "TASKS" + styleReset

	sepWidth := width - 2
	if sepWidth < 10 {
		sepWidth = 10
	}

	separator := " " + styleFgBrand() + styleDim + strings.Repeat(kanbanDoubleDash, sepWidth) + styleReset

	return []string{title, separator, ""}
}

func kanbanThinSeparator(width int) string {
	if width < 4 {
		width = 4
	}

	return "    " + styleDim + styleFgGray + strings.Repeat(kanbanDash, width) + styleReset
}

func kanbanEpicLine(title string, index, total int) string {
	counter := styleDim + styleFgGray + fmt.Sprintf("[%d/%d]", index, total) + styleReset

	return " " + styleBold + styleFgBrand() + "▌" + styleReset + " " +
		styleBold + styleFgHighlight() + title + styleReset +
		" " + counter
}

func kanbanStoryLine(story team.Story, width int) string {
	badge := kanbanStyledBadge(story.Status)
	maxTitle := width - 20
	if maxTitle < 10 {
		maxTitle = 10
	}

	title := truncate(story.Title, maxTitle)

	return "    " + badge + " " + title
}

func kanbanTaskLine(task team.Task, width int) string {
	maxTitle := width - 12
	if maxTitle < 10 {
		maxTitle = 10
	}

	title := truncate(task.Title, maxTitle)

	if task.Status == team.StatusDone || task.Status == team.StatusReview {
		return "      " + styleFgGreen + styleBold + kanbanBulletDone + styleReset +
			" " + styleDim + title + styleReset
	}

	return "      " + styleDim + kanbanBulletOpen + styleReset + " " + title
}

func kanbanSummaryLine(counts [5]int, total int) string {
	statuses := [5]struct {
		label string
		style string
	}{
		{"Backlog", styleFgGray},
		{"Todo", styleFgBlue},
		{"Active", styleFgYellow},
		{"Done", styleFgGreen},
		{"Review", styleFgMagenta},
	}

	var parts []string

	for statusIndex, status := range statuses {
		if counts[statusIndex] > 0 {
			parts = append(parts, fmt.Sprintf("%s%s%d %s%s",
				status.style, styleBold, counts[statusIndex], status.label, styleReset))
		}
	}

	return " " + styleDim + styleFgGray + kanbanDot + styleReset + " " + strings.Join(parts, styleFgGray+" │ "+styleReset)
}

func kanbanColumnFooter(width int) string {
	columns := []struct {
		label string
		style string
	}{
		{"Backlog", styleFgGray},
		{"Todo", styleFgBlue},
		{"InProgress", styleFgYellow},
		{"Done", styleFgGreen},
		{"Review", styleFgMagenta},
	}

	var parts []string

	for _, col := range columns {
		parts = append(parts, col.style+styleBold+col.label+styleReset)
	}

	arrow := styleDim + " " + kanbanArrow + " " + styleReset

	return " " + strings.Join(parts, arrow)
}

func kanbanHelpLine() string {
	return styleDim + " " +
		styleFgBrand() + ":epic" + styleReset + styleDim + "/" +
		styleFgBrand() + ":story" + styleReset + styleDim + "/" +
		styleFgBrand() + ":task" + styleReset + styleDim + " Title │ " +
		styleFgHighlight() + ":refine" + styleReset + styleDim + " │ " +
		styleFgHighlight() + ":board" + styleReset + styleDim + " │ " +
		styleFgHighlight() + ":accept" + styleReset + styleDim + "/" +
		styleFgHighlight() + ":reject" + styleReset + styleDim + " │ Esc" + styleReset
}
