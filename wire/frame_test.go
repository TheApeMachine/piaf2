package wire

import (
	"io"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestFrameReadWriteRoundTrip(t *testing.T) {
	convey.Convey("Given a Frame", t, func() {
		convey.Convey("When Read then Write", func() {
			frame := &Frame{
				Lines:     []string{"hello", "world"},
				CursorRow: 1,
				CursorCol: 3,
				Mode:      "normal",
				Width:     80,
				Height:    24,
			}

			data, err := io.ReadAll(frame)
			convey.So(err, convey.ShouldBeNil)

			decoded := &Frame{}
			_, err = decoded.Write(data)
			convey.So(err, convey.ShouldBeNil)
			convey.So(decoded.Lines, convey.ShouldResemble, frame.Lines)
			convey.So(decoded.CursorRow, convey.ShouldEqual, frame.CursorRow)
			convey.So(decoded.CursorCol, convey.ShouldEqual, frame.CursorCol)
			convey.So(decoded.Mode, convey.ShouldEqual, frame.Mode)
			convey.So(decoded.Width, convey.ShouldEqual, frame.Width)
			convey.So(decoded.Height, convey.ShouldEqual, frame.Height)
			convey.So(decoded.CommandLine, convey.ShouldEqual, frame.CommandLine)
		})
	})
}

func TestFrameWriteShortBuffer(t *testing.T) {
	convey.Convey("Given short data", t, func() {
		convey.Convey("When Write", func() {
			frame := &Frame{}
			_, err := frame.Write([]byte{0, 0})

			convey.Convey("It should return ErrShortBuffer", func() {
				convey.So(err, convey.ShouldEqual, io.ErrShortBuffer)
			})
		})
	})
}

func BenchmarkFrameRead(b *testing.B) {
	frame := &Frame{Lines: []string{"line one", "line two"}, CursorRow: 1, CursorCol: 4, Mode: "normal", Width: 80, Height: 24}
	buf := make([]byte, 256)
	for b.Loop() {
		frame.readBuf = nil
		frame.readOffset = 0
		_, _ = frame.Read(buf)
	}
}

func BenchmarkFrameWrite(b *testing.B) {
	frame := &Frame{Lines: []string{"hello", "world"}, CursorRow: 0, CursorCol: 5, Width: 80, Height: 24}
	data, _ := io.ReadAll(frame)
	decoded := &Frame{}
	for b.Loop() {
		decoded.readBuf = nil
		decoded.readOffset = 0
		_, _ = decoded.Write(data)
	}
}
