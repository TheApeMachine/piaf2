package editor

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestNewBuffer(t *testing.T) {
	convey.Convey("Given NewBuffer", t, func() {
		convey.Convey("When called with no options", func() {
			buf := NewBuffer()

			convey.Convey("It should return a non-nil Buffer", func() {
				convey.So(buf, convey.ShouldNotBeNil)
			})

			convey.Convey("It should have one empty line", func() {
				convey.So(len(buf.lines), convey.ShouldEqual, 1)
				convey.So(len(buf.lines[0]), convey.ShouldEqual, 0)
			})

			convey.Convey("It should have default dimensions 80x24", func() {
				convey.So(buf.width, convey.ShouldEqual, 80)
				convey.So(buf.height, convey.ShouldEqual, 24)
			})
		})

		convey.Convey("When called with BufferWithSize", func() {
			buf := NewBuffer(BufferWithSize(120, 40))

			convey.Convey("It should set the dimensions", func() {
				convey.So(buf.width, convey.ShouldEqual, 120)
				convey.So(buf.height, convey.ShouldEqual, 40)
			})
		})
	})
}

func TestBufferInsertRune(t *testing.T) {
	convey.Convey("Given a Buffer", t, func() {
		buf := NewBuffer()

		convey.Convey("When a rune is inserted", func() {
			buf.InsertRune('A')

			convey.Convey("It should appear in the line", func() {
				convey.So(string(buf.lines[0]), convey.ShouldEqual, "A")
			})

			convey.Convey("It should advance the cursor", func() {
				convey.So(buf.cursorCol, convey.ShouldEqual, 1)
			})
		})

		convey.Convey("When multiple runes are inserted", func() {
			buf.InsertRune('h')
			buf.InsertRune('i')

			convey.Convey("It should build the word", func() {
				convey.So(string(buf.lines[0]), convey.ShouldEqual, "hi")
				convey.So(buf.cursorCol, convey.ShouldEqual, 2)
			})
		})
	})
}

func TestBufferDeleteBefore(t *testing.T) {
	convey.Convey("Given a Buffer with content", t, func() {
		buf := NewBuffer()
		buf.InsertRune('a')
		buf.InsertRune('b')
		buf.InsertRune('c')

		convey.Convey("When DeleteBefore is called", func() {
			buf.DeleteBefore()

			convey.Convey("It should remove the character before the cursor", func() {
				convey.So(string(buf.lines[0]), convey.ShouldEqual, "ab")
				convey.So(buf.cursorCol, convey.ShouldEqual, 2)
			})
		})

		convey.Convey("When DeleteBefore is called at column 0", func() {
			buf.Newline()
			buf.InsertRune('d')
			buf.MoveLineStart()
			buf.DeleteBefore()

			convey.Convey("It should merge with the previous line", func() {
				convey.So(len(buf.lines), convey.ShouldEqual, 1)
				convey.So(string(buf.lines[0]), convey.ShouldEqual, "abcd")
			})
		})
	})
}

func TestBufferDeleteAt(t *testing.T) {
	convey.Convey("Given a Buffer with content", t, func() {
		buf := NewBuffer()
		buf.InsertRune('a')
		buf.InsertRune('b')
		buf.InsertRune('c')
		buf.MoveLineStart()

		convey.Convey("When DeleteAt is called at column 0", func() {
			buf.DeleteAt()

			convey.Convey("It should remove the character at the cursor", func() {
				convey.So(string(buf.lines[0]), convey.ShouldEqual, "bc")
				convey.So(buf.cursorCol, convey.ShouldEqual, 0)
			})
		})

		convey.Convey("When DeleteAt is called at end of line", func() {
			buf.MoveLineEnd()
			buf.Newline()
			buf.InsertRune('d')
			buf.cursorRow = 0
			buf.cursorCol = 3
			buf.DeleteAt()

			convey.Convey("It should merge the next line", func() {
				convey.So(len(buf.lines), convey.ShouldEqual, 1)
				convey.So(string(buf.lines[0]), convey.ShouldEqual, "abcd")
			})
		})
	})
}

