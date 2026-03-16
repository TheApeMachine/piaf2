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

func TestKanbanAddEpicStoryTask(t *testing.T) {
	convey.Convey("AddEpic appends an epic", t, func() {
		kanban := &Kanban{}
		kanban.AddEpic("Auth flow")
		convey.So(len(kanban.Epics), convey.ShouldEqual, 1)
		convey.So(kanban.Epics[0].Title, convey.ShouldEqual, "Auth flow")
		convey.So(kanban.Epics[0].ID, convey.ShouldEqual, "epic-1")
	})

	convey.Convey("AddStory with no epics creates General epic first", t, func() {
		kanban := &Kanban{}
		kanban.AddStory(-1, "Login")
		convey.So(len(kanban.Epics), convey.ShouldEqual, 1)
		convey.So(kanban.Epics[0].Title, convey.ShouldEqual, "General")
		convey.So(len(kanban.Epics[0].Stories), convey.ShouldEqual, 1)
		convey.So(kanban.Epics[0].Stories[0].Title, convey.ShouldEqual, "Login")
	})

	convey.Convey("Given a Kanban with one epic", t, func() {
		kanban := &Kanban{}
		kanban.AddEpic("Auth")
		kanban.AddStory(-1, "Login")
		kanban.AddStory(-1, "Logout")

		convey.Convey("Stories belong to the epic", func() {
			convey.So(len(kanban.Epics[0].Stories), convey.ShouldEqual, 2)
			convey.So(kanban.Epics[0].Stories[0].Title, convey.ShouldEqual, "Login")
			convey.So(kanban.Epics[0].Stories[1].Title, convey.ShouldEqual, "Logout")
		})

		convey.Convey("AddTask appends to last story", func() {
			kanban.AddTask(-1, -1, "Add unit tests")
			convey.So(len(kanban.Epics[0].Stories[1].Tasks), convey.ShouldEqual, 1)
			convey.So(kanban.Epics[0].Stories[1].Tasks[0].Title, convey.ShouldEqual, "Add unit tests")
		})
	})

	convey.Convey("FormatForPM serializes to PM format", t, func() {
		kanban := &Kanban{}
		kanban.AddEpic("Feature X")
		kanban.AddStory(-1, "Story A")
		kanban.AddTask(-1, -1, "Task 1")
		out := kanban.FormatForPM()
		convey.So(out, convey.ShouldContainSubstring, "## Epic: Feature X")
		convey.So(out, convey.ShouldContainSubstring, "### Story: Story A")
		convey.So(out, convey.ShouldContainSubstring, "#### Task: Task 1")
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
