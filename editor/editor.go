package editor

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/theapemachine/piaf/event"
	"github.com/theapemachine/piaf/wire"
)

const (
	modeNormal  = "NORMAL"
	modeInsert  = "INSERT"
	modeCommand = "COMMAND"
)

const jumpAlphabet = "asdfghjklqwertyuiopzxcvbnm"

var jumpAlphabetLookup = newJumpAlphabetLookup()

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
	jumpPrefix    string
	jumpTargets   []jumpTarget
	jumpCodeLen   int
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
	if ed.jumpActive() {
		ed.clearJump()
		ed.render()

		return
	}

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
	case event.KeyEnter:
		if ed.mode == modeCommand {
			ed.executeCommand()
			ed.mode = modeNormal
			ed.commandLine = ed.commandLine[:0]
		} else if ed.mode == modeInsert && ed.inChat {
			if ed.chat != nil {
				ed.chat.Submit(string(ed.commandLine))
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
		if !ed.handleJumpRune(r) {
			if ed.inExplorer {
				ed.applyExplorerCommand(r)
			} else if ed.inChat {
				ed.applyChatCommand(r)
			} else {
				ed.applyNormalCommand(r)
			}
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
	case 'f':
		ed.startJump()
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

	if ed.inChat && ed.chat != nil {
		lines = ed.chat.Lines()
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

	if ed.jumpActive() {
		lines = ed.jumpLines(lines)
		cmdLine = "f " + ed.jumpPrefix
	} else if ed.mode == modeCommand {
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
		ed.chat = NewChat(ChatWithRoot(ed.workspaceRoot()))
	}

	ed.inChat = true
	ed.inExplorer = false
	ed.chat.SetMode(mode)
}

/*
jumpTarget represents a single navigable position in jump mode.
It holds the row and column coordinates and the associated label code.
*/
type jumpTarget struct {
	row  int
	col  int
	code string
}

/*
jumpActive returns true if jump mode is currently active.
*/
func (ed *Editor) jumpActive() bool {
	return len(ed.jumpTargets) > 0
}

/*
clearJump exits jump mode by resetting all jump state.
*/
func (ed *Editor) clearJump() {
	ed.jumpPrefix = ""
	ed.jumpTargets = nil
	ed.jumpCodeLen = 0
}

/*
startJump initiates jump mode by discovering visible targets and assigning label codes.
*/
func (ed *Editor) startJump() {
	targets := ed.visibleJumpTargets()

	if len(targets) == 0 {
		return
	}

	codeLen := jumpCodeLength(len(targets))

	for index := range targets {
		targets[index].code = jumpCode(index, codeLen)
	}

	ed.jumpPrefix = ""
	ed.jumpTargets = targets
	ed.jumpCodeLen = codeLen
}

/*
handleJumpRune processes a rune during jump mode.
It returns true if the rune was consumed by jump mode.
*/
func (ed *Editor) handleJumpRune(r rune) bool {
	if !ed.jumpActive() {
		return false
	}

	if r >= 256 || !jumpAlphabetLookup[byte(r)] {
		ed.clearJump()

		return true
	}

	ed.jumpPrefix += string(r)

	if len(ed.jumpPrefix) < ed.jumpCodeLen {
		if len(ed.filteredJumpTargets()) == 0 {
			ed.clearJump()
		}

		return true
	}

	for _, target := range ed.jumpTargets {
		if target.code == ed.jumpPrefix {
			ed.buffer.cursorRow = target.row
			ed.buffer.cursorCol = target.col
			break
		}
	}

	ed.clearJump()

	return true
}

/*
visibleJumpTargets collects all navigable positions in the visible buffer area.
*/
func (ed *Editor) visibleJumpTargets() []jumpTarget {
	lines := ed.buffer.lines
	maxRows := ed.buffer.height - 1

	if maxRows <= 0 || maxRows > len(lines) {
		maxRows = len(lines)
	}

	targets := make([]jumpTarget, 0)

	for row := range maxRows {
		line := lines[row]

		if len(line) == 0 {
			continue
		}

		targets = append(targets, jumpTarget{row: row, col: 0})

		for col, r := range line {
			if col == 0 || unicode.IsSpace(r) {
				continue
			}

			targets = append(targets, jumpTarget{row: row, col: col})
		}
	}

	return targets
}

/*
filteredJumpTargets returns targets matching the current jump prefix.
*/
func (ed *Editor) filteredJumpTargets() []jumpTarget {
	if !ed.jumpActive() {
		return nil
	}

	if ed.jumpPrefix == "" {
		return ed.jumpTargets
	}

	targets := make([]jumpTarget, 0, len(ed.jumpTargets))

	for _, target := range ed.jumpTargets {
		if strings.HasPrefix(target.code, ed.jumpPrefix) {
			targets = append(targets, target)
		}
	}

	return targets
}

/*
jumpLines overlays jump label characters onto the visible lines.
*/
func (ed *Editor) jumpLines(lines []string) []string {
	overlaidLines := append([]string(nil), lines...)

	for _, target := range ed.filteredJumpTargets() {
		if target.row >= len(overlaidLines) {
			continue
		}

		line := []rune(overlaidLines[target.row])

		if target.col >= len(line) || len(target.code) <= len(ed.jumpPrefix) {
			continue
		}

		line[target.col] = rune(target.code[len(ed.jumpPrefix)])
		overlaidLines[target.row] = string(line)
	}

	return overlaidLines
}

/*
jumpCodeLength calculates the minimum label length needed to encode count targets.
*/
func jumpCodeLength(count int) int {
	codeLen := 1
	capacity := len(jumpAlphabet)

	for count > capacity {
		codeLen++
		capacity *= len(jumpAlphabet)
	}

	return codeLen
}

/*
jumpCode generates a base-N label for the given index with the specified length.
*/
func jumpCode(index, codeLen int) string {
	code := make([]byte, codeLen)

	for position := codeLen - 1; position >= 0; position-- {
		code[position] = jumpAlphabet[index%len(jumpAlphabet)]
		index /= len(jumpAlphabet)
	}

	return string(code)
}

func newJumpAlphabetLookup() [256]bool {
	lookup := [256]bool{}

	for _, r := range jumpAlphabet {
		lookup[byte(r)] = true
	}

	return lookup
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
