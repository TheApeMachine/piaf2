package editor

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
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
const jumpPromptTarget = "target"

var jumpAlphabetLookup = newJumpAlphabetLookup()

/*
Editor consumes event wire format and emits Frame wire format.
Implements io.ReadWriteCloser: Write receives events, Read yields Frame bytes.
*/
type Editor struct {
	buffer        *Buffer
	chat          *Chat
	explorer      *Explorer
	palette       *Palette
	kanbanView    *KanbanView
	inChat        bool
	inExplorer    bool
	inPalette     bool
	inKanban      bool
	path          string
	mode          string
	commandLine   []rune
	quitRequested bool
	jumpNeedle    rune
	jumpPrefix    string
	jumpTargets   []jumpTarget
	jumpCodeLen   int
	output        []byte
	readOff       int
	streamUpdates chan struct{}
	systemPrompt  string
	swallowSpace  bool
	pendingSpace  bool
	chatTimeout   time.Duration
	chatDumpPath  string
}

/*
EditorOpt is an option for configuring the Editor.
Exported so callers can build option slices (e.g. from config).
*/
type EditorOpt func(*Editor)

type editorOpts = EditorOpt

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
EditorWithChatDumpPath appends all chat model output (discussion + implementation) to the given file.
*/
func EditorWithChatDumpPath(path string) editorOpts {
	return func(ed *Editor) {
		ed.chatDumpPath = strings.TrimSpace(path)
	}
}

