package editor

import (
	"strings"
	"testing"

	"github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/piaf/team"
)

func TestKanbanViewLinesEmpty(t *testing.T) {
	convey.Convey("Given a KanbanView with nil kanban", t, func() {
		view := NewKanbanView(nil, 80)

		convey.Convey("It should return styled empty-state lines", func() {
			lines := view.Lines()
			convey.So(len(lines), convey.ShouldBeGreaterThan, 3)
			joined := strings.Join(lines, "\n")
			convey.So(joined, convey.ShouldContainSubstring, "ROADMAP")
			convey.So(joined, convey.ShouldContainSubstring, "No epics")
			convey.So(joined, convey.ShouldContainSubstring, ":epic")
		})
	})

	convey.Convey("Given a KanbanView with empty kanban", t, func() {
		view := NewKanbanView(&team.Kanban{}, 80)

		convey.Convey("It should return styled empty-state lines", func() {
			lines := view.Lines()
			joined := strings.Join(lines, "\n")
			convey.So(joined, convey.ShouldContainSubstring, "No epics")
		})
	})
}

func TestKanbanViewLinesPopulated(t *testing.T) {
	convey.Convey("Given a KanbanView with epics and stories", t, func() {
		kanban := &team.Kanban{
			Epics: []team.Epic{
				{
					ID:    "epic-1",
					Title: "Authentication",
					Stories: []team.Story{
						{ID: "s1", Title: "Login flow", Status: team.StatusTodo, EpicID: "epic-1"},
						{ID: "s2", Title: "Session management", Status: team.StatusInProgress, EpicID: "epic-1",
							Tasks: []team.Task{
								{ID: "t1", Title: "Setup JWT", Status: team.StatusDone, StoryID: "s2"},
								{ID: "t2", Title: "Add refresh tokens", Status: team.StatusTodo, StoryID: "s2"},
							},
						},
					},
				},
				{
					ID:    "epic-2",
					Title: "API",
					Stories: []team.Story{
						{ID: "s3", Title: "REST endpoints", Status: team.StatusDone, EpicID: "epic-2"},
					},
				},
			},
		}
		view := NewKanbanView(kanban, 100)

		convey.Convey("It should produce styled header lines", func() {
			lines := view.Lines()
			convey.So(len(lines), convey.ShouldBeGreaterThan, 5)
			convey.So(lines[0], convey.ShouldContainSubstring, "ROADMAP")
			convey.So(lines[0], convey.ShouldContainSubstring, "EPICS")
		})

		convey.Convey("It should contain epic titles", func() {
			lines := view.Lines()
			joined := strings.Join(lines, "\n")
			convey.So(joined, convey.ShouldContainSubstring, "Authentication")
			convey.So(joined, convey.ShouldContainSubstring, "API")
		})

		convey.Convey("It should contain styled status badges", func() {
			lines := view.Lines()
			joined := strings.Join(lines, "\n")
			convey.So(joined, convey.ShouldContainSubstring, "Todo")
			convey.So(joined, convey.ShouldContainSubstring, "InProgress")
			convey.So(joined, convey.ShouldContainSubstring, "Done")
		})

		convey.Convey("It should contain task items with check/circle markers", func() {
			lines := view.Lines()
			joined := strings.Join(lines, "\n")
			convey.So(joined, convey.ShouldContainSubstring, kanbanBulletDone)
			convey.So(joined, convey.ShouldContainSubstring, kanbanBulletOpen)
			convey.So(joined, convey.ShouldContainSubstring, "Setup JWT")
			convey.So(joined, convey.ShouldContainSubstring, "Add refresh tokens")
		})

		convey.Convey("It should contain progress summary", func() {
			lines := view.Lines()
			joined := strings.Join(lines, "\n")
			convey.So(joined, convey.ShouldContainSubstring, "█")
		})

		convey.Convey("It should contain column footer", func() {
			lines := view.Lines()
			joined := strings.Join(lines, "\n")
			convey.So(joined, convey.ShouldContainSubstring, "Backlog")
			convey.So(joined, convey.ShouldContainSubstring, "InProgress")
		})

		convey.Convey("It should contain help line", func() {
			lines := view.Lines()
			lastLine := lines[len(lines)-1]
			convey.So(lastLine, convey.ShouldContainSubstring, ":epic")
			convey.So(lastLine, convey.ShouldContainSubstring, ":refine")
			convey.So(lastLine, convey.ShouldContainSubstring, "Esc")
		})
	})
}

