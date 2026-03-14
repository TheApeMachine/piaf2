package tui

import (
	"fmt"
	"io"
	"time"
)

const introFrameDelay = 90 * time.Millisecond

/*
Intro renders a brief branded splash animation on startup.
Implements io.ReadWriteCloser: Read yields ANSI frames with built-in timing,
Write is a no-op, Close is a no-op.
*/
type Intro struct {
	width      int
	height     int
	frames     [][]byte
	frameIndex int
	readOff    int
}

/*
NewIntro creates an Intro animation sized for the given terminal dimensions.
*/
func NewIntro(width, height int) *Intro {
	intro := &Intro{
		width:  width,
		height: height,
	}

	intro.generateFrames()

	return intro
}

/*
Read implements the io.Reader interface.
Returns one animation frame per call with a short sleep between frames.
*/
func (intro *Intro) Read(p []byte) (n int, err error) {
	if intro.frameIndex >= len(intro.frames) {
		return 0, io.EOF
	}

	if intro.frameIndex > 0 && intro.readOff == 0 {
		time.Sleep(introFrameDelay)
	}

	frame := intro.frames[intro.frameIndex]
	n = copy(p, frame[intro.readOff:])
	intro.readOff += n

	if intro.readOff >= len(frame) {
		intro.frameIndex++
		intro.readOff = 0
	}

	return n, nil
}

/*
Write implements the io.Writer interface.
Intro is output-only; writes are discarded.
*/
func (intro *Intro) Write(p []byte) (n int, err error) {
	return len(p), nil
}

/*
Close implements the io.Closer interface.
*/
func (intro *Intro) Close() error {
	return nil
}

func (intro *Intro) generateFrames() {
	if intro.width < 20 || intro.height < 8 {
		return
	}

	logo := "piaf"
	tagline := "A.I. Code Editor"

	centerRow := intro.height / 2
	logoCol := (intro.width-len(logo))/2 + 1
	tagRow := centerRow + 2
	tagCol := (intro.width-len(tagline))/2 + 1

	brandR, brandG, brandB := 108, 80, 255
	highlightR, highlightG, highlightB := 254, 135, 255
	glowR, glowG, glowB := 40, 30, 80

	clear := ansiEnterAlternate + ansiHideCursor + ansiCursorHome + ansiClearDown

	for charIdx := 0; charIdx < len(logo); charIdx++ {
		var buf []byte
		buf = append(buf, clear...)
		buf = append(buf, fmt.Sprintf(ansiCursorPos, centerRow, logoCol)...)

		for prev := 0; prev < charIdx; prev++ {
			buf = append(buf, fmt.Sprintf("\033[38;2;%d;%d;%dm", brandR, brandG, brandB)...)
			buf = append(buf, ansiBold...)
			buf = append(buf, logo[prev])
			buf = append(buf, ansiReset...)
		}

		buf = append(buf, fmt.Sprintf("\033[48;2;%d;%d;%dm", glowR, glowG, glowB)...)
		buf = append(buf, fmt.Sprintf("\033[38;2;%d;%d;%dm", highlightR, highlightG, highlightB)...)
		buf = append(buf, ansiBold...)
		buf = append(buf, logo[charIdx])
		buf = append(buf, ansiReset...)

		intro.frames = append(intro.frames, buf)
	}

	dimTagFg := fmt.Sprintf("\033[38;2;%d;%d;%dm", highlightR/3, highlightG/3, highlightB/3)
	{
		var buf []byte
		buf = append(buf, clear...)
		buf = append(buf, fmt.Sprintf(ansiCursorPos, centerRow, logoCol)...)
		buf = append(buf, fmt.Sprintf("\033[38;2;%d;%d;%dm", brandR, brandG, brandB)...)
		buf = append(buf, ansiBold...)
		buf = append(buf, logo...)
		buf = append(buf, ansiReset...)

		buf = append(buf, fmt.Sprintf(ansiCursorPos, tagRow, tagCol)...)
		buf = append(buf, dimTagFg...)
		buf = append(buf, tagline...)
		buf = append(buf, ansiReset...)

		intro.frames = append(intro.frames, buf)
	}

	fullTagFg := fmt.Sprintf("\033[38;2;%d;%d;%dm", highlightR, highlightG, highlightB)
	for hold := 0; hold < 3; hold++ {
		var buf []byte
		buf = append(buf, clear...)
		buf = append(buf, fmt.Sprintf(ansiCursorPos, centerRow, logoCol)...)
		buf = append(buf, fmt.Sprintf("\033[38;2;%d;%d;%dm", brandR, brandG, brandB)...)
		buf = append(buf, ansiBold...)
		buf = append(buf, logo...)
		buf = append(buf, ansiReset...)

		buf = append(buf, fmt.Sprintf(ansiCursorPos, tagRow, tagCol)...)
		buf = append(buf, fullTagFg...)
		buf = append(buf, ansiDim...)
		buf = append(buf, tagline...)
		buf = append(buf, ansiReset...)

		intro.frames = append(intro.frames, buf)
	}
}
