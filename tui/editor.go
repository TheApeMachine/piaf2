package tui

import "io"

const (
	modeNormal = "NORMAL"
	modeInsert = "INSERT"
)

type editorState uint8

const (
	stateNormal editorState = iota
	stateEscape
	stateCSI
)

/*
Editor is the core text editor.
It processes raw terminal key bytes via Write and emits ANSI output via Read.
Internally it owns a Buffer and Renderer, re-rendering on every keystroke.
*/
type Editor struct {
	buffer    *Buffer
	renderer  *Renderer
	mode      string
	state     editorState
	output    []byte
	readOff   int
	renderBuf [65536]byte
}

/*
editorOpts configures Editor with options.
*/
type editorOpts func(*Editor)

/*
NewEditor instantiates a new Editor in normal mode with an empty buffer.
The initial screen state is rendered immediately so the first Read returns output.
*/
func NewEditor(opts ...editorOpts) *Editor {
	editor := &Editor{
		buffer:   NewBuffer(),
		renderer: NewRenderer(),
		mode:     modeNormal,
	}

	for _, opt := range opts {
		opt(editor)
	}

	editor.render()

	return editor
}

/*
EditorWithSize sets the terminal dimensions.
*/
func EditorWithSize(width, height int) editorOpts {
	return func(editor *Editor) {
		editor.buffer.width = width
		editor.buffer.height = height
	}
}

/*
Read implements the io.Reader interface.
Returns buffered ANSI output from the last rendered frame; returns io.EOF when exhausted.
*/
func (editor *Editor) Read(p []byte) (n int, err error) {
	if editor.readOff >= len(editor.output) {
		return 0, io.EOF
	}

	n = copy(p, editor.output[editor.readOff:])
	editor.readOff += n

	return n, nil
}

/*
Write implements the io.Writer interface.
Processes raw terminal input bytes, dispatching to the active mode handler.
Flushes any incomplete escape sequence at the end of the call so that
a lone ESC (e.g. to exit INSERT mode) is handled within a single Write.
*/
func (editor *Editor) Write(p []byte) (n int, err error) {
	for _, b := range p {
		editor.processByte(b)
	}

	if editor.state != stateNormal {
		editor.mode = modeNormal
		editor.state = stateNormal
		editor.render()
	}

	return len(p), nil
}

/*
Close implements the io.Closer interface.
Closes the underlying renderer to restore the terminal screen buffer.
*/
func (editor *Editor) Close() error {
	return editor.renderer.Close()
}

/*
processByte dispatches a single input byte through the escape-sequence state machine.
*/
func (editor *Editor) processByte(b byte) {
	switch editor.state {
	case stateNormal:
		editor.handleNormalByte(b)
	case stateEscape:
		editor.handleEscapeByte(b)
	case stateCSI:
		editor.handleCSIByte(b)
	}
}

/*
handleNormalByte handles a byte outside any escape sequence.
*/
func (editor *Editor) handleNormalByte(b byte) {
	if b == 0x1b {
		editor.state = stateEscape
		return
	}

	if editor.mode == modeInsert {
		editor.handleInsertByte(b)
	} else {
		editor.handleNormalModeByte(b)
	}
}

/*
handleEscapeByte handles the byte immediately following ESC.
*/
func (editor *Editor) handleEscapeByte(b byte) {
	if b == '[' {
		editor.state = stateCSI
		return
	}

	editor.mode = modeNormal
	editor.state = stateNormal
	editor.render()
}

/*
handleCSIByte handles the final byte of a CSI escape sequence (ESC [ <b>).
Arrow keys are mapped to cursor movement; unknown sequences are silently dropped.
*/
func (editor *Editor) handleCSIByte(b byte) {
	editor.state = stateNormal

	switch b {
	case 'A':
		editor.buffer.MoveUp()
	case 'B':
		editor.buffer.MoveDown()
	case 'C':
		editor.buffer.MoveRight()
	case 'D':
		editor.buffer.MoveLeft()
	}

	editor.render()
}

/*
handleInsertByte processes a byte while the editor is in insert mode.
Printable ASCII is inserted; control codes trigger structural edits or mode changes.
*/
func (editor *Editor) handleInsertByte(b byte) {
	switch b {
	case 0x7f, 0x08:
		editor.buffer.DeleteBefore()
	case '\r', '\n':
		editor.buffer.Newline()
	default:
		if b >= 0x20 {
			editor.buffer.InsertRune(rune(b))
		}
	}

	editor.render()
}

/*
handleNormalModeByte processes a byte while the editor is in normal (command) mode.
Implements a vim-compatible subset of motion and editing commands.
*/
func (editor *Editor) handleNormalModeByte(b byte) {
	switch b {
	case 'i':
		editor.mode = modeInsert
	case 'a':
		editor.buffer.MoveRight()
		editor.mode = modeInsert
	case 'A':
		editor.buffer.MoveLineEnd()
		editor.mode = modeInsert
	case 'I':
		editor.buffer.MoveLineStart()
		editor.mode = modeInsert
	case 'o':
		editor.buffer.MoveLineEnd()
		editor.buffer.Newline()
		editor.mode = modeInsert
	case 'O':
		editor.buffer.MoveLineStart()
		editor.buffer.Newline()
		editor.buffer.MoveUp()
		editor.mode = modeInsert
	case 'h':
		editor.buffer.MoveLeft()
	case 'j':
		editor.buffer.MoveDown()
	case 'k':
		editor.buffer.MoveUp()
	case 'l':
		editor.buffer.MoveRight()
	case 'x':
		editor.buffer.DeleteAt()
	case '0':
		editor.buffer.MoveLineStart()
	case '$':
		editor.buffer.MoveLineEnd()
	}

	editor.render()
}

/*
render builds a Frame from the current buffer and mode, pipes it through the
Renderer, and stores the resulting ANSI bytes for the next Read call.
Uses a fixed-size stack buffer to avoid per-keystroke heap allocation.
*/
func (editor *Editor) render() {
	frame := &Frame{
		Lines:     editor.buffer.StringLines(),
		CursorRow: uint32(editor.buffer.cursorRow),
		CursorCol: uint32(editor.buffer.cursorCol),
		Width:     uint32(editor.buffer.width),
		Height:    uint32(editor.buffer.height),
		Mode:      editor.mode,
	}

	data, err := io.ReadAll(frame)

	if err != nil || len(data) == 0 {
		return
	}

	editor.renderer.Write(data)

	count, _ := editor.renderer.Read(editor.renderBuf[:])

	editor.output = editor.renderBuf[:count]
	editor.readOff = 0
}
