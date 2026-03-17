package editor

import (
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

/*
Buffer stores the editor's text content and cursor position.
Implements io.ReadWriteCloser: Write inserts UTF-8 text at the cursor,
Read emits the current state as wire format (internal use).
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
LoadPath reads the file at path and populates the buffer.
Creates one empty line if the file cannot be read.
*/
func (buf *Buffer) LoadPath(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		buf.lines = [][]rune{{}}
		buf.cursorRow = 0
		buf.cursorCol = 0
		return
	}

	lines := strings.Split(string(data), "\n")
	buf.lines = make([][]rune, len(lines))

	for index, line := range lines {
		buf.lines[index] = []rune(line)
	}

	buf.cursorRow = 0
	buf.cursorCol = 0
	buf.readBuf = nil
}

/*
BufferWithSize sets the terminal dimensions.
*/
func BufferWithSize(width, height int) bufferOpts {
	return func(buf *Buffer) {
		buf.width = width
		buf.height = height
	}
}

/*
InsertRune inserts a rune at the current cursor position.
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
DeleteBefore removes the rune before the cursor.
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
DeleteAt removes the rune at the cursor.
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
Newline splits the current line at the cursor.
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
MoveUp moves the cursor one row up.
*/
func (buf *Buffer) MoveUp() {
	if buf.cursorRow > 0 {
		buf.cursorRow--
		buf.clampCol()
	}
}

/*
MoveDown moves the cursor one row down.
*/
func (buf *Buffer) MoveDown() {
	if buf.cursorRow < len(buf.lines)-1 {
		buf.cursorRow++
		buf.clampCol()
	}
}

/*
MoveLeft moves the cursor one column left.
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
MoveRight moves the cursor one column right.
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
StringLines returns the buffer content as strings.
*/
func (buf *Buffer) StringLines() []string {
	result := make([]string, len(buf.lines))

	for index, line := range buf.lines {
		result[index] = string(line)
	}

	return result
}

/*
clampCol ensures the cursor column stays within the current line length.
*/
func (buf *Buffer) clampCol() {
	if lineLen := len(buf.lines[buf.cursorRow]); buf.cursorCol > lineLen {
		buf.cursorCol = lineLen
	}
}

/*
Read implements the io.Reader interface.
Buffer does not support reading; returns EOF.
*/
func (buf *Buffer) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

/*
Write implements the io.Writer interface.
Inserts the given UTF-8 bytes at the cursor.
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
Buffer has no resources to release.
*/
func (buf *Buffer) Close() error {
	return nil
}
