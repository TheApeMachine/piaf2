package tui

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestNewApp(t *testing.T) {
	convey.Convey("Given NewApp", t, func() {
		convey.Convey("When called with no options", func() {
			app := NewApp()

			convey.Convey("It should return a non-nil App", func() {
				convey.So(app, convey.ShouldNotBeNil)
			})

			convey.Convey("It should have nil renderer", func() {
				convey.So(app.renderer, convey.ShouldBeNil)
			})
		})

		convey.Convey("When called with AppWithRenderer", func() {
			renderer := NewRenderer()
			app := NewApp(AppWithRenderer(renderer))

			convey.Convey("It should set the renderer", func() {
				convey.So(app.renderer, convey.ShouldEqual, renderer)
			})
		})
	})
}

func TestAppRead(t *testing.T) {
	convey.Convey("Given an App", t, func() {
		app := NewApp()

		convey.Convey("When Read is called", func() {
			buf := make([]byte, 8)
			number, err := app.Read(buf)

			convey.Convey("It should return 0 and nil", func() {
				convey.So(number, convey.ShouldEqual, 0)
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
