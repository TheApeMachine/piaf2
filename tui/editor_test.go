package tui

import (
	"io"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestNewEditor(t *testing.T) {
	convey.Convey("Given NewEditor", t, func() {
		convey.Convey("When called with no options", func() {
			editor := NewEditor()

			convey.Convey("It should return a non-nil Editor", func() {
				convey.So(editor, convey.ShouldNotBeNil)
			})

			convey.Convey("It should start in NORMAL mode", func() {
				convey.So(editor.mode, convey.ShouldEqual, modeNormal)
			})

			convey.Convey("It should have a non-nil buffer", func() {
				convey.So(editor.buffer, convey.ShouldNotBeNil)
			})

			convey.Convey("It should produce ANSI output immediately", func() {
				buf := make([]byte, 4096)
				number, err := editor.Read(buf)
				convey.So(err, convey.ShouldBeNil)
				convey.So(number, convey.ShouldBeGreaterThan, 0)
			})
		})

		convey.Convey("When called with EditorWithSize", func() {
			editor := NewEditor(EditorWithSize(100, 30))

			convey.Convey("It should set the buffer dimensions", func() {
				convey.So(editor.buffer.width, convey.ShouldEqual, 100)
				convey.So(editor.buffer.height, convey.ShouldEqual, 30)
			})
		})
	})
}

func TestEditorInsertMode(t *testing.T) {
	convey.Convey("Given an Editor in NORMAL mode", t, func() {
		editor := NewEditor()

		convey.Convey("When 'i' is pressed", func() {
			editor.Write([]byte("i"))

			convey.Convey("It should switch to INSERT mode", func() {
				convey.So(editor.mode, convey.ShouldEqual, modeInsert)
			})
		})

		convey.Convey("When 'i' then text is entered", func() {
			editor.Write([]byte("ihello"))

			convey.Convey("It should insert the text into the buffer", func() {
				convey.So(string(editor.buffer.lines[0]), convey.ShouldEqual, "hello")
				convey.So(editor.buffer.cursorCol, convey.ShouldEqual, 5)
			})
		})

		convey.Convey("When ESC is received while in INSERT mode", func() {
			editor.Write([]byte("itest\x1b"))

			convey.Convey("It should return to NORMAL mode", func() {
				convey.So(editor.mode, convey.ShouldEqual, modeNormal)
			})
		})
	})
}

func TestEditorNormalModeMotions(t *testing.T) {
	convey.Convey("Given an Editor with two lines of content", t, func() {
		editor := NewEditor()
		editor.Write([]byte("iline one\nline two\x1b"))
		editor.buffer.cursorRow = 0
		editor.buffer.cursorCol = 0

		convey.Convey("When 'l' is pressed", func() {
			editor.Write([]byte("l"))

			convey.Convey("It should move cursor right", func() {
				convey.So(editor.buffer.cursorCol, convey.ShouldEqual, 1)
			})
		})

		convey.Convey("When 'j' then 'k' are pressed", func() {
			editor.Write([]byte("jk"))

			convey.Convey("It should return to the original row", func() {
				convey.So(editor.buffer.cursorRow, convey.ShouldEqual, 0)
			})
		})

		convey.Convey("When '0' is pressed", func() {
			editor.buffer.cursorCol = 4
			editor.Write([]byte("0"))

			convey.Convey("It should move to line start", func() {
				convey.So(editor.buffer.cursorCol, convey.ShouldEqual, 0)
			})
		})

		convey.Convey("When '$' is pressed", func() {
			editor.Write([]byte("$"))

			convey.Convey("It should move to line end", func() {
				convey.So(editor.buffer.cursorCol, convey.ShouldEqual, len(editor.buffer.lines[0]))
			})
		})
	})
}

func TestEditorDeleteCommands(t *testing.T) {
	convey.Convey("Given an Editor in INSERT mode with content", t, func() {
		editor := NewEditor()
		editor.Write([]byte("iabc"))

		convey.Convey("When backspace is pressed", func() {
			editor.Write([]byte{0x7f})

			convey.Convey("It should delete the character before the cursor", func() {
				convey.So(string(editor.buffer.lines[0]), convey.ShouldEqual, "ab")
			})
		})
	})

	convey.Convey("Given an Editor in NORMAL mode with content", t, func() {
		editor := NewEditor()
		editor.Write([]byte("iabc\x1b"))
		editor.buffer.cursorCol = 0

		convey.Convey("When 'x' is pressed", func() {
			editor.Write([]byte("x"))

			convey.Convey("It should delete the character at the cursor", func() {
				convey.So(string(editor.buffer.lines[0]), convey.ShouldEqual, "bc")
			})
		})
	})
}

func TestEditorArrowKeys(t *testing.T) {
	convey.Convey("Given an Editor with two lines", t, func() {
		editor := NewEditor()
		editor.Write([]byte("ifirst\nsecond\x1b"))
		editor.buffer.cursorRow = 0
		editor.buffer.cursorCol = 0

		convey.Convey("When down arrow is received", func() {
			editor.Write([]byte("\x1b[B"))

			convey.Convey("It should move cursor down", func() {
				convey.So(editor.buffer.cursorRow, convey.ShouldEqual, 1)
			})
		})

		convey.Convey("When right then left arrow are received", func() {
			editor.Write([]byte("\x1b[C\x1b[D"))

			convey.Convey("It should return to the original column", func() {
				convey.So(editor.buffer.cursorCol, convey.ShouldEqual, 0)
			})
		})
	})
}

func TestEditorRead(t *testing.T) {
	convey.Convey("Given an Editor", t, func() {
		editor := NewEditor()

		convey.Convey("When Read is called after initial render", func() {
			buf := make([]byte, 4096)
			number, err := editor.Read(buf)

			convey.Convey("It should return ANSI output and nil error", func() {
				convey.So(err, convey.ShouldBeNil)
				convey.So(number, convey.ShouldBeGreaterThan, 0)
			})

			convey.Convey("It should contain the mode indicator", func() {
				convey.So(string(buf[:number]), convey.ShouldContainSubstring, "NORMAL")
			})
		})

		convey.Convey("When Read is called after all output consumed", func() {
			buf := make([]byte, 65536)
			io.ReadFull(editor, buf)
			number, err := editor.Read(buf)

			convey.Convey("It should return 0 and EOF", func() {
				convey.So(number, convey.ShouldEqual, 0)
				convey.So(err, convey.ShouldEqual, io.EOF)
			})
		})
	})
}

func TestEditorClose(t *testing.T) {
	convey.Convey("Given an Editor", t, func() {
		editor := NewEditor()

		convey.Convey("When Close is called", func() {
			err := editor.Close()

			convey.Convey("It should return nil", func() {
				convey.So(err, convey.ShouldBeNil)
			})
		})
	})
}

func BenchmarkEditorWrite(b *testing.B) {
	editor := NewEditor()
	input := []byte("ihello world")

	for b.Loop() {
		editor.buffer.lines = [][]rune{{}}
		editor.buffer.cursorRow = 0
		editor.buffer.cursorCol = 0
		editor.mode = modeNormal
		editor.Write(input)
	}
}

func BenchmarkEditorRead(b *testing.B) {
	editor := NewEditor()
	editor.Write([]byte("ihello world"))
	buf := make([]byte, 4096)

	for b.Loop() {
		editor.readOff = 0
		_, _ = editor.Read(buf)
	}
}

func BenchmarkEditorRender(b *testing.B) {
	editor := NewEditor()
	editor.Write([]byte("ihello world\nextra line"))

	for b.Loop() {
		editor.render()
	}
}
