package team

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestKanbanParserParse(t *testing.T) {
	convey.Convey("Given PM output with epics and stories", t, func() {
		parser := NewKanbanParser()
		text := `## Epic: Authentication
### Story: Login
### Story: Logout (InProgress)
## Epic: API
### Story: Add endpoint
`

		convey.Convey("Parse extracts kanban structure", func() {
			kanban := parser.Parse(text)
			convey.So(kanban, convey.ShouldNotBeNil)
			convey.So(len(kanban.Epics), convey.ShouldEqual, 2)
			convey.So(kanban.Epics[0].Title, convey.ShouldEqual, "Authentication")
			convey.So(len(kanban.Epics[0].Stories), convey.ShouldEqual, 2)
			convey.So(kanban.Epics[0].Stories[0].Title, convey.ShouldEqual, "Login")
			convey.So(kanban.Epics[0].Stories[1].Status, convey.ShouldEqual, StatusInProgress)
			convey.So(kanban.Epics[1].Title, convey.ShouldEqual, "API")
			convey.So(len(kanban.Epics[1].Stories), convey.ShouldEqual, 1)
		})
	})

	convey.Convey("Given empty text", t, func() {
		parser := NewKanbanParser()
		kanban := parser.Parse("")

		convey.Convey("Parse returns kanban with no epics", func() {
			convey.So(kanban, convey.ShouldNotBeNil)
			convey.So(len(kanban.Epics), convey.ShouldEqual, 0)
		})
	})
}

func BenchmarkKanbanParserParse(b *testing.B) {
	parser := NewKanbanParser()
	text := `## Epic: Auth
### Story: Login
### Story: Logout
## Epic: API
### Story: Endpoint
`

	for index := 0; index < b.N; index++ {
		_ = parser.Parse(text)
	}
}
