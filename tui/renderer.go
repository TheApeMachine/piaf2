package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/theapemachine/piaf/wire"
)

const (
	ansiEnterAlternate = "\033[?1049h"
	ansiExitAlternate  = "\033[?1049l"
	ansiCursorHome     = "\033[H"
	ansiClearDown      = "\033[J"
	ansiCursorPos      = "\033[%d;%dH"
	ansiShowCursor     = "\033[?25h"
	ansiHideCursor     = "\033[?25l"
	ansiClearLine      = "\033[2K"
	ansiReset          = "\033[0m"
	ansiBold           = "\033[1m"
	ansiDim            = "\033[2m"
	ansiFgGreen        = "\033[32m"
	ansiFgYellow       = "\033[33m"
	ansiFgBlue         = "\033[34m"
	ansiFgMagenta      = "\033[35m"
	ansiFgCyan         = "\033[36m"
	ansiFgGray         = "\033[90m"
	ansiFgWhite        = "\033[97m"

	ansiFgBrand     = "\033[38;2;108;80;255m"
	ansiFgHighlight = "\033[38;2;254;135;255m"
	ansiBgBrand     = "\033[48;2;108;80;255m"
	ansiBgHighlight = "\033[48;2;254;135;255m"

	boxDash = "\u2500"
)

/*
Renderer converts Frame state to ANSI terminal output.
Implements io.ReadWriteCloser: Write accepts Frame wire format, Read yields ANSI bytes.
*/
type Renderer struct {
	output      []byte
	readOffset  int
	alternateOn bool
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

	estimatedSize := 128

	for _, line := range frame.Lines {
		estimatedSize += len(line) + len(ansiClearLine) + 2
	}

	if cap(renderer.output) < estimatedSize {
		renderer.output = make([]byte, 0, estimatedSize*2)
	} else {
		renderer.output = renderer.output[:0]
	}

	renderer.readOffset = 0

	if !renderer.alternateOn {
		renderer.output = append(renderer.output, ansiEnterAlternate...)
		renderer.alternateOn = true
	}

	renderer.output = append(renderer.output, ansiHideCursor...)
	renderer.output = append(renderer.output, ansiCursorHome...)

	maxLines := int(frame.Height) - 1

	for index, line := range frame.Lines {
		if index >= maxLines {
			break
		}

		renderer.output = append(renderer.output, ansiClearLine...)
		renderer.output = append(renderer.output, line...)
		renderer.output = append(renderer.output, '\r', '\n')
	}

	renderer.output = append(renderer.output, ansiClearDown...) // clear any leftover lines from previous longer frames

	renderer.output = append(renderer.output, fmt.Sprintf(ansiCursorPos, int(frame.Height), 1)...)
	renderer.output = append(renderer.output, ansiClearLine...)

	if frame.CommandLine != "" {
		renderer.output = append(renderer.output, frame.CommandLine...)
	} else if frame.Mode != "" {
		renderer.output = append(renderer.output, styledStatusBar(frame.Mode, int(frame.Width))...)
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

func styledStatusBar(mode string, width int) string {
	pill := styledModePill(mode)
	pillLength := len(mode) + 2

	label := ansiFgBrand + ansiDim + "piaf" + ansiReset
	labelLength := 4

	gap := width - pillLength - labelLength
	if gap < 0 {
		gap = 0
	}

	return pill + strings.Repeat(" ", gap) + label
}

func styledModePill(mode string) string {
	content := " " + mode + " "

	switch mode {
	case "NORMAL":
		return ansiFgGray + ansiDim + content + ansiReset
	case "INSERT":
		return ansiBgBrand + ansiFgWhite + ansiBold + content + ansiReset
	case "COMMAND":
		return ansiBgHighlight + "\033[38;2;40;0;40m" + ansiBold + content + ansiReset
	case "IMPLEMENT":
		return ansiBgHighlight + "\033[38;2;40;0;40m" + ansiBold + content + ansiReset
	default:
		return ansiBgBrand + ansiFgWhite + ansiBold + content + ansiReset
	}
}
