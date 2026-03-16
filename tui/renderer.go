package tui

import (
	"io"
	"strconv"
	"strings"

	"github.com/theapemachine/piaf/theme"
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

	boxDash = "\u2500"
)

func ansiFgBrand() string       { return theme.Active().FgBrand() }
func ansiFgHighlight() string   { return theme.Active().FgHighlight() }
func ansiBgBrand() string       { return theme.Active().BgBrand() }
func ansiBgHighlight() string   { return theme.Active().BgHighlight() }
func ansiBgSubtleBrand() string { return theme.Active().BgSubtleBrand() }
func ansiBgSubtleHigh() string  { return theme.Active().BgSubtleHigh() }

/*
Renderer converts Frame state to ANSI terminal output.
Implements io.ReadWriteCloser: Write accepts Frame wire format, Read yields ANSI bytes.
*/
type Renderer struct {
	output      []byte
	readOffset  int
	alternateOn bool
	frame       wire.Frame
	lastLines   []string
	lastStatus  string
	lastWidth   int
	lastHeight  int

	statusMode  string
	statusWidth int
	statusLine  string
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

	frame := &renderer.frame
	if _, err := frame.Write(p); err != nil {
		return 0, err
	}

	height := int(frame.Height)
	width := int(frame.Width)
	maxLines := 0
	if height > 0 {
		maxLines = height - 1
	}

	visibleLines := len(frame.Lines)
	if visibleLines > maxLines {
		visibleLines = maxLines
	}

	statusLine := frame.CommandLine
	if statusLine == "" && frame.Mode != "" {
		statusLine = renderer.cachedStatusBar(frame.Mode, width)
	}

	fullRedraw := !renderer.alternateOn || height != renderer.lastHeight || width != renderer.lastWidth
	estimatedSize := len(statusLine) + 64

	if fullRedraw {
		for index := 0; index < visibleLines; index++ {
			estimatedSize += len(frame.Lines[index]) + len(ansiClearLine) + 8
		}
	} else {
		prevVisibleLines := len(renderer.lastLines)
		if prevVisibleLines > renderer.lastHeight-1 {
			prevVisibleLines = renderer.lastHeight - 1
		}

		maxVisibleLines := visibleLines
		if prevVisibleLines > maxVisibleLines {
			maxVisibleLines = prevVisibleLines
		}

		for index := 0; index < maxVisibleLines; index++ {
			currentLine := ""
			if index < visibleLines {
				currentLine = frame.Lines[index]
			}

			previousLine := ""
			if index < prevVisibleLines {
				previousLine = renderer.lastLines[index]
			}

			if currentLine != previousLine {
				estimatedSize += len(currentLine) + len(ansiClearLine) + 16
			}
		}

		if statusLine != renderer.lastStatus {
			estimatedSize += len(statusLine) + len(ansiClearLine) + 16
		}
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

	if fullRedraw {
		renderer.output = append(renderer.output, ansiCursorHome...)

		for index := 0; index < visibleLines; index++ {
			renderer.output = append(renderer.output, ansiClearLine...)
			renderer.output = append(renderer.output, frame.Lines[index]...)
			renderer.output = append(renderer.output, '\r', '\n')
		}

		renderer.output = append(renderer.output, ansiClearDown...)
		renderer.output = appendCursorPos(renderer.output, height, 1)
		renderer.output = append(renderer.output, ansiClearLine...)
		if statusLine != "" {
			renderer.output = append(renderer.output, statusLine...)
		}
	} else {
		prevVisibleLines := len(renderer.lastLines)
		if prevVisibleLines > renderer.lastHeight-1 {
			prevVisibleLines = renderer.lastHeight - 1
		}

		maxVisibleLines := visibleLines
		if prevVisibleLines > maxVisibleLines {
			maxVisibleLines = prevVisibleLines
		}

		for index := 0; index < maxVisibleLines; index++ {
			currentLine := ""
			if index < visibleLines {
				currentLine = frame.Lines[index]
			}

			previousLine := ""
			if index < prevVisibleLines {
				previousLine = renderer.lastLines[index]
			}

			if currentLine == previousLine {
				continue
			}

			renderer.output = appendCursorPos(renderer.output, index+1, 1)
			renderer.output = append(renderer.output, ansiClearLine...)
			if currentLine != "" {
				renderer.output = append(renderer.output, currentLine...)
			}
		}

		if statusLine != renderer.lastStatus {
			renderer.output = appendCursorPos(renderer.output, height, 1)
			renderer.output = append(renderer.output, ansiClearLine...)
			if statusLine != "" {
				renderer.output = append(renderer.output, statusLine...)
			}
		}
	}

	row := int(frame.CursorRow) + 1
	col := int(frame.CursorCol) + 1
	renderer.output = appendCursorPos(renderer.output, row, col)
	renderer.output = append(renderer.output, ansiShowCursor...)
	renderer.storeFrame(frame.Lines[:visibleLines], statusLine, width, height)

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
		renderer.lastLines = renderer.lastLines[:0]
		renderer.lastStatus = ""
		renderer.lastWidth = 0
		renderer.lastHeight = 0
	}

	return nil
}

func (renderer *Renderer) cachedStatusBar(mode string, width int) string {
	if mode == renderer.statusMode && width == renderer.statusWidth {
		return renderer.statusLine
	}

	renderer.statusMode = mode
	renderer.statusWidth = width
	renderer.statusLine = styledStatusBar(mode, width)

	return renderer.statusLine
}

func (renderer *Renderer) storeFrame(lines []string, statusLine string, width, height int) {
	if cap(renderer.lastLines) < len(lines) {
		renderer.lastLines = make([]string, len(lines))
	} else {
		renderer.lastLines = renderer.lastLines[:len(lines)]
	}

	copy(renderer.lastLines, lines)
	renderer.lastStatus = statusLine
	renderer.lastWidth = width
	renderer.lastHeight = height
}

func appendCursorPos(dst []byte, row, col int) []byte {
	dst = append(dst, '\033', '[')
	dst = strconv.AppendInt(dst, int64(row), 10)
	dst = append(dst, ';')
	dst = strconv.AppendInt(dst, int64(col), 10)
	dst = append(dst, 'H')

	return dst
}

func styledStatusBar(mode string, width int) string {
	pill := styledModePill(mode)
	pillLength := len(mode) + 2

	label := ansiFgHighlight() + ansiDim + "piaf" + ansiReset
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
		return ansiBgSubtleBrand() + ansiFgBrand() + ansiBold + content + ansiReset
	case "COMMAND":
		return ansiBgSubtleHigh() + ansiFgHighlight() + ansiBold + content + ansiReset
	case "IMPLEMENT":
		return ansiBgSubtleHigh() + ansiFgHighlight() + ansiBold + content + ansiReset
	case "BOARD":
		return ansiBgBrand() + ansiFgWhite + ansiBold + content + ansiReset
	default:
		return ansiBgSubtleBrand() + ansiFgBrand() + ansiBold + content + ansiReset
	}
}
