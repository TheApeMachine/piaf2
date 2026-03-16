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

		convey.Convey("When the same frame is rendered twice", func() {
			frame := &wire.Frame{
				Lines:     []string{"hello", "world"},
				CursorRow: 1,
				CursorCol: 2,
				Mode:      "NORMAL",
				Width:     80,
				Height:    24,
			}
			data, _ := io.ReadAll(frame)

			_, err := renderer.Write(data)
			convey.So(err, convey.ShouldBeNil)

			first := make([]byte, 4096)
			firstNumber, firstErr := renderer.Read(first)
			convey.So(firstErr, convey.ShouldBeNil)

			_, err = renderer.Write(data)
			convey.So(err, convey.ShouldBeNil)

			second := make([]byte, 4096)
			secondNumber, secondErr := renderer.Read(second)

			convey.Convey("It should emit a much smaller delta update", func() {
				convey.So(secondErr, convey.ShouldBeNil)
				convey.So(secondNumber, convey.ShouldBeLessThan, firstNumber)
				convey.So(string(second[:secondNumber]), convey.ShouldNotContainSubstring, "hello")
				convey.So(string(second[:secondNumber]), convey.ShouldNotContainSubstring, "world")
			})
		})

		convey.Convey("When a later frame removes a previously rendered line", func() {
			firstFrame := &wire.Frame{
				Lines:     []string{"hello", "world"},
				CursorRow: 0,
				CursorCol: 0,
				Mode:      "NORMAL",
				Width:     80,
				Height:    24,
			}
			secondFrame := &wire.Frame{
				Lines:     []string{"hello"},
				CursorRow: 0,
				CursorCol: 0,
				Mode:      "NORMAL",
				Width:     80,
				Height:    24,
			}

			firstData, _ := io.ReadAll(firstFrame)
			secondData, _ := io.ReadAll(secondFrame)

			_, err := renderer.Write(firstData)
			convey.So(err, convey.ShouldBeNil)

			drain := make([]byte, 4096)
			_, _ = renderer.Read(drain)

			_, err = renderer.Write(secondData)
			convey.So(err, convey.ShouldBeNil)

			buf := make([]byte, 4096)
			number, readErr := renderer.Read(buf)

			convey.Convey("It should clear the removed row without redrawing the unchanged row", func() {
				convey.So(readErr, convey.ShouldBeNil)
				convey.So(string(buf[:number]), convey.ShouldNotContainSubstring, "hello")
				convey.So(string(buf[:number]), convey.ShouldNotContainSubstring, "world")
				convey.So(string(buf[:number]), convey.ShouldContainSubstring, ansiClearLine)
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
