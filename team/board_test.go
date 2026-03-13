package team

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestKanbanBoard(t *testing.T) {
	convey.Convey("Given an empty Kanban", t, func() {
		kanban := &Kanban{}

		convey.Convey("Board returns nil", func() {
			convey.So(kanban.Board(), convey.ShouldBeNil)
		})

		convey.Convey("DeveloperTasks with maxTasks 2 returns nil", func() {
			convey.So(kanban.DeveloperTasks(2), convey.ShouldBeNil)
		})
	})

	convey.Convey("Given a Kanban with one epic and two stories", t, func() {
		kanban := &Kanban{
			Epics: []Epic{
				{
					ID:     "epic-1",
					Title:  "Auth",
					Stories: []Story{
						{ID: "story-1-1", Title: "Login", Status: StatusTodo, EpicID: "epic-1"},
						{ID: "story-1-2", Title: "Logout", Status: StatusInProgress, EpicID: "epic-1"},
					},
				},
			},
		}

		convey.Convey("Board returns Implement lines", func() {
			board := kanban.Board()
			convey.So(len(board), convey.ShouldEqual, 2)
			convey.So(board[0], convey.ShouldEqual, "Implement Login")
			convey.So(board[1], convey.ShouldEqual, "Implement Logout")
		})

		convey.Convey("DeveloperTasks returns up to maxTasks stories", func() {
			tasks := kanban.DeveloperTasks(1)
			convey.So(len(tasks), convey.ShouldEqual, 1)
			convey.So(tasks[0], convey.ShouldEqual, "Login")
		})
	})
}

func BenchmarkKanbanBoard(b *testing.B) {
	kanban := &Kanban{
		Epics: []Epic{
			{ID: "epic-1", Title: "Epic", Stories: []Story{
				{Title: "Story 1"}, {Title: "Story 2"}, {Title: "Story 3"},
			}},
		},
	}

	for index := 0; index < b.N; index++ {
		_ = kanban.Board()
	}
}