func TestKanbanViewSetKanban(t *testing.T) {
	convey.Convey("Given a KanbanView", t, func() {
		view := NewKanbanView(nil, 80)

		convey.Convey("It should update kanban via SetKanban", func() {
			kanban := &team.Kanban{
				Epics: []team.Epic{
					{ID: "e1", Title: "New Epic", Stories: []team.Story{}},
				},
			}
			view.SetKanban(kanban)
			lines := view.Lines()
			joined := strings.Join(lines, "\n")
			convey.So(joined, convey.ShouldContainSubstring, "New Epic")
		})
	})
}

func TestKanbanViewNarrowWidth(t *testing.T) {
	convey.Convey("Given a KanbanView with very narrow width", t, func() {
		kanban := &team.Kanban{
			Epics: []team.Epic{
				{ID: "e1", Title: "Epic", Stories: []team.Story{
					{ID: "s1", Title: "A very long story title that should be truncated", Status: team.StatusTodo},
				}},
			},
		}
		view := NewKanbanView(kanban, 30)

		convey.Convey("It should not panic and should truncate", func() {
			lines := view.Lines()
			convey.So(len(lines), convey.ShouldBeGreaterThan, 0)
		})
	})
}

func TestKanbanCountStories(t *testing.T) {
	convey.Convey("Given a kanban with mixed statuses", t, func() {
		kanban := &team.Kanban{
			Epics: []team.Epic{
				{ID: "e1", Title: "E1", Stories: []team.Story{
					{Status: team.StatusTodo},
					{Status: team.StatusInProgress},
					{Status: team.StatusDone},
					{Status: team.StatusDone},
					{Status: team.StatusBacklog},
				}},
			},
		}

		convey.Convey("It should count each status correctly", func() {
			counts, total := kanbanCountStories(kanban)
			convey.So(total, convey.ShouldEqual, 5)
			convey.So(counts[0], convey.ShouldEqual, 1)
			convey.So(counts[1], convey.ShouldEqual, 1)
			convey.So(counts[2], convey.ShouldEqual, 1)
			convey.So(counts[3], convey.ShouldEqual, 2)
			convey.So(counts[4], convey.ShouldEqual, 0)
		})
	})
}

func TestKanbanProgressBar(t *testing.T) {
	convey.Convey("Given story counts", t, func() {
		counts := [5]int{1, 2, 1, 3, 0}

		convey.Convey("It should produce a progress bar with block chars", func() {
			bar := kanbanProgressBar(counts, 7, 40)
			convey.So(bar, convey.ShouldContainSubstring, "█")
		})

		convey.Convey("It should return empty for zero total", func() {
			bar := kanbanProgressBar([5]int{}, 0, 40)
			convey.So(bar, convey.ShouldBeEmpty)
		})
	})
}

func BenchmarkKanbanViewLines(b *testing.B) {
	kanban := &team.Kanban{
		Epics: []team.Epic{
			{ID: "e1", Title: "Auth", Stories: []team.Story{
				{ID: "s1", Title: "Login", Status: team.StatusTodo, Tasks: []team.Task{
					{ID: "t1", Title: "OAuth", Status: team.StatusTodo},
					{ID: "t2", Title: "Form", Status: team.StatusDone},
				}},
				{ID: "s2", Title: "Sessions", Status: team.StatusInProgress},
			}},
			{ID: "e2", Title: "API", Stories: []team.Story{
				{ID: "s3", Title: "Endpoints", Status: team.StatusDone},
			}},
		},
	}
	view := NewKanbanView(kanban, 120)

	for index := 0; index < b.N; index++ {
		_ = view.Lines()
	}
}

func BenchmarkKanbanProgressBar(b *testing.B) {
	counts := [5]int{2, 5, 3, 8, 1}

	for index := 0; index < b.N; index++ {
		_ = kanbanProgressBar(counts, 19, 80)
	}
}
