package editor

import (
	"strings"
	"testing"

	"github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/piaf/team"
)

func TestKanbanViewLines(t *testing.T) {
	convey.Convey("Given a KanbanView with roadmap tasks in multiple statuses", t, func() {
		view := NewKanbanView(&team.Kanban{
			Epics: []team.Epic{
				{
					ID:    "epic-1",
					Title: "Prompt workflow",
					Stories: []team.Story{
						{
							ID:     "story-1-1",
							Title:  "Plan workflow",
							Status: team.StatusTodo,
							EpicID: "epic-1",
							Tasks: []team.Task{
								{ID: "task-1-1-1", Title: "Create roadmap", Status: team.StatusBacklog, StoryID: "story-1-1"},
								{ID: "task-1-1-2", Title: "Assign developers", Status: team.StatusInProgress, StoryID: "story-1-1"},
								{ID: "task-1-1-3", Title: "Move to review", Status: team.StatusReview, StoryID: "story-1-1"},
								{ID: "task-1-1-4", Title: "Final sign-off", Status: team.StatusDone, StoryID: "story-1-1"},
							},
						},
					},
				},
			},
		}, 120)

		convey.Convey("Lines should render roadmap plus split kanban columns", func() {
			lines := view.Lines()
			rendered := strings.Join(lines, "\n")

			convey.So(rendered, convey.ShouldContainSubstring, "Roadmap")
			convey.So(rendered, convey.ShouldContainSubstring, "Epic [InProgress]: Prompt workflow")
			convey.So(rendered, convey.ShouldContainSubstring, "Task [Review]: Move to review")
			convey.So(rendered, convey.ShouldContainSubstring, "Backlog")
			convey.So(rendered, convey.ShouldContainSubstring, "InProgress")
			convey.So(rendered, convey.ShouldContainSubstring, "Review")
			convey.So(rendered, convey.ShouldContainSubstring, "Plan workflow: Assign developers")
			convey.So(rendered, convey.ShouldContainSubstring, "Split view follows multiple active developer lanes at once.")
		})
	})
}

func BenchmarkKanbanViewLines(b *testing.B) {
	view := NewKanbanView(&team.Kanban{
		Epics: []team.Epic{
			{
				ID:    "epic-1",
				Title: "Epic",
				Stories: []team.Story{
					{
						ID:     "story-1-1",
						Title:  "Story",
						Status: team.StatusTodo,
						EpicID: "epic-1",
						Tasks: []team.Task{
							{ID: "task-1-1-1", Title: "Task 1", Status: team.StatusTodo, StoryID: "story-1-1"},
							{ID: "task-1-1-2", Title: "Task 2", Status: team.StatusInProgress, StoryID: "story-1-1"},
						},
					},
				},
			},
		},
	}, 120)

	for index := 0; index < b.N; index++ {
		_ = view.Lines()
	}
}
