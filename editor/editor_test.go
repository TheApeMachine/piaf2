package editor

import (
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/piaf/event"
	"github.com/theapemachine/piaf/wire"
)

func encodeRune(r rune) []byte {
	return event.EncodeRune(nil, r)
}

func encodeSpecial(key event.Key) []byte {
	return event.EncodeSpecial(nil, key)
}

/*
decodeFrame reads and deserializes a wire.Frame from the Editor's output.
It returns nil if reading or decoding fails.
*/
func decodeFrame(ed *Editor) *wire.Frame {
	data, err := io.ReadAll(ed)

	if err != nil {
		return nil
	}

	frame := &wire.Frame{}
	if _, err := frame.Write(data); err != nil {
		return nil
	}

	return frame
}

func jumpTargetCodeForWord(ed *Editor, word string) string {
	for _, target := range ed.jumpTargets {
		if target.word == word {
			return target.code
		}
	}

	return ""
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

func TestEditorJumpMode(t *testing.T) {
	convey.Convey("Given an Editor with visible code words in a workspace", t, func() {
		tempDir := t.TempDir()
		visibleWords := make([]string, 0, len(jumpAlphabet)+3)

		visibleWords = append(visibleWords, "alpha")

		for index := 0; index < len(jumpAlphabet)+2; index++ {
			visibleWords = append(visibleWords, "word"+string(rune('a'+index/10))+string(rune('a'+index%10)))
		}

		mainPath := filepath.Join(tempDir, "main.go")
		freqPath := filepath.Join(tempDir, "freq.go")

		os.WriteFile(mainPath, []byte(strings.Join(visibleWords, " ")), 0o644)
		os.WriteFile(freqPath, []byte(strings.Repeat("alpha ", 200)+strings.Repeat("wordaa ", 5)), 0o644)

		ed := NewEditor(EditorWithSize(400, 4), EditorWithPath(mainPath))
		ed.buffer.cursorRow = 0
		ed.buffer.cursorCol = 0

		convey.Convey("When 'f' is pressed", func() {
			ed.Write(encodeRune('f'))
			frame := decodeFrame(ed)
			alphaCode := jumpTargetCodeForWord(ed, "alpha")
			lastWord := visibleWords[len(visibleWords)-1]
			lastCode := jumpTargetCodeForWord(ed, lastWord)

			convey.Convey("It should label visible word starts and rank frequent words first", func() {
				convey.So(frame, convey.ShouldNotBeNil)
				convey.So(ed.jumpActive(), convey.ShouldBeTrue)
				convey.So(len(ed.jumpTargets), convey.ShouldEqual, len(visibleWords))
				convey.So(frame.CommandLine, convey.ShouldContainSubstring, "word")
				convey.So(frame.Lines[0], convey.ShouldContainSubstring, ansiInverse)
				convey.So(alphaCode, convey.ShouldNotBeBlank)
				convey.So(lastCode, convey.ShouldNotBeBlank)
				convey.So(len(alphaCode), convey.ShouldEqual, 1)
				convey.So(len(lastCode), convey.ShouldBeGreaterThan, len(alphaCode))
			})
		})

		convey.Convey("When 'f' then the most common word code are pressed", func() {
			ed.Write(encodeRune('f'))
			for _, r := range jumpTargetCodeForWord(ed, "alpha") {
				ed.Write(encodeRune(r))
			}

			convey.Convey("It should jump to that word start and exit jump mode", func() {
				convey.So(ed.buffer.cursorRow, convey.ShouldEqual, 0)
				convey.So(ed.buffer.cursorCol, convey.ShouldEqual, 0)
				convey.So(ed.jumpActive(), convey.ShouldBeFalse)
			})
		})

		convey.Convey("When 'f' then a less common word code are pressed", func() {
			targetWord := visibleWords[len(visibleWords)-1]
			targetCol := strings.Index(strings.Join(visibleWords, " "), targetWord)
			ed.Write(encodeRune('f'))
			frame := decodeFrame(ed)

			for _, r := range jumpTargetCodeForWord(ed, targetWord)[:1] {
				ed.Write(encodeRune(r))
			}

			refined := decodeFrame(ed)

			for _, r := range jumpTargetCodeForWord(ed, targetWord)[1:] {
				ed.Write(encodeRune(r))
			}

			convey.Convey("It should refine the overlay before jumping to the target word", func() {
				convey.So(frame, convey.ShouldNotBeNil)
				convey.So(refined, convey.ShouldNotBeNil)
				convey.So(ed.buffer.cursorRow, convey.ShouldEqual, 0)
				convey.So(ed.buffer.cursorCol, convey.ShouldEqual, targetCol)
				convey.So(ed.jumpActive(), convey.ShouldBeFalse)
				convey.So(refined.CommandLine, convey.ShouldContainSubstring, "f ")
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

func TestEditorRenderSyntaxHighlighting(t *testing.T) {
	convey.Convey("Given an Editor opened on a Go file", t, func() {
		file, err := os.CreateTemp("", "piaf-highlight-*.go")
		convey.So(err, convey.ShouldBeNil)
		defer os.Remove(file.Name())
		_, err = file.WriteString("package main\nfunc main() {\n\treturn 42 // answer\n}\n")
		convey.So(err, convey.ShouldBeNil)
		convey.So(file.Close(), convey.ShouldBeNil)

		ed := NewEditor(EditorWithPath(file.Name()), EditorWithSize(80, 10))

		convey.Convey("When the initial frame is rendered", func() {
			frame := decodeFrame(ed)

			convey.Convey("It should include syntax highlighting for the buffer", func() {
				convey.So(frame, convey.ShouldNotBeNil)
				convey.So(frame.Lines[0], convey.ShouldContainSubstring, styleBold+styleFgMagenta+"package"+styleReset)
				convey.So(frame.Lines[2], convey.ShouldContainSubstring, styleFgYellow+"42"+styleReset)
				convey.So(frame.Lines[2], convey.ShouldContainSubstring, styleDim+styleFgGray+"// answer"+styleReset)
			})
		})

		convey.Convey("When jump mode is activated", func() {
			ed = NewEditor(EditorWithPath(file.Name()), EditorWithSize(80, 10))
			ed.Write(encodeRune('f'))
			frame := decodeFrame(ed)

			convey.Convey("It should keep the code readable while prompting for a target", func() {
				convey.So(frame, convey.ShouldNotBeNil)
				convey.So(frame.CommandLine, convey.ShouldContainSubstring, "target")
				convey.So(frame.Lines[1], convey.ShouldContainSubstring, styleBold+styleFgMagenta+"func"+styleReset)
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

			convey.Convey("It should set quit in the wire Frame", func() {
				frameBytes, _ := io.ReadAll(ed)
				frame := &wire.Frame{}
				frame.Write(frameBytes)
				convey.So(frame.Quit, convey.ShouldBeTrue)
			})
		})

		convey.Convey("When ':' then ESC are pressed", func() {
			ed.Write(encodeRune(':'))
			ed.Write(encodeSpecial(event.KeyEsc))

			convey.Convey("It should return to NORMAL mode", func() {
				convey.So(ed.mode, convey.ShouldEqual, modeNormal)
			})
		})

		convey.Convey("When ':chat' and Enter are entered", func() {
			ed.Write(encodeRune(':'))
			ed.Write(encodeRune('c'))
			ed.Write(encodeRune('h'))
			ed.Write(encodeRune('a'))
			ed.Write(encodeRune('t'))
			ed.Write(encodeSpecial(event.KeyEnter))

			convey.Convey("It should enter the chat window", func() {
				convey.So(ed.inChat, convey.ShouldBeTrue)
				convey.So(ed.chat, convey.ShouldNotBeNil)
				convey.So(ed.chat.Mode(), convey.ShouldEqual, "CHAT")
			})
		})

		convey.Convey("When ':implement' and Enter are entered", func() {
			ed.Write(encodeRune(':'))
			for _, r := range "implement" {
				ed.Write(encodeRune(r))
			}
			ed.Write(encodeSpecial(event.KeyEnter))

			convey.Convey("It should enter implementation mode", func() {
				convey.So(ed.inChat, convey.ShouldBeTrue)
				convey.So(ed.chat, convey.ShouldNotBeNil)
				convey.So(ed.chat.Mode(), convey.ShouldEqual, "IMPLEMENT")
			})
		})
		convey.Convey("When '/' is pressed", func() {
			ed.Write(encodeRune('/'))

			convey.Convey("It should open the universal palette", func() {
				convey.So(ed.inPalette, convey.ShouldBeTrue)
				convey.So(ed.palette, convey.ShouldNotBeNil)
				convey.So(ed.palette.Query(), convey.ShouldEqual, "")
			})
		})
	})
}

func TestEditorPalette(t *testing.T) {
	convey.Convey("Given an Editor", t, func() {
		ed := NewEditor(EditorWithPath("."))

		convey.Convey("When '/' is pressed", func() {
			ed.Write(encodeRune('/'))
			frame := decodeFrame(ed)

			convey.Convey("It should show the palette with commands", func() {
				convey.So(ed.inPalette, convey.ShouldBeTrue)
				convey.So(frame, convey.ShouldNotBeNil)
				convey.So(frame.Mode, convey.ShouldEqual, "PALETTE")
				convey.So(len(frame.Lines), convey.ShouldBeGreaterThan, 0)
			})
		})

		convey.Convey("When '/' then 'chat' then Enter are entered", func() {
			ed.Write(encodeRune('/'))
			for _, r := range "chat" {
				ed.Write(encodeRune(r))
			}
			ed.Write(encodeSpecial(event.KeyEnter))

			convey.Convey("It should execute the selected command", func() {
				convey.So(ed.inPalette, convey.ShouldBeFalse)
				convey.So(ed.inChat, convey.ShouldBeTrue)
			})
		})

		convey.Convey("When '/' then Esc are entered", func() {
			ed.Write(encodeRune('/'))
			ed.Write(encodeSpecial(event.KeyEsc))

			convey.Convey("It should close the palette", func() {
				convey.So(ed.inPalette, convey.ShouldBeFalse)
				convey.So(ed.palette, convey.ShouldBeNil)
			})
		})
	})
}

func TestEditorChatFlow(t *testing.T) {
	convey.Convey("Given an Editor in the chat window", t, func() {
		submitDone := make(chan struct{})
		ed := NewEditor(EditorWithSize(80, 12))
		ed.chat = NewChat(
			ChatWithRandom(rand.New(rand.NewSource(7))),
			ChatWithProviders(
				&stubProvider{name: "OpenAI GPT-5.4", responses: []string{"first response"}},
				&stubProvider{name: "Claude Open 4.6", responses: []string{"second response"}},
				&stubProvider{name: "Gemini Pro 3.1", responses: []string{"third response"}},
			),
			ChatWithOnComplete(func() { close(submitDone) }),
		)
		ed.openChat("CHAT")

		convey.Convey("When a message is submitted", func() {
			ed.Write(encodeRune('i'))
			for _, r := range "browse ." {
				ed.Write(encodeRune(r))
			}
			ed.Write(encodeSpecial(event.KeyEnter))

			<-submitDone

			convey.Convey("It should keep the transcript in chat history", func() {
				transcript := strings.Join(ed.chat.Lines(), "\n")
				convey.So(ed.mode, convey.ShouldEqual, modeNormal)
				convey.So(transcript, convey.ShouldContainSubstring, "You: browse .")
				convey.So(transcript, convey.ShouldContainSubstring, "Pipeline:")
				convey.So(transcript, convey.ShouldContainSubstring, "first response")
			})
		})

		convey.Convey("When ':q' is executed in chat", func() {
			ed.Write(encodeRune(':'))
			ed.Write(encodeRune('q'))
			ed.Write(encodeSpecial(event.KeyEnter))

			convey.Convey("It should close chat without quitting the editor", func() {
				convey.So(ed.inChat, convey.ShouldBeFalse)
				frameBytes, _ := io.ReadAll(ed)
				frame := &wire.Frame{}
				frame.Write(frameBytes)
				convey.So(frame.Quit, convey.ShouldBeFalse)
			})
		})
	})
}

func TestEditorImplementAccept(t *testing.T) {
	convey.Convey("Given an Editor in implementation mode", t, func() {
		submitDone := make(chan struct{})
		ed := NewEditor(EditorWithSize(80, 12))
		ed.chat = NewChat(
			ChatWithRandom(rand.New(rand.NewSource(11))),
			ChatWithProviders(
				&stubProvider{name: "OpenAI GPT-5.4", responses: []string{"scoped the request"}},
				&stubProvider{name: "Claude Open 4.6", responses: []string{"prepared the diff"}},
				&stubProvider{name: "Gemini Pro 3.1", responses: []string{"final implementation summary"}},
			),
			ChatWithOnComplete(func() { close(submitDone) }),
		)
		ed.openChat("IMPLEMENT")

		convey.Convey("When a prompt is submitted and accepted", func() {
			ed.Write(encodeRune('i'))
			for _, r := range "add tests" {
				ed.Write(encodeRune(r))
			}
			ed.Write(encodeSpecial(event.KeyEnter))
			<-submitDone
			ed.Write(encodeRune(':'))
			for _, r := range "accept" {
				ed.Write(encodeRune(r))
			}
			ed.Write(encodeSpecial(event.KeyEnter))

			convey.Convey("It should record the review result", func() {
				transcript := strings.Join(ed.chat.Lines(), "\n")
				convey.So(transcript, convey.ShouldContainSubstring, "Accept with :accept or :reject.")
				convey.So(transcript, convey.ShouldContainSubstring, "implementation proposal accepted")
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