/*
EditorWithChatTimeout sets the per-stage timeout for chat API calls.
Override when tool-heavy rounds need more than the default 180s.
*/
func EditorWithChatTimeout(timeout time.Duration) editorOpts {
	return func(ed *Editor) {
		if timeout > 0 {
			ed.chatTimeout = timeout
		}
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
			if ed.swallowSpace && runeVal == ' ' {
				ed.swallowSpace = false
			} else {
				ed.swallowSpace = false
				if ed.pendingSpace {
					ed.handleRune(' ')
					ed.pendingSpace = false
				}
				if runeVal == ' ' {
					ed.pendingSpace = true
				} else {
					ed.handleRune(runeVal)
				}
			}
		} else {
			if ed.pendingSpace && key == event.KeyBackspace {
				ed.pendingSpace = false
			}
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

	if ed.inPalette {
		ed.handlePaletteKey(key)
		ed.render()

		return
	}

	switch key {
	case event.KeyEsc:
		if ed.inKanban {
			ed.inKanban = false
			ed.kanbanView = nil
		} else if ed.mode == modeCommand {
			ed.mode = modeNormal
			ed.commandLine = ed.commandLine[:0]
		} else {
			ed.mode = modeNormal
		}
	case event.KeyUp:
		if ed.mode != modeCommand && !ed.inPalette {
			if ed.inChat {
				if ed.chat != nil {
					ed.chat.ScrollUp()
				}
			} else if ed.inExplorer {
				ed.explorer.MoveUp()
			} else {
				ed.buffer.MoveUp()
			}
		}
	case event.KeyDown:
		if ed.mode != modeCommand && !ed.inPalette {
			if ed.inChat {
				if ed.chat != nil {
					ed.chat.ScrollDown()
				}
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
		ed.swallowSpace = true
		if ed.inPalette {
			ed.palette.Backspace()
		} else if ed.mode == modeInsert && ed.inChat {
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
		if ed.inPalette {
			ed.executePaletteSelection()
		} else if ed.mode == modeCommand {
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
	if ed.inPalette {
		ed.palette.Append(r)
		ed.render()

		return
	}

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

func (ed *Editor) handlePaletteKey(key event.Key) {
	switch key {
	case event.KeyEsc:
		ed.inPalette = false
		ed.palette = nil
	case event.KeyBackspace:
		ed.palette.Backspace()
	case event.KeyUp:
		ed.palette.MoveUp()
	case event.KeyDown:
		ed.palette.MoveDown()
	case event.KeyEnter:
		ed.executePaletteSelection()
	}

	ed.render()
}

func (ed *Editor) applyExplorerCommand(r rune) {
	switch r {
	case '/':
		ed.openPalette()
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
	case '/':
		ed.openPalette()
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
	case '/':
		ed.openPalette()
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
			ed.inKanban = false
		case "board":
			if ed.chat != nil && ed.chat.Mode() == "IMPLEMENT" {
				ed.inKanban = !ed.inKanban
				if ed.inKanban {
					ed.kanbanView = NewKanbanView(ed.chat.Kanban(), ed.buffer.width)
				} else {
					ed.kanbanView = nil
				}
			}
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
		case "implement", "team":
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
	case "implement", "team":
		ed.openChat("IMPLEMENT")
	case "board":
		ed.openChat("IMPLEMENT")
		if ed.chat != nil && ed.chat.Mode() == "IMPLEMENT" {
			ed.inKanban = true
			ed.kanbanView = NewKanbanView(ed.chat.Kanban(), ed.buffer.width)
		}
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

	if ed.inKanban && ed.kanbanView != nil && ed.chat != nil {
		ed.kanbanView.SetKanban(ed.chat.Kanban())
		lines = ed.kanbanView.Lines()
		maxLines := ed.buffer.height - 1
		if maxLines > 0 && len(lines) > maxLines {
			lines = lines[:maxLines]
		}
		cursorRow = 0
		cursorCol = 0
	} else if ed.inChat && ed.chat != nil {
		raw := ed.chat.Lines()
		lines = wrapChatLines(raw, ed.buffer.width)
		maxLines := ed.buffer.height - 1

		offset := ed.chat.ScrollOffset()
		maxOffset := len(lines) - maxLines
		if maxOffset < 0 {
			maxOffset = 0
		}
		if offset > maxOffset {
			offset = maxOffset
		}

		if maxLines > 0 && len(lines) > maxLines {
			start := len(lines) - maxLines - offset
			lines = lines[start : start+maxLines]
		}

		lines = styleChatLines(lines, ed.buffer.width)
		cursorRow = len(lines) - 1
		if cursorRow < 0 {
			cursorRow = 0
		}
		cursorCol = 0
	} else if ed.inExplorer && ed.explorer != nil {
		raw := ed.explorer.Lines()
		maxLines := ed.buffer.height - 1

		cursorRow = ed.explorer.Cursor()
		offset := 0
		if cursorRow >= maxLines {
			offset = cursorRow - maxLines + 1
		}

		if maxLines > 0 && len(raw) > maxLines {
			end := offset + maxLines
			if end > len(raw) {
				end = len(raw)
			}
			lines = raw[offset:end]
			cursorRow -= offset
		} else {
			lines = raw
		}

		lines = styleExplorerLines(lines)
		cursorCol = 0
	} else if !ed.jumpActive() || ed.jumpNeedle == 0 {
		lines = styleCodeLines(lines, ed.path)
	}

	if ed.inPalette && ed.palette != nil {
		lines = stylePaletteOverlay(lines, ed.palette, ed.buffer.width, ed.buffer.height)
		cursorRow = 0
		cursorCol = 0
		cmdLine = ""
	} else if ed.jumpActive() {
		if ed.jumpNeedle == 0 {
			cmdLine = styleBold + styleFgHighlight + "f " + styleReset + jumpPromptTarget
		} else {
			lines = ed.jumpLines(lines)
			cmdLine = styleBold + styleFgHighlight + "f " + styleReset + string(ed.jumpNeedle) + ed.jumpPrefix
		}
	} else if ed.mode == modeCommand {
		cmdLine = styleBold + styleFgBrand + ": " + styleReset + string(ed.commandLine)
		cursorRow = ed.buffer.height - 1
		cursorCol = 2 + len(ed.commandLine)
	} else if ed.mode == modeInsert && ed.inChat {
		cmdLine = styleBold + styleFgHighlight + "> " + styleReset + string(ed.commandLine)
		cursorRow = ed.buffer.height - 1
		cursorCol = 2 + len(ed.commandLine)
	}

	displayMode := ed.mode
	if ed.inPalette {
		displayMode = "PALETTE"
	} else if ed.inKanban {
		displayMode = "BOARD"
	} else if ed.inChat && ed.chat != nil && displayMode == modeNormal {
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

func (ed *Editor) openPalette() {
	ed.palette = NewPalette(PaletteWithRoot(ed.workspaceRoot()))
	ed.inPalette = true
	ed.palette.refresh()
}

func (ed *Editor) executePaletteSelection() {
	if ed.palette == nil {
		ed.inPalette = false
		return
	}

	kind, value := ed.palette.Selected()
	ed.inPalette = false
	ed.palette = nil

	if kind == "" {
		return
	}

	if kind == paletteKindCommand {
		ed.commandLine = []rune(value)
		ed.mode = modeCommand
		ed.executeCommand()
		ed.mode = modeNormal
		ed.commandLine = ed.commandLine[:0]
	} else if kind == paletteKindFile {
		ed.buffer.LoadPath(value)
		ed.path = value
		ed.inExplorer = false
		ed.inChat = false
	} else if kind == paletteKindContent {
		parts := strings.SplitN(value, ":", 2)
		if len(parts) == 2 {
			ed.buffer.LoadPath(parts[0])
			ed.path = parts[0]
			ed.inExplorer = false
			ed.inChat = false

			lineNum := 0
			for _, r := range parts[1] {
				if r >= '0' && r <= '9' {
					lineNum = lineNum*10 + int(r-'0')
				}
			}
			if lineNum > 0 && lineNum <= len(ed.buffer.lines) {
				ed.buffer.cursorRow = lineNum - 1
				ed.buffer.cursorCol = 0
			}
		}
	}
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
		if ed.chatTimeout > 0 {
			opts = append(opts, ChatWithTimeout(ed.chatTimeout))
		}
		if ed.chatDumpPath != "" {
			opts = append(opts, ChatWithDumpFile(ed.chatDumpPath))
		}
		ed.chat = NewChat(opts...)
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
	ed.jumpNeedle = 0
	ed.jumpPrefix = ""
	ed.jumpTargets = nil
	ed.jumpCodeLen = 0
}

/*
startJump initiates jump mode by discovering visible targets.
The next rune selects which character to jump toward before labels are assigned.
*/
func (ed *Editor) startJump() {
	targets := ed.visibleJumpTargets()

	if len(targets) == 0 {
		return
	}

	ed.jumpNeedle = 0
	ed.jumpPrefix = ""
	ed.jumpTargets = targets
	ed.jumpCodeLen = 0
}

/*
handleJumpRune processes a rune during jump mode.
It first narrows jump targets to a chosen character, then consumes label runes.
*/
func (ed *Editor) handleJumpRune(r rune) bool {
	if !ed.jumpActive() {
		return false
	}

	if ed.jumpNeedle == 0 {
		ed.selectJumpTargets(r)

		return true
	}

	r = unicode.ToLower(r)

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
selectJumpTargets narrows visible jump targets to the requested character.
It jumps immediately when the character is unique on screen.
*/
func (ed *Editor) selectJumpTargets(r rune) {
	needle := unicode.ToLower(r)
	targets := ed.jumpTargetsForNeedle(needle)

	if len(targets) == 0 {
		ed.clearJump()

		return
	}

	if len(targets) == 1 {
		ed.buffer.cursorRow = targets[0].row
		ed.buffer.cursorCol = targets[0].col
		ed.clearJump()

		return
	}

	codeLen := jumpCodeLength(len(targets))

	for index := range targets {
		targets[index].code = jumpCode(index, codeLen)
	}

	ed.jumpNeedle = needle
	ed.jumpPrefix = ""
	ed.jumpTargets = targets
	ed.jumpCodeLen = codeLen
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
jumpTargetsForNeedle returns visible jump targets whose rune matches needle.
Matching is case-insensitive so target selection stays lightweight.
*/
func (ed *Editor) jumpTargetsForNeedle(needle rune) []jumpTarget {
	targets := make([]jumpTarget, 0, len(ed.jumpTargets))

	for _, target := range ed.jumpTargets {
		line := ed.buffer.lines[target.row]

		if target.col >= len(line) {
			continue
		}

		if unicode.ToLower(line[target.col]) == needle {
			targets = append(targets, target)
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

const ansiInverse = "\033[7m"
const ansiReset = "\033[0m"

/*
jumpLines overlays jump label characters onto the visible lines.
Labels are shown in inverse video after each target without replacing the original text.
Processes right-to-left so column positions stay valid across inserts.
*/
func (ed *Editor) jumpLines(lines []string) []string {
	overlaidLines := append([]string(nil), lines...)
	targets := ed.filteredJumpTargets()

	for index := len(targets) - 1; index >= 0; index-- {
		target := targets[index]

		if target.row >= len(overlaidLines) {
			continue
		}

		line := overlaidLines[target.row]
		runes := []rune(line)

		if target.col >= len(runes) || len(target.code) <= len(ed.jumpPrefix) {
			continue
		}

		label := string(rune(target.code[len(ed.jumpPrefix)]))
		before := string(runes[:target.col+1])
		after := string(runes[target.col+1:])
		overlaidLines[target.row] = before + ansiInverse + label + ansiReset + after
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
