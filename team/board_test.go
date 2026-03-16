package team

import (
	"strings"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestKanbanBoard(t *testing.T) {
	convey.Convey("Given an empty Kanban", t, func() {
		kanban := &Kanban{}

		convey.Convey("Board returns nil", func() {
			convey.So(kanban.Board(), convey.ShouldBeNil)
		})

		convey.Convey("DeveloperTasks with maxTasks 2 falls back to a generic task", func() {
			convey.So(kanban.DeveloperTasks(2), convey.ShouldResemble, []string{"Implement the requested change"})
		})
	})

	convey.Convey("Given a Kanban with one epic and two stories", t, func() {
		kanban := &Kanban{
			Epics: []Epic{
				{
					ID:    "epic-1",
					Title: "Auth",
					Stories: []Story{
						{
							ID:     "story-1-1",
							Title:  "Login",
							Status: StatusTodo,
							EpicID: "epic-1",
							Tasks: []Task{
								{ID: "task-1-1-1", Title: "Build login form", Status: StatusTodo, StoryID: "story-1-1"},
							},
						},
						{ID: "story-1-2", Title: "Logout", Status: StatusInProgress, EpicID: "epic-1"},
					},
				},
			},
		}

		convey.Convey("Board returns task-level Implement lines when available", func() {
			board := kanban.Board()
			convey.So(len(board), convey.ShouldEqual, 2)
			convey.So(board[0], convey.ShouldEqual, "Implement Login: Build login form")
			convey.So(board[1], convey.ShouldEqual, "Implement Logout")
		})

		convey.Convey("DeveloperTasks returns up to maxTasks tasks", func() {
			tasks := kanban.DeveloperTasks(2)
			convey.So(len(tasks), convey.ShouldEqual, 2)
			convey.So(tasks[0], convey.ShouldEqual, "Login: Build login form")
			convey.So(tasks[1], convey.ShouldEqual, "Logout")
		})

		convey.Convey("MoveAllToReview updates task and story statuses", func() {
			kanban.MoveAllToReview()

			convey.So(kanban.Epics[0].Stories[0].Tasks[0].Status, convey.ShouldEqual, StatusReview)
			convey.So(kanban.Epics[0].Stories[1].Status, convey.ShouldEqual, StatusReview)
		})

		convey.Convey("MarkTaskInProgress updates a matching task label", func() {
			ok := kanban.MarkTaskInProgress("Login: Build login form")

			convey.So(ok, convey.ShouldBeTrue)
			convey.So(kanban.Epics[0].Stories[0].Tasks[0].Status, convey.ShouldEqual, StatusInProgress)
		})

		convey.Convey("RoadmapLines include hierarchical statuses", func() {
			lines := kanban.RoadmapLines()

			convey.So(strings.Join(lines, "\n"), convey.ShouldContainSubstring, "Epic [InProgress]: Auth")
			convey.So(strings.Join(lines, "\n"), convey.ShouldContainSubstring, "Story [Todo]: Login")
			convey.So(strings.Join(lines, "\n"), convey.ShouldContainSubstring, "Task [Todo]: Build login form")
		})
	})

	convey.Convey("Given a Kanban with no developer-safe tasks", t, func() {
		kanban := &Kanban{
			Epics: []Epic{
				{
					ID:    "epic-1",
					Title: "Coordination",
					Stories: []Story{
						{ID: "story-1-1", Title: "Write unit and integration coverage", Status: StatusTodo, EpicID: "epic-1"},
					},
				},
			},
		}

		convey.Convey("DeveloperTasks falls back to a generic task", func() {
			tasks := kanban.DeveloperTasks(1)

			convey.So(len(tasks), convey.ShouldEqual, 1)
			convey.So(tasks[0], convey.ShouldEqual, "Implement the requested change")
		})
	})
}

func BenchmarkKanbanBoard(b *testing.B) {
	kanban := &Kanban{
		Epics: []Epic{
			{ID: "epic-1", Title: "Epic", Stories: []Story{
				{Title: "Story 1", Tasks: []Task{{Title: "Task 1"}}},
				{Title: "Story 2", Tasks: []Task{{Title: "Task 2"}}},
				{Title: "Story 3", Tasks: []Task{{Title: "Task 3"}}},
			}},
		},
	}

	for index := 0; index < b.N; index++ {
		_ = kanban.Board()
	}
}
