package editor

import (
	"io"
	"testing"

	"github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/piaf/event"
)

func encodeRune(r rune) []byte {
	return event.EncodeRune(nil, r)
}

func encodeSpecial(key event.Key) []byte {
	return event.EncodeSpecial(nil, key)
}

func TestNewEditor(t *testing.T) {
	convey.Convey("Given NewEditor", t, func() {
		convey.Convey("When called with no options", func() {
			ed := NewEditor()

			convey.Convey("It should return a non-nil Editor", func() {
				convey.So(ed, convey.ShouldNotBeNil)
			})

			convey.Convey("It should start in NORMAL mode", func() {
				convey.So(ed.mode, convey.ShouldEqual, modeNormal)
			})

			convey.Convey("It should have a non-nil buffer", func() {
				convey.So(ed.buffer, convey.ShouldNotBeNil)
			})

			convey.Convey("It should produce output immediately", func() {
				buf := make([]byte, 4096)
				n, err := ed.Read(buf)
				convey.So(err, convey.ShouldBeNil)
				convey.So(n, convey.ShouldBeGreaterThan, 0)
			})
		})

		convey.Convey("When called with EditorWithSize", func() {
			ed := NewEditor(EditorWithSize(100, 30))

			convey.Convey("It should set the buffer dimensions", func() {
				convey.So(ed.buffer.width, convey.ShouldEqual, 100)
				convey.So(ed.buffer.height, convey.ShouldEqual, 30)
			})
		})
	})
}

func TestEditorInsertMode(t *testing.T) {
	convey.Convey("Given an Editor in NORMAL mode", t, func() {
		ed := NewEditor()

		convey.Convey("When 'i' is pressed", func() {
			ed.Write(encodeRune('i'))

			convey.Convey("It should switch to INSERT mode", func() {
				convey.So(ed.mode, convey.ShouldEqual, modeInsert)
			})
		})

		convey.Convey("When 'i' then text is entered", func() {
			ed.Write(append(encodeRune('i'), encodeRune('h')...))
			ed.Write(encodeRune('e'))
			ed.Write(encodeRune('l'))
			ed.Write(encodeRune('l'))
			ed.Write(encodeRune('o'))

			convey.Convey("It should insert the text into the buffer", func() {
				convey.So(string(ed.buffer.lines[0]), convey.ShouldEqual, "hello")
				convey.So(ed.buffer.cursorCol, convey.ShouldEqual, 5)
			})
		})

		convey.Convey("When ESC is received while in INSERT mode", func() {
			ed.Write(append(encodeRune('i'), encodeRune('t')...))
			ed.Write(append(encodeRune('e'), encodeRune('s')...))
			ed.Write(append(encodeRune('t'), encodeSpecial(event.KeyEsc)...))

			convey.Convey("It should return to NORMAL mode", func() {
				convey.So(ed.mode, convey.ShouldEqual, modeNormal)
			})
		})
	})
}

func TestEditorNormalModeMotions(t *testing.T) {
	convey.Convey("Given an Editor with two lines of content", t, func() {
		ed := NewEditor()
		ed.Write(append(encodeRune('i'), encodeRune('l')...))
		ed.Write(append(encodeRune('i'), encodeRune('n')...))
		ed.Write(append(encodeRune('e'), encodeRune(' ')...))
		ed.Write(append(encodeRune(' '), encodeRune('o')...))
		ed.Write(append(encodeRune('n'), encodeRune('e')...))
		ed.Write(append(encodeSpecial(event.KeyEnter), encodeRune('l')...))
		ed.Write(append(encodeRune('i'), encodeRune('n')...))
		ed.Write(append(encodeRune('e'), encodeRune(' ')...))
		ed.Write(append(encodeRune(' '), encodeRune('t')...))
		ed.Write(append(encodeRune('w'), encodeRune('o')...))
		ed.Write(encodeSpecial(event.KeyEsc))
		ed.buffer.cursorRow = 0
		ed.buffer.cursorCol = 0

		convey.Convey("When 'l' is pressed", func() {
			ed.Write(encodeRune('l'))

			convey.Convey("It should move cursor right", func() {
				convey.So(ed.buffer.cursorCol, convey.ShouldEqual, 1)
			})
		})

		convey.Convey("When 'j' then 'k' are pressed", func() {
			ed.Write(encodeRune('j'))
			ed.Write(encodeRune('k'))

			convey.Convey("It should return to the original row", func() {
				convey.So(ed.buffer.cursorRow, convey.ShouldEqual, 0)
			})
		})

		convey.Convey("When '0' is pressed", func() {
			ed.buffer.cursorCol = 4
			ed.Write(encodeRune('0'))

			convey.Convey("It should move to line start", func() {
				convey.So(ed.buffer.cursorCol, convey.ShouldEqual, 0)
			})
		})

		convey.Convey("When '$' is pressed", func() {
			ed.Write(encodeRune('$'))

			convey.Convey("It should move to line end", func() {
				convey.So(ed.buffer.cursorCol, convey.ShouldEqual, len(ed.buffer.lines[0]))
			})
		})
	})
}

