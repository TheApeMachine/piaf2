package tui

import (
	"fmt"
	"io"

	"github.com/theapemachine/piaf/wire"
)

const (
	ansiEnterAlternate = "\033[?1049h"
	ansiExitAlternate  = "\033[?1049l"
	ansiClearHome      = "\033[H\033[2J"
	ansiCursorPos      = "\033[%d;%dH"
	ansiShowCursor     = "\033[?25h"
	ansiHideCursor     = "\033[?25l"
	ansiClearLine      = "\033[2K"
)

/*
Renderer converts Frame state to ANSI terminal output.
Implements io.ReadWriteCloser: Write accepts Frame wire format, Read yields ANSI bytes.
*/
type Renderer struct {
	output        []byte
	readOffset    int
	alternateOn   bool
}

/*
NewRenderer creates a new Renderer instance.
*/
func NewRenderer() *Renderer {
	return &Renderer{}
}

/*
Read implements the io.Reader interface.
*/
func (renderer *Renderer) Read(p []byte) (n int, err error) {
	if renderer.readOffset >= len(renderer.output) {
		return 0, io.EOF
	}

	n = copy(p, renderer.output[renderer.readOffset:])
	renderer.readOffset += n

	return n, nil
}

/*
Write implements the io.Writer interface.
Accepts Frame wire format; renders to ANSI and buffers for Read.
*/
func (renderer *Renderer) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	frame := &wire.Frame{}
	if _, err := frame.Write(p); err != nil {
		return 0, err
	}

	renderer.output = renderer.output[:0]
	renderer.readOffset = 0

	if !renderer.alternateOn {
		renderer.output = append(renderer.output, ansiEnterAlternate...)
		renderer.alternateOn = true
	}

	renderer.output = append(renderer.output, ansiClearHome...)

	maxLines := int(frame.Height) - 1

	for index, line := range frame.Lines {
		if index >= maxLines {
			break
		}

		renderer.output = append(renderer.output, line...)
		renderer.output = append(renderer.output, '\r', '\n')
	}

	renderer.output = append(renderer.output, fmt.Sprintf(ansiCursorPos, int(frame.Height), 1)...)
	renderer.output = append(renderer.output, ansiClearLine...)

	if frame.CommandLine != "" {
		renderer.output = append(renderer.output, frame.CommandLine...)
	} else if frame.Mode != "" {
		renderer.output = append(renderer.output, "-- "+frame.Mode+" --"...)
	}

	row := int(frame.CursorRow) + 1
	col := int(frame.CursorCol) + 1
	renderer.output = append(renderer.output, fmt.Sprintf(ansiCursorPos, row, col)...)
	renderer.output = append(renderer.output, ansiShowCursor...)

	return len(p), nil
}

/*
Close implements the io.Closer interface.
Restores main screen buffer when alternate was active.
*/
func (renderer *Renderer) Close() error {
	if renderer.alternateOn {
		renderer.output = append(renderer.output[:0], ansiExitAlternate...)
		renderer.readOffset = 0
		renderer.alternateOn = false
	}

	return nil
}
