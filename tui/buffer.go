package tui

import (
	"io"
	"unicode/utf8"
)

/*
Buffer stores the editor's text content and cursor position.
It implements io.ReadWriteCloser: Write inserts UTF-8 text at the cursor,
Read emits the current state as Frame wire format.
*/
type Buffer struct {
	lines      [][]rune
	cursorRow  int
	cursorCol  int
	width      int
	height     int
	readBuf    []byte
	readOffset int
}

/*
bufferOpts configures Buffer with options.
*/
type bufferOpts func(*Buffer)

/*
NewBuffer instantiates a new Buffer with one empty line.
The buffer starts empty and ready for editing.
*/
func NewBuffer(opts ...bufferOpts) *Buffer {
	buf := &Buffer{
		lines:  [][]rune{{}},
		width:  80,
		height: 24,
	}

	for _, opt := range opts {
		opt(buf)
	}

	return buf
}

/*
BufferWithSize sets the terminal dimensions used when emitting Frame data.
*/
func BufferWithSize(width, height int) bufferOpts {
	return func(buf *Buffer) {
		buf.width = width
		buf.height = height
	}
}

/*
InsertRune inserts a rune at the current cursor position and advances the cursor.
*/
func (buf *Buffer) InsertRune(r rune) {
	line := buf.lines[buf.cursorRow]
	col := buf.cursorCol

	if col > len(line) {
		col = len(line)
	}

	newLine := make([]rune, len(line)+1)
	copy(newLine, line[:col])
	newLine[col] = r
	copy(newLine[col+1:], line[col:])

	buf.lines[buf.cursorRow] = newLine
	buf.cursorCol++
	buf.readBuf = nil
}

/*
DeleteBefore removes the rune immediately before the cursor (backspace semantics).
If the cursor is at column 0, it merges the current line with the line above.
*/
func (buf *Buffer) DeleteBefore() {
	if buf.cursorCol > 0 {
		line := buf.lines[buf.cursorRow]
		col := buf.cursorCol

		newLine := make([]rune, len(line)-1)
		copy(newLine, line[:col-1])
		copy(newLine[col-1:], line[col:])

		buf.lines[buf.cursorRow] = newLine
		buf.cursorCol--
	} else if buf.cursorRow > 0 {
		prevLine := buf.lines[buf.cursorRow-1]
		currLine := buf.lines[buf.cursorRow]

		merged := make([]rune, len(prevLine)+len(currLine))
		copy(merged, prevLine)
		copy(merged[len(prevLine):], currLine)

		buf.cursorCol = len(prevLine)
		buf.lines[buf.cursorRow-1] = merged
		buf.lines = append(buf.lines[:buf.cursorRow], buf.lines[buf.cursorRow+1:]...)
		buf.cursorRow--
	}

	buf.readBuf = nil
}

/*
DeleteAt removes the rune at the cursor position (delete-key semantics).
If the cursor is at end of line, it merges with the next line.
*/
func (buf *Buffer) DeleteAt() {
	line := buf.lines[buf.cursorRow]

	if buf.cursorCol < len(line) {
		newLine := make([]rune, len(line)-1)
		copy(newLine, line[:buf.cursorCol])
		copy(newLine[buf.cursorCol:], line[buf.cursorCol+1:])

		buf.lines[buf.cursorRow] = newLine
	} else if buf.cursorRow < len(buf.lines)-1 {
		nextLine := buf.lines[buf.cursorRow+1]

		merged := make([]rune, len(line)+len(nextLine))
		copy(merged, line)
		copy(merged[len(line):], nextLine)

		buf.lines[buf.cursorRow] = merged
		buf.lines = append(buf.lines[:buf.cursorRow+1], buf.lines[buf.cursorRow+2:]...)
	}

	buf.readBuf = nil
}