func TestBufferNewline(t *testing.T) {
	convey.Convey("Given a Buffer with content", t, func() {
		buf := NewBuffer()
		buf.InsertRune('a')
		buf.InsertRune('b')

		convey.Convey("When Newline is called in the middle", func() {
			buf.MoveLineStart()
			buf.MoveRight()
			buf.Newline()

			convey.Convey("It should split the line", func() {
				convey.So(len(buf.lines), convey.ShouldEqual, 2)
				convey.So(string(buf.lines[0]), convey.ShouldEqual, "a")
				convey.So(string(buf.lines[1]), convey.ShouldEqual, "b")
				convey.So(buf.cursorRow, convey.ShouldEqual, 1)
				convey.So(buf.cursorCol, convey.ShouldEqual, 0)
			})
		})
	})
}

func TestBufferMovement(t *testing.T) {
	convey.Convey("Given a Buffer with two lines", t, func() {
		buf := NewBuffer()
		buf.InsertRune('a')
		buf.InsertRune('b')
		buf.Newline()
		buf.InsertRune('c')

		convey.Convey("When MoveUp is called", func() {
			buf.MoveUp()

			convey.Convey("It should move to the previous row", func() {
				convey.So(buf.cursorRow, convey.ShouldEqual, 0)
			})
		})

		convey.Convey("When MoveDown is called from row 0", func() {
			buf.cursorRow = 0
			buf.MoveDown()

			convey.Convey("It should move to the next row", func() {
				convey.So(buf.cursorRow, convey.ShouldEqual, 1)
			})
		})

		convey.Convey("When MoveLeft wraps", func() {
			buf.cursorRow = 1
			buf.cursorCol = 0
			buf.MoveLeft()

			convey.Convey("It should wrap to the end of the previous line", func() {
				convey.So(buf.cursorRow, convey.ShouldEqual, 0)
				convey.So(buf.cursorCol, convey.ShouldEqual, 2)
			})
		})

		convey.Convey("When MoveRight wraps", func() {
			buf.cursorRow = 0
			buf.cursorCol = 2
			buf.MoveRight()

			convey.Convey("It should wrap to the start of the next line", func() {
				convey.So(buf.cursorRow, convey.ShouldEqual, 1)
				convey.So(buf.cursorCol, convey.ShouldEqual, 0)
			})
		})

		convey.Convey("When MoveLineStart is called", func() {
			buf.cursorCol = 1
			buf.MoveLineStart()

			convey.Convey("It should move to column 0", func() {
				convey.So(buf.cursorCol, convey.ShouldEqual, 0)
			})
		})

		convey.Convey("When MoveLineEnd is called on row 0", func() {
			buf.cursorRow = 0
			buf.MoveLineEnd()

			convey.Convey("It should move to end of line", func() {
				convey.So(buf.cursorCol, convey.ShouldEqual, 2)
			})
		})
	})
}

func TestBufferWrite(t *testing.T) {
	convey.Convey("Given a Buffer", t, func() {
		buf := NewBuffer()

		convey.Convey("When Write inserts text", func() {
			n, err := buf.Write([]byte("hi"))

			convey.Convey("It should return bytes written and nil", func() {
				convey.So(n, convey.ShouldEqual, 2)
				convey.So(err, convey.ShouldBeNil)
			})

			convey.Convey("It should have the content", func() {
				convey.So(string(buf.lines[0]), convey.ShouldEqual, "hi")
			})
		})
	})
}

func TestBufferClose(t *testing.T) {
	convey.Convey("Given a Buffer", t, func() {
		buf := NewBuffer()

		convey.Convey("When Close is called", func() {
			err := buf.Close()

			convey.Convey("It should return nil", func() {
				convey.So(err, convey.ShouldBeNil)
			})
		})
	})
}

func BenchmarkBufferInsertRune(b *testing.B) {
	buf := NewBuffer()

	for b.Loop() {
		buf.InsertRune('x')
	}
}

func BenchmarkBufferWrite(b *testing.B) {
	buf := NewBuffer()
	input := []byte("hello world")

	for b.Loop() {
		buf.lines = [][]rune{{}}
		buf.cursorRow = 0
		buf.cursorCol = 0
		_, _ = buf.Write(input)
	}
}
