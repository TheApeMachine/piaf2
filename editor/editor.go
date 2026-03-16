package editor

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/theapemachine/piaf/event"
	"github.com/theapemachine/piaf/theme"
	"github.com/theapemachine/piaf/wire"
)

const (
	modeNormal  = "NORMAL"
	modeInsert  = "INSERT"
	modeCommand = "COMMAND"
)

const jumpAlphabet = "asdfghjklqwertyuiopzxcvbnm"
const jumpPromptTarget = "word"

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
	colorPicker   *theme.Picker
	inChat        bool
	inExplorer    bool
	inPalette     bool
	inKanban      bool
	inColorPicker bool
	path          string
	mode          string
	commandLine   []rune
	quitRequested bool
	jumpNeedle    rune
	jumpPrefix    string
	jumpTargets   []jumpTarget
	jumpCodeLen   int
	jumpWordFreqs map[string]int
	jumpWordRoot  string
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

				if runeVal == ' ' && ed.mode == modeInsert {
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
	if ed.inColorPicker {
		ed.handleColorPickerKey(key)
		ed.render()

		return
	}

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
	if ed.inColorPicker {
		ed.handleColorPickerRune(r)
		ed.render()

		return
	}

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
	case ' ':
		ed.buffer.MoveRight()
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
	case "theme":
		ed.executeThemeCommand(arg)
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

func (ed *Editor) handleColorPickerKey(key event.Key) {
	switch key {
	case event.KeyEsc, event.KeyEnter:
		ed.inColorPicker = false
		ed.colorPicker = nil
		separatorLineCache = sync.Map{}
	case event.KeyUp:
		ed.colorPicker.MoveUp()
	case event.KeyDown:
		ed.colorPicker.MoveDown()
	case event.KeyLeft:
		ed.colorPicker.Decrease(5)
	case event.KeyRight:
		ed.colorPicker.Increase(5)
	}
}

func (ed *Editor) handleColorPickerRune(r rune) {
	switch r {
	case 'j':
		ed.colorPicker.MoveDown()
	case 'k':
		ed.colorPicker.MoveUp()
	case 'h':
		ed.colorPicker.Decrease(5)
	case 'l':
		ed.colorPicker.Increase(5)
	case 'H':
		ed.colorPicker.Decrease(1)
	case 'L':
		ed.colorPicker.Increase(1)
	case '\t':
		ed.colorPicker.CycleChannel()
	}
}

func (ed *Editor) openColorPicker() {
	ed.colorPicker = theme.NewPicker(theme.Active())
	ed.inColorPicker = true
}

func (ed *Editor) executeThemeCommand(arg string) {
	parts := strings.Fields(arg)
	sub := ""
	if len(parts) > 0 {
		sub = parts[0]
	}

	subArg := ""
	if len(parts) > 1 {
		subArg = strings.Join(parts[1:], " ")
	}

	switch sub {
	case "save":
		theme.Active().Save()
	case "load":
		if subArg != "" {
			if loaded, err := theme.Load(subArg); err == nil {
				theme.SetActive(loaded)
				separatorLineCache = sync.Map{}
			}
		}
	case "rename":
		if subArg != "" {
			theme.Active().Rename(subArg)
		}
	case "edit", "picker", "":
		ed.openColorPicker()
	case "default":
		theme.SetActive(theme.Default())
		separatorLineCache = sync.Map{}
	}
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

	if ed.inColorPicker && ed.colorPicker != nil {
		lines = ed.colorPicker.Overlay(lines, ed.buffer.width, ed.buffer.height)
		cursorRow = 0
		cursorCol = 0
		cmdLine = styleBold + styleFgBrand() + "THEME" + styleReset + " j/k:move h/l:adjust Tab:channel Enter:done"
	} else if ed.inPalette && ed.palette != nil {
		lines = stylePaletteOverlay(lines, ed.palette, ed.buffer.width, ed.buffer.height)
		cursorRow = 0
		cursorCol = 0
		cmdLine = ""
	} else if ed.jumpActive() {
		lines = ed.jumpLines(lines)
		cmdLine = styleBold + styleFgHighlight() + "f " + styleReset + jumpPromptTarget
		if ed.jumpPrefix != "" {
			cmdLine += " " + ed.jumpPrefix
		}
	} else if ed.mode == modeCommand {
		cmdLine = styleBold + styleFgBrand() + ": " + styleReset + string(ed.commandLine)
		cursorRow = ed.buffer.height - 1
		cursorCol = 2 + len(ed.commandLine)
	} else if ed.mode == modeInsert && ed.inChat {
		cmdLine = styleBold + styleFgHighlight() + "> " + styleReset + string(ed.commandLine)
		cursorRow = ed.buffer.height - 1
		cursorCol = 2 + len(ed.commandLine)
	}

	displayMode := ed.mode
	if ed.inColorPicker {
		displayMode = "THEME"
	} else if ed.inPalette {
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
	word string
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
*/
func (ed *Editor) startJump() {
	targets := ed.visibleJumpTargets()

	if len(targets) == 0 {
		return
	}

	frequencies := ed.jumpWordFrequencies()
	sort.SliceStable(targets, func(left, right int) bool {
		leftFrequency := jumpWordFrequency(targets[left].word, frequencies)
		rightFrequency := jumpWordFrequency(targets[right].word, frequencies)

		if leftFrequency != rightFrequency {
			return leftFrequency > rightFrequency
		}

		if targets[left].row != targets[right].row {
			return targets[left].row < targets[right].row
		}

		return targets[left].col < targets[right].col
	})

	codes := jumpCodes(targets, frequencies)

	for index := range targets {
		targets[index].code = codes[index]
	}

	ed.jumpNeedle = 0
	ed.jumpPrefix = ""
	ed.jumpTargets = targets
	ed.jumpCodeLen = 0
}

/*
handleJumpRune processes a rune during jump mode.
It consumes label runes until a word jump target is uniquely identified.
*/
func (ed *Editor) handleJumpRune(r rune) bool {
	if !ed.jumpActive() {
		return false
	}

	r = unicode.ToLower(r)

	if r >= 256 || !jumpAlphabetLookup[byte(r)] {
		ed.clearJump()

		return true
	}

	ed.jumpPrefix += string(r)
	targets := ed.filteredJumpTargets()

	if len(targets) == 0 {
		ed.clearJump()

		return true
	}

	if len(targets) == 1 && targets[0].code == ed.jumpPrefix {
		ed.buffer.cursorRow = targets[0].row
		ed.buffer.cursorCol = targets[0].col
		ed.clearJump()
	}

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

		for col, r := range line {
			if !isJumpWordRune(r) {
				continue
			}

			if col > 0 && isJumpWordRune(line[col-1]) {
				continue
			}

			wordEnd := col + 1

			for wordEnd < len(line) && isJumpWordRune(line[wordEnd]) {
				wordEnd++
			}

			targets = append(targets, jumpTarget{
				row:  row,
				col:  col,
				word: strings.ToLower(string(line[col:wordEnd])),
			})
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

type jumpCodeNode struct {
	index    int
	weight   int
	order    int
	children []*jumpCodeNode
}

func jumpCodes(targets []jumpTarget, frequencies map[string]int) []string {
	if len(targets) == 0 {
		return nil
	}

	if len(targets) == 1 {
		return []string{string(jumpAlphabet[0])}
	}

	nodes := make([]*jumpCodeNode, 0, len(targets)+len(jumpAlphabet))

	for index, target := range targets {
		nodes = append(nodes, &jumpCodeNode{
			index:  index,
			weight: jumpWordFrequency(target.word, frequencies),
			order:  index,
		})
	}

	padding := (len(jumpAlphabet) - 1 - (len(nodes)-1)%(len(jumpAlphabet)-1)) % (len(jumpAlphabet) - 1)
	order := len(nodes)

	for range padding {
		nodes = append(nodes, &jumpCodeNode{index: -1, order: order})
		order++
	}

	for len(nodes) > 1 {
		sort.Slice(nodes, func(left, right int) bool {
			if nodes[left].weight != nodes[right].weight {
				return nodes[left].weight < nodes[right].weight
			}

			return nodes[left].order > nodes[right].order
		})

		children := append([]*jumpCodeNode(nil), nodes[:len(jumpAlphabet)]...)
		parent := &jumpCodeNode{index: -1, order: order, children: children}

		for _, child := range children {
			parent.weight += child.weight
		}

		order++
		nodes = append(nodes[len(jumpAlphabet):], parent)
	}

	codes := make([]string, len(targets))
	assignJumpCodes(nodes[0], "", codes)

	return codes
}

func assignJumpCodes(node *jumpCodeNode, prefix string, codes []string) {
	if node == nil {
		return
	}

	if len(node.children) == 0 {
		if node.index >= 0 {
			if prefix == "" {
				prefix = string(jumpAlphabet[0])
			}

			codes[node.index] = prefix
		}

		return
	}

	sort.SliceStable(node.children, func(left, right int) bool {
		if node.children[left].weight != node.children[right].weight {
			return node.children[left].weight > node.children[right].weight
		}

		return node.children[left].order < node.children[right].order
	})

	for index, child := range node.children {
		assignJumpCodes(child, prefix+string(jumpAlphabet[index]), codes)
	}
}

func jumpWordFrequency(word string, frequencies map[string]int) int {
	if frequencies == nil {
		return 1
	}

	if frequency := frequencies[word]; frequency > 0 {
		return frequency
	}

	return 1
}

func isJumpWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func newJumpAlphabetLookup() [256]bool {
	lookup := [256]bool{}

	for _, r := range jumpAlphabet {
		lookup[byte(r)] = true
	}

	return lookup
}

func (ed *Editor) jumpWordFrequencies() map[string]int {
	root := ed.workspaceRoot()

	if ed.jumpWordFreqs != nil && ed.jumpWordRoot == root {
		return ed.jumpWordFreqs
	}

	frequencies := map[string]int{}

	filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if entry.IsDir() {
			name := entry.Name()

			if strings.HasPrefix(name, ".") && path != root {
				return filepath.SkipDir
			}

			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil || bytes.IndexByte(data, 0) >= 0 {
			return nil
		}

		countJumpWords(data, frequencies)

		return nil
	})

	ed.jumpWordFreqs = frequencies
	ed.jumpWordRoot = root

	return ed.jumpWordFreqs
}

func countJumpWords(data []byte, frequencies map[string]int) {
	runes := []rune(string(data))
	start := -1

	for index, r := range runes {
		if isJumpWordRune(r) {
			if start == -1 {
				start = index
			}

			continue
		}

		if start >= 0 {
			frequencies[strings.ToLower(string(runes[start:index]))]++
			start = -1
		}
	}

	if start >= 0 {
		frequencies[strings.ToLower(string(runes[start:]))]++
	}
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