/*
Newline splits the current line at the cursor, inserting a new line below.
*/
func (buf *Buffer) Newline() {
	line := buf.lines[buf.cursorRow]
	col := buf.cursorCol

	before := make([]rune, col)
	copy(before, line[:col])

	after := make([]rune, len(line)-col)
	copy(after, line[col:])

	newLines := make([][]rune, len(buf.lines)+1)
	copy(newLines, buf.lines[:buf.cursorRow+1])
	newLines[buf.cursorRow] = before
	newLines[buf.cursorRow+1] = after
	copy(newLines[buf.cursorRow+2:], buf.lines[buf.cursorRow+1:])

	buf.lines = newLines
	buf.cursorRow++
	buf.cursorCol = 0
	buf.readBuf = nil
}

/*
MoveUp moves the cursor one row up, clamping the column to the line length.
*/
func (buf *Buffer) MoveUp() {
	if buf.cursorRow > 0 {
		buf.cursorRow--
		buf.clampCol()
	}
}

/*
MoveDown moves the cursor one row down, clamping the column to the line length.
*/
func (buf *Buffer) MoveDown() {
	if buf.cursorRow < len(buf.lines)-1 {
		buf.cursorRow++
		buf.clampCol()
	}
}

/*
MoveLeft moves the cursor one column left, wrapping to the previous line end if needed.
*/
func (buf *Buffer) MoveLeft() {
	if buf.cursorCol > 0 {
		buf.cursorCol--
	} else if buf.cursorRow > 0 {
		buf.cursorRow--
		buf.cursorCol = len(buf.lines[buf.cursorRow])
	}
}

/*
MoveRight moves the cursor one column right, wrapping to the next line start if needed.
*/
func (buf *Buffer) MoveRight() {
	if buf.cursorCol < len(buf.lines[buf.cursorRow]) {
		buf.cursorCol++
	} else if buf.cursorRow < len(buf.lines)-1 {
		buf.cursorRow++
		buf.cursorCol = 0
	}
}

/*
MoveLineStart moves the cursor to the beginning of the current line.
*/
func (buf *Buffer) MoveLineStart() {
	buf.cursorCol = 0
}

/*
MoveLineEnd moves the cursor to the end of the current line.
*/
func (buf *Buffer) MoveLineEnd() {
	buf.cursorCol = len(buf.lines[buf.cursorRow])
}

/*
StringLines returns the buffer content as a slice of strings for Frame encoding.
*/
func (buf *Buffer) StringLines() []string {
	result := make([]string, len(buf.lines))

	for index, line := range buf.lines {
		result[index] = string(line)
	}

	return result
}

/*
clampCol ensures the cursor column is within the current line bounds.
*/
func (buf *Buffer) clampCol() {
	if lineLen := len(buf.lines[buf.cursorRow]); buf.cursorCol > lineLen {
		buf.cursorCol = lineLen
	}
}

/*
Read implements the io.Reader interface.
Emits the current buffer state as Frame wire format.
*/
func (buf *Buffer) Read(p []byte) (n int, err error) {
	if buf.readBuf == nil {
		frame := &Frame{
			Lines:     buf.StringLines(),
			CursorRow: uint32(buf.cursorRow),
			CursorCol: uint32(buf.cursorCol),
			Width:     uint32(buf.width),
			Height:    uint32(buf.height),
		}

		if buf.readBuf, err = io.ReadAll(frame); err != nil {
			return 0, err
		}

		buf.readOffset = 0
	}

	if buf.readOffset >= len(buf.readBuf) {
		return 0, io.EOF
	}

	n = copy(p, buf.readBuf[buf.readOffset:])
	buf.readOffset += n

	return n, nil
}

/*
Write implements the io.Writer interface.
Inserts the given UTF-8 bytes as text at the cursor position.
*/
func (buf *Buffer) Write(p []byte) (n int, err error) {
	for len(p) > 0 {
		r, size := utf8.DecodeRune(p)

		if r == utf8.RuneError && size <= 1 {
			break
		}

		buf.InsertRune(r)
		p = p[size:]
		n += size
	}

	return n, nil
}

/*
Close implements the io.Closer interface.
*/
func (buf *Buffer) Close() error {
	return nil
}
