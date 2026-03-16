package editor

import (
	"strings"
	"testing"

	"github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/piaf/team"
)

func TestWorkflowBoardLines(t *testing.T) {
	convey.Convey("Given a Workflow with a kanban roadmap", t, func() {
		workflow := NewWorkflow(t.TempDir())
		workflow.SetKanban(&team.Kanban{
			Epics: []team.Epic{
				{
					ID:    "epic-1",
					Title: "Prompt workflow",
					Stories: []team.Story{
						{
							ID:     "story-1-1",
							Title:  "Plan implementation",
							Status: team.StatusTodo,
							EpicID: "epic-1",
							Tasks: []team.Task{
								{ID: "task-1-1-1", Title: "Create roadmap", Status: team.StatusTodo, StoryID: "story-1-1"},
								{ID: "task-1-1-2", Title: "Coordinate developers", Status: team.StatusTodo, StoryID: "story-1-1"},
							},
						},
					},
				},
			},
		})

		convey.Convey("BoardLines should expose roadmap and workflow sections", func() {
			rendered := strings.Join(workflow.BoardLines(), "\n")

			convey.So(rendered, convey.ShouldContainSubstring, "Project board:")
			convey.So(rendered, convey.ShouldContainSubstring, "Roadmap:")
			convey.So(rendered, convey.ShouldContainSubstring, "Task [Todo]: Create roadmap")
			convey.So(rendered, convey.ShouldContainSubstring, "Workflow:")
			convey.So(rendered, convey.ShouldContainSubstring, "User performs the final review")
		})

		convey.Convey("GoalAchieved should move roadmap items into review after PASS", func() {
			workflow.MarkDeveloperTaskInProgress("Plan implementation: Create roadmap")
			workflow.SetReview("PASS")
			line := workflow.GoalAchieved()
			rendered := strings.Join(workflow.BoardLines(), "\n")

			convey.So(line, convey.ShouldEqual, "Project Manager: goal achieved. All epics, stories, and tasks moved to Review.")
			convey.So(rendered, convey.ShouldContainSubstring, "Task [Review]: Create roadmap")
			convey.So(rendered, convey.ShouldContainSubstring, "QA gate: PASS, moved to Review")
		})
	})
}

func BenchmarkWorkflowBoardLines(b *testing.B) {
	workflow := NewWorkflow(b.TempDir())
	workflow.SetKanban(&team.Kanban{
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
						},
					},
				},
			},
		},
	})

	for index := 0; index < b.N; index++ {
		_ = workflow.BoardLines()
	}
}
