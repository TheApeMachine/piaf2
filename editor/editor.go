package editor

import (
	"io"
	"strings"

	"github.com/theapemachine/piaf/event"
	"github.com/theapemachine/piaf/wire"
)

const (
	modeNormal  = "NORMAL"
	modeInsert  = "INSERT"
	modeCommand = "COMMAND"
)

/*
Editor consumes event wire format and emits Frame wire format.
Implements io.ReadWriteCloser: Write receives events, Read yields Frame bytes.
*/
type Editor struct {
	buffer        *Buffer
	explorer      *Explorer
	inExplorer    bool
	path          string
	mode          string
	commandLine   []rune
	quitRequested bool
	output        []byte
	readOff       int
}

/*
editorOpts configures Editor with options.
*/
type editorOpts func(*Editor)

/*
NewEditor instantiates a new Editor in normal mode.
*/
func NewEditor(opts ...editorOpts) *Editor {
	ed := &Editor{
		buffer: NewBuffer(),
		mode:   modeNormal,
	}

	for _, opt := range opts {
		opt(ed)
	}

	ed.render()

	return ed
}

/*
EditorWithSize sets the terminal dimensions.
*/
func EditorWithSize(width, height int) editorOpts {
	return func(ed *Editor) {
		ed.buffer.width = width
		ed.buffer.height = height
	}
}

/*
EditorWithPath sets the initial path. Empty or "." starts in explorer mode.
A file path loads that file into the buffer.
*/
func EditorWithPath(path string) editorOpts {
	return func(ed *Editor) {
		ed.path = path
		ed.inExplorer = path == "" || path == "."
		if ed.inExplorer {
			ed.explorer = NewExplorer(path)
		} else {
			ed.buffer.LoadPath(path)
		}
	}
}

/*
Read implements the io.Reader interface.
Returns buffered Frame wire format; EOF when exhausted.
*/
func (ed *Editor) Read(p []byte) (n int, err error) {
	if ed.readOff >= len(ed.output) {
		return 0, io.EOF
	}

	n = copy(p, ed.output[ed.readOff:])
	ed.readOff += n

	return n, nil
}

/*
Write implements the io.Writer interface.
Processes event wire format, updates buffer, re-renders.
*/
func (ed *Editor) Write(p []byte) (n int, err error) {
	offset := 0

	for offset < len(p) {
		isRune, runeVal, key, size := event.ParseEvent(p[offset:])

		if size == 0 {
			break
		}

		offset += size

		if isRune {
			ed.handleRune(runeVal)
		} else {
			ed.handleKey(key)
		}
	}

	return len(p), nil
}

/*
Close implements the io.Closer interface.
*/
func (ed *Editor) Close() error {
	return ed.buffer.Close()
}

func (ed *Editor) handleKey(key event.Key) {
	switch key {
	case event.KeyEsc:
		if ed.mode == modeCommand {
			ed.mode = modeNormal
			ed.commandLine = ed.commandLine[:0]
		} else {
			ed.mode = modeNormal
		}
	case event.KeyUp:
		if ed.mode != modeCommand {
			if ed.inExplorer {
				ed.explorer.MoveUp()
			} else {
				ed.buffer.MoveUp()
			}
		}
	case event.KeyDown:
		if ed.mode != modeCommand {
			if ed.inExplorer {
				ed.explorer.MoveDown()
			} else {
				ed.buffer.MoveDown()
			}
		}
	case event.KeyLeft:
		if ed.mode != modeCommand && !ed.inExplorer {
			ed.buffer.MoveLeft()
		}
	case event.KeyRight:
		if ed.mode != modeCommand && !ed.inExplorer {
			ed.buffer.MoveRight()
		}
	case event.KeyBackspace:
		if ed.mode == modeInsert {
			ed.buffer.DeleteBefore()
		} else if ed.mode == modeCommand && len(ed.commandLine) > 0 {
			ed.commandLine = ed.commandLine[:len(ed.commandLine)-1]
		}
	case event.KeyEnter:
		if ed.mode == modeCommand {
			ed.executeCommand()
			ed.mode = modeNormal
			ed.commandLine = ed.commandLine[:0]
		} else if ed.mode == modeInsert {
			ed.buffer.Newline()
		} else if ed.inExplorer {
			target, _, loadFile := ed.explorer.Enter()
			if loadFile {
				ed.buffer.LoadPath(target)
				ed.path = target
				ed.inExplorer = false
			}
		}
	}

	ed.render()
}