func TestEditorDeleteCommands(t *testing.T) {
	convey.Convey("Given an Editor in INSERT mode with content", t, func() {
		ed := NewEditor()
		ed.Write(encodeRune('i'))
		ed.Write(encodeRune('a'))
		ed.Write(encodeRune('b'))
		ed.Write(encodeRune('c'))

		convey.Convey("When backspace is pressed", func() {
			ed.Write(encodeSpecial(event.KeyBackspace))

			convey.Convey("It should delete the character before the cursor", func() {
				convey.So(string(ed.buffer.lines[0]), convey.ShouldEqual, "ab")
			})
		})
	})

	convey.Convey("Given an Editor in NORMAL mode with content", t, func() {
		ed := NewEditor()
		ed.Write(encodeRune('i'))
		ed.Write(encodeRune('a'))
		ed.Write(encodeRune('b'))
		ed.Write(encodeRune('c'))
		ed.Write(encodeSpecial(event.KeyEsc))
		ed.buffer.cursorCol = 0

		convey.Convey("When 'x' is pressed", func() {
			ed.Write(encodeRune('x'))

			convey.Convey("It should delete the character at the cursor", func() {
				convey.So(string(ed.buffer.lines[0]), convey.ShouldEqual, "bc")
			})
		})
	})
}

func TestEditorArrowKeys(t *testing.T) {
	convey.Convey("Given an Editor with two lines", t, func() {
		ed := NewEditor()
		ed.Write(encodeRune('i'))
		ed.Write(append(encodeRune('f'), encodeRune('i')...))
		ed.Write(append(encodeRune('r'), encodeRune('s')...))
		ed.Write(append(encodeRune('t'), encodeSpecial(event.KeyEnter)...))
		ed.Write(append(encodeRune('s'), encodeRune('e')...))
		ed.Write(append(encodeRune('c'), encodeRune('o')...))
		ed.Write(append(encodeRune('n'), encodeRune('d')...))
		ed.Write(encodeSpecial(event.KeyEsc))
		ed.buffer.cursorRow = 0
		ed.buffer.cursorCol = 0

		convey.Convey("When down arrow is received", func() {
			ed.Write(encodeSpecial(event.KeyDown))

			convey.Convey("It should move cursor down", func() {
				convey.So(ed.buffer.cursorRow, convey.ShouldEqual, 1)
			})
		})

		convey.Convey("When right then left arrow are received", func() {
			ed.Write(encodeSpecial(event.KeyRight))
			ed.Write(encodeSpecial(event.KeyLeft))

			convey.Convey("It should return to the original column", func() {
				convey.So(ed.buffer.cursorCol, convey.ShouldEqual, 0)
			})
		})
	})
}

func TestEditorRead(t *testing.T) {
	convey.Convey("Given an Editor", t, func() {
		ed := NewEditor()

		convey.Convey("When Read is called after initial render", func() {
			buf := make([]byte, 4096)
			n, err := ed.Read(buf)

			convey.Convey("It should return output and nil error", func() {
				convey.So(err, convey.ShouldBeNil)
				convey.So(n, convey.ShouldBeGreaterThan, 0)
			})
		})

		convey.Convey("When Read is called after all output consumed", func() {
			buf := make([]byte, 65536)
			io.ReadFull(ed, buf)
			n, err := ed.Read(buf)

			convey.Convey("It should return 0 and EOF", func() {
				convey.So(n, convey.ShouldEqual, 0)
				convey.So(err, convey.ShouldEqual, io.EOF)
			})
		})
	})
}

func TestEditorCommandMode(t *testing.T) {
	convey.Convey("Given an Editor in NORMAL mode", t, func() {
		ed := NewEditor()

		convey.Convey("When ':' is pressed", func() {
			ed.Write(encodeRune(':'))

			convey.Convey("It should enter COMMAND mode", func() {
				convey.So(ed.mode, convey.ShouldEqual, modeCommand)
			})
		})

		convey.Convey("When ':q' and Enter are entered", func() {
			ed.Write(encodeRune(':'))
			ed.Write(encodeRune('q'))
			ed.Write(encodeSpecial(event.KeyEnter))

			convey.Convey("It should set QuitRequested", func() {
				convey.So(ed.QuitRequested(), convey.ShouldBeTrue)
			})
		})

		convey.Convey("When ':' then ESC are pressed", func() {
			ed.Write(encodeRune(':'))
			ed.Write(encodeSpecial(event.KeyEsc))

			convey.Convey("It should return to NORMAL mode", func() {
				convey.So(ed.mode, convey.ShouldEqual, modeNormal)
			})
		})
	})
}

func TestEditorClose(t *testing.T) {
	convey.Convey("Given an Editor", t, func() {
		ed := NewEditor()

		convey.Convey("When Close is called", func() {
			err := ed.Close()

			convey.Convey("It should return nil", func() {
				convey.So(err, convey.ShouldBeNil)
			})
		})
	})
}

func BenchmarkEditorWrite(b *testing.B) {
	ed := NewEditor()
	input := append(encodeRune('i'), encodeRune('h')...)
	input = append(input, encodeRune('e')...)
	input = append(input, encodeRune('l')...)
	input = append(input, encodeRune('l')...)
	input = append(input, encodeRune('o')...)
	input = append(input, encodeRune(' ')...)
	input = append(input, encodeRune('w')...)
	input = append(input, encodeRune('o')...)
	input = append(input, encodeRune('r')...)
	input = append(input, encodeRune('l')...)
	input = append(input, encodeRune('d')...)

	for b.Loop() {
		ed.buffer.lines = [][]rune{{}}
		ed.buffer.cursorRow = 0
		ed.buffer.cursorCol = 0
		ed.mode = modeNormal
		ed.Write(input)
	}
}

func BenchmarkEditorRead(b *testing.B) {
	ed := NewEditor()
	input := encodeRune('i')
	for _, r := range "hello world" {
		input = append(input, encodeRune(r)...)
	}
	ed.Write(input)
	buf := make([]byte, 4096)

	for b.Loop() {
		ed.readOff = 0
		_, _ = ed.Read(buf)
	}
}
