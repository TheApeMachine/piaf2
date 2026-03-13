package editor

import (
	"io"
	"os"
	"path/filepath"
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
	chat          *Chat
	explorer      *Explorer
	inChat        bool
	inExplorer    bool
	path          string
	mode          string
	commandLine   []rune
	quitRequested bool
	output        []byte
	readOff       int
	streamUpdates chan struct{}
	systemPrompt  string
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
EditorWithSystemPrompt sets the AI system prompt for chat provider requests.
*/
func EditorWithSystemPrompt(prompt string) editorOpts {
	return func(ed *Editor) {
		ed.systemPrompt = prompt
	}
}

/*
EditorWithStreamUpdates sets the channel to signal when streaming produces new content.
*/
func EditorWithStreamUpdates(ch chan struct{}) editorOpts {
	return func(ed *Editor) {
		ed.streamUpdates = ch
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
			if ed.inChat {
			} else if ed.inExplorer {
				ed.explorer.MoveUp()
			} else {
				ed.buffer.MoveUp()
			}
		}
	case event.KeyDown:
		if ed.mode != modeCommand {
			if ed.inChat {
			} else if ed.inExplorer {
				ed.explorer.MoveDown()
			} else {
				ed.buffer.MoveDown()
			}
		}
	case event.KeyLeft:
		if ed.mode != modeCommand && !ed.inExplorer && !ed.inChat {
			ed.buffer.MoveLeft()
		}
	case event.KeyRight:
		if ed.mode != modeCommand && !ed.inExplorer && !ed.inChat {
			ed.buffer.MoveRight()
		}
	case event.KeyBackspace:
		if ed.mode == modeInsert && ed.inChat {
			if len(ed.commandLine) > 0 {
				ed.commandLine = ed.commandLine[:len(ed.commandLine)-1]
			}
		} else if ed.mode == modeInsert {
			ed.buffer.DeleteBefore()
		} else if ed.mode == modeCommand && len(ed.commandLine) > 0 {
			ed.commandLine = ed.commandLine[:len(ed.commandLine)-1]
		}
	case event.KeyRefresh:
		ed.render()
	case event.KeyEnter:
		if ed.mode == modeCommand {
			ed.executeCommand()
			ed.mode = modeNormal
			ed.commandLine = ed.commandLine[:0]
		} else if ed.mode == modeInsert && ed.inChat {
			if ed.chat != nil {
				go ed.chat.Submit(string(ed.commandLine))
			}
			ed.commandLine = ed.commandLine[:0]
			ed.mode = modeNormal
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
		if ed.inChat {
			ed.commandLine = append(ed.commandLine, r)
		} else if !ed.inExplorer {
			ed.buffer.InsertRune(r)
		}
	case modeCommand:
		ed.commandLine = append(ed.commandLine, r)
	default:
		if ed.inExplorer {
			ed.applyExplorerCommand(r)
		} else if ed.inChat {
			ed.applyChatCommand(r)
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
	case 'j':
		ed.explorer.MoveDown()
	case 'k':
		ed.explorer.MoveUp()
	case 'h':
		for ed.explorer.Cursor() > 0 {
			ed.explorer.MoveUp()
		}
		target, _, loadFile := ed.explorer.Enter()
		if loadFile {
			ed.buffer.LoadPath(target)
			ed.path = target
			ed.inExplorer = false
		}
	case 'l':
		target, _, loadFile := ed.explorer.Enter()
		if loadFile {
			ed.buffer.LoadPath(target)
			ed.path = target
			ed.inExplorer = false
		}
	}
}

func (ed *Editor) applyChatCommand(r rune) {
	switch r {
	case ':':
		ed.mode = modeCommand
		ed.commandLine = ed.commandLine[:0]
	case 'i':
		ed.mode = modeInsert
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

	if ed.inChat {
		switch cmd {
		case "q", "quit":
			ed.inChat = false
		case "accept":
			if ed.chat != nil && ed.chat.Mode() == "IMPLEMENT" {
				ed.chat.Accept()
			}
		case "reject":
			if ed.chat != nil && ed.chat.Mode() == "IMPLEMENT" {
				ed.chat.Reject()
			}
		case "chat":
			ed.openChat("CHAT")
		case "implement":
			ed.openChat("IMPLEMENT")
		}

		return
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
	case "chat":
		ed.openChat("CHAT")
	case "implement":
		ed.openChat("IMPLEMENT")
	}
}

func wrapChatLines(raw []string, width int) []string {
	if width <= 0 {
		return raw
	}

	out := make([]string, 0)

	for index, entry := range raw {
		if index > 0 {
			out = append(out, "")
		}

		for _, segment := range strings.Split(entry, "\n") {
			runes := []rune(strings.TrimRight(segment, " \t"))
			for len(runes) > 0 {
				if len(runes) <= width {
					out = append(out, string(runes))
					break
				}

				breakAt := width
				for index := width - 1; index >= 0; index-- {
					if index < len(runes) && (runes[index] == ' ' || runes[index] == '\t') {
						breakAt = index + 1
						break
					}
				}

				out = append(out, string(runes[:breakAt]))
				runes = runes[breakAt:]
				runes = trimLeftSpaces(runes)
			}
		}
	}

	return out
}

func trimLeftSpaces(runes []rune) []rune {
	for index, r := range runes {
		if r != ' ' && r != '\t' {
			return runes[index:]
		}
	}

	return nil
}

func (ed *Editor) render() {
	cmdLine := ""
	cursorRow := ed.buffer.cursorRow
	cursorCol := ed.buffer.cursorCol
	lines := ed.buffer.StringLines()

	if ed.inChat && ed.chat != nil {
		raw := ed.chat.Lines()
		lines = wrapChatLines(raw, ed.buffer.width)
		maxLines := ed.buffer.height - 1
		if maxLines > 0 && len(lines) > maxLines {
			lines = lines[len(lines)-maxLines:]
		}
		cursorRow = len(lines) - 1
		if cursorRow < 0 {
			cursorRow = 0
		}
		cursorCol = 0
	} else if ed.inExplorer && ed.explorer != nil {
		lines = ed.explorer.Lines()
		cursorRow = ed.explorer.Cursor()
		cursorCol = 0
	}

	if ed.mode == modeCommand {
		cmdLine = ": " + string(ed.commandLine)
		cursorRow = ed.buffer.height - 1
		cursorCol = 2 + len(ed.commandLine)
	} else if ed.mode == modeInsert && ed.inChat {
		cmdLine = "> " + string(ed.commandLine)
		cursorRow = ed.buffer.height - 1
		cursorCol = 2 + len(ed.commandLine)
	}

	displayMode := ed.mode
	if ed.inChat && ed.chat != nil && displayMode == modeNormal {
		displayMode = ed.chat.Mode()
	} else if ed.inExplorer && displayMode == modeNormal {
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
		Quit:        ed.quitRequested,
	}

	data, err := io.ReadAll(frame)

	if err != nil || len(data) == 0 {
		return
	}

	ed.output = append(ed.output[:0], data...)
	ed.readOff = 0
}

func (ed *Editor) openChat(mode string) {
	if ed.chat == nil {
		opts := []chatOpts{ChatWithRoot(ed.workspaceRoot())}
		if ed.streamUpdates != nil {
			opts = append(opts, ChatWithOnStream(ed.onStreamUpdate))
		}
		if ed.systemPrompt != "" {
			opts = append(opts, ChatWithSystemPrompt(ed.systemPrompt))
		}
		ed.chat = NewChat(opts...)
	}

	ed.inChat = true
	ed.inExplorer = false
	ed.chat.SetMode(mode)
}

func (ed *Editor) onStreamUpdate() {
	if ed.streamUpdates != nil {
		select {
		case ed.streamUpdates <- struct{}{}:
		default:
		}
	}
}

func (ed *Editor) workspaceRoot() string {
	if ed.path == "" {
		return "."
	}

	info, err := os.Stat(ed.path)
	if err != nil {
		return filepath.Dir(ed.path)
	}

	if info.IsDir() {
		return ed.path
	}

	return filepath.Dir(ed.path)
}