func (ed *Editor) handleRune(r rune) {
	switch ed.mode {
	case modeInsert:
		if !ed.inExplorer {
			ed.buffer.InsertRune(r)
		}
	case modeCommand:
		ed.commandLine = append(ed.commandLine, r)
	default:
		if ed.inExplorer {
			ed.applyExplorerCommand(r)
		} else {
			ed.applyNormalCommand(r)
		}
	}

	ed.render()
}

func (ed *Editor) applyExplorerCommand(r rune) {
	switch r {
	case ':':
		ed.mode = modeCommand
		ed.commandLine = ed.commandLine[:0]
	}
}

func (ed *Editor) applyNormalCommand(r rune) {
	switch r {
	case ':':
		ed.mode = modeCommand
		ed.commandLine = ed.commandLine[:0]
	case 'i':
		ed.mode = modeInsert
	case 'a':
		ed.buffer.MoveRight()
		ed.mode = modeInsert
	case 'A':
		ed.buffer.MoveLineEnd()
		ed.mode = modeInsert
	case 'I':
		ed.buffer.MoveLineStart()
		ed.mode = modeInsert
	case 'o':
		ed.buffer.MoveLineEnd()
		ed.buffer.Newline()
		ed.mode = modeInsert
	case 'O':
		ed.buffer.MoveLineStart()
		ed.buffer.Newline()
		ed.buffer.MoveUp()
		ed.mode = modeInsert
	case 'h':
		ed.buffer.MoveLeft()
	case 'j':
		ed.buffer.MoveDown()
	case 'k':
		ed.buffer.MoveUp()
	case 'l':
		ed.buffer.MoveRight()
	case 'x':
		ed.buffer.DeleteAt()
	case '0':
		ed.buffer.MoveLineStart()
	case '$':
		ed.buffer.MoveLineEnd()
	}
}

func (ed *Editor) executeCommand() {
	line := strings.TrimSpace(string(ed.commandLine))
	parts := strings.Fields(line)
	cmd := ""
	if len(parts) > 0 {
		cmd = parts[0]
	}
	arg := ""
	if len(parts) > 1 {
		arg = strings.Join(parts[1:], " ")
	}

	switch cmd {
	case "q", "quit":
		ed.quitRequested = true
	case "q!":
		ed.quitRequested = true
	case "w", "write":
	case "wq":
		ed.quitRequested = true
	case "e", "edit":
		if arg != "" {
			ed.buffer.LoadPath(arg)
			ed.path = arg
			ed.inExplorer = false
		}
	case "E", "Ex", "Explore":
		ed.explorer = NewExplorer(arg)
		ed.inExplorer = true
	}
}

/*
QuitRequested returns true if the user executed :q, :q!, or :wq.
*/
func (ed *Editor) QuitRequested() bool {
	return ed.quitRequested
}

func (ed *Editor) render() {
	cmdLine := ""
	cursorRow := ed.buffer.cursorRow
	cursorCol := ed.buffer.cursorCol
	lines := ed.buffer.StringLines()

	if ed.inExplorer && ed.explorer != nil {
		lines = ed.explorer.Lines()
		cursorRow = ed.explorer.Cursor()
		cursorCol = 0
	}

	if ed.mode == modeCommand {
		cmdLine = ": " + string(ed.commandLine)
		cursorRow = ed.buffer.height - 1
		cursorCol = 2 + len(ed.commandLine)
	}

	displayMode := ed.mode
	if ed.inExplorer && displayMode == modeNormal {
		displayMode = "EXPLORER"
	}

	frame := &wire.Frame{
		Lines:       lines,
		CursorRow:   uint32(cursorRow),
		CursorCol:   uint32(cursorCol),
		Width:       uint32(ed.buffer.width),
		Height:      uint32(ed.buffer.height),
		Mode:        displayMode,
		CommandLine: cmdLine,
	}

	data, err := io.ReadAll(frame)

	if err != nil || len(data) == 0 {
		return
	}

	ed.output = append(ed.output[:0], data...)
	ed.readOff = 0
}
