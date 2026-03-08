package tui

import (
	"io"
	"testing"

	"github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/piaf/wire"
)

func TestNewRenderer(t *testing.T) {
	convey.Convey("Given NewRenderer", t, func() {
		convey.Convey("When called", func() {
			renderer := NewRenderer()

			convey.Convey("It should return a non-nil Renderer", func() {
				convey.So(renderer, convey.ShouldNotBeNil)
			})
		})
	})
}

func TestRendererRead(t *testing.T) {
	convey.Convey("Given a Renderer", t, func() {
		renderer := NewRenderer()

		convey.Convey("When Read is called with no output buffered", func() {
			buf := make([]byte, 8)
			number, err := renderer.Read(buf)

			convey.Convey("It should return 0 and EOF", func() {
				convey.So(number, convey.ShouldEqual, 0)
				convey.So(err, convey.ShouldEqual, io.EOF)
			})
		})
	})
}

func TestRendererWrite(t *testing.T) {
	convey.Convey("Given a Renderer", t, func() {
		renderer := NewRenderer()

		convey.Convey("When Write is called with valid Frame wire format", func() {
			frame := &wire.Frame{Lines: []string{"hello"}, CursorRow: 0, CursorCol: 5, Width: 80, Height: 24}
			data, _ := io.ReadAll(frame)
			number, err := renderer.Write(data)

			convey.Convey("It should return bytes written and nil", func() {
				convey.So(err, convey.ShouldBeNil)
				convey.So(number, convey.ShouldEqual, len(data))
			})

			convey.Convey("It should produce ANSI output on Read", func() {
				buf := make([]byte, 4096)
				number, err := renderer.Read(buf)
				convey.So(err, convey.ShouldBeNil)
				convey.So(number, convey.ShouldBeGreaterThan, 0)
				convey.So(string(buf[:number]), convey.ShouldContainSubstring, "hello")
			})
		})

		convey.Convey("When Write is called with invalid data", func() {
			number, err := renderer.Write([]byte("invalid"))

			convey.Convey("It should return 0 and error", func() {
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(number, convey.ShouldEqual, 0)
			})
		})
	})
}

func TestRendererClose(t *testing.T) {
	convey.Convey("Given a Renderer", t, func() {
		renderer := NewRenderer()

		convey.Convey("When Close is called", func() {
			err := renderer.Close()

			convey.Convey("It should return nil", func() {
				convey.So(err, convey.ShouldBeNil)
			})
		})
	})
}

func BenchmarkRendererRead(b *testing.B) {
	renderer := NewRenderer()
	frame := &wire.Frame{Lines: []string{"x"}}
	data, _ := io.ReadAll(frame)
	renderer.Write(data)
	buf := make([]byte, 4096)
	for b.Loop() {
		_, _ = renderer.Read(buf)
	}
}

func BenchmarkRendererWrite(b *testing.B) {
	renderer := NewRenderer()
	frame := &wire.Frame{Lines: []string{"line one", "line two"}, CursorRow: 1, CursorCol: 4, Width: 80, Height: 24}
	data, _ := io.ReadAll(frame)
	for b.Loop() {
		_, _ = renderer.Write(data)
	}
}
