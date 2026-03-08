package tui

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/piaf/editor"
)

func TestNewApp(t *testing.T) {
	convey.Convey("Given NewApp", t, func() {
		convey.Convey("When called with no options", func() {
			app := NewApp()

			convey.Convey("It should return a non-nil App", func() {
				convey.So(app, convey.ShouldNotBeNil)
			})

			convey.Convey("It should have a non-nil editor", func() {
				convey.So(app.editor, convey.ShouldNotBeNil)
			})
		})

		convey.Convey("When called with AppWithEditor", func() {
			ed := editor.NewEditor()
			app := NewApp(AppWithEditor(ed))

			convey.Convey("It should set the editor", func() {
				convey.So(app.editor, convey.ShouldEqual, ed)
			})
		})
	})
}

func TestAppRead(t *testing.T) {
	convey.Convey("Given an App", t, func() {
		app := NewApp()

		convey.Convey("When Read is called", func() {
			buf := make([]byte, 4096)
			number, err := app.Read(buf)

			convey.Convey("It should return rendered ANSI bytes and nil error", func() {
				convey.So(number, convey.ShouldBeGreaterThan, 0)
				convey.So(err, convey.ShouldBeNil)
			})
		})
	})
}

func TestAppWrite(t *testing.T) {
	convey.Convey("Given an App", t, func() {
		app := NewApp()

		convey.Convey("When Write is called", func() {
			number, err := app.Write([]byte("hello"))

			convey.Convey("It should return 5 and nil", func() {
				convey.So(number, convey.ShouldEqual, 5)
				convey.So(err, convey.ShouldBeNil)
			})
		})
	})
}

func TestAppClose(t *testing.T) {
	convey.Convey("Given an App", t, func() {
		app := NewApp()

		convey.Convey("When Close is called", func() {
			err := app.Close()

			convey.Convey("It should return nil", func() {
				convey.So(err, convey.ShouldBeNil)
			})
		})
	})
}

func BenchmarkAppRead(b *testing.B) {
	app := NewApp()
	buf := make([]byte, 4096)

	for b.Loop() {
		app.readOff = 0
		_, _ = app.Read(buf)
	}
}

func BenchmarkAppWrite(b *testing.B) {
	app := NewApp()
	data := []byte("benchmark data")

	for b.Loop() {
		_, _ = app.Write(data)
	}
}

