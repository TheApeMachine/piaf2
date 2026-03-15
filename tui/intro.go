package tui

import (
	"fmt"
	"io"
	"math"
	"time"
)

const introFrameDelay = 40 * time.Millisecond

/*
Intro renders a branded splash animation with particle trails, gradient sweeps,
and glow pulses on startup.
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

func introLerp(a, b int, t float64) int {
	return a + int(float64(b-a)*t)
}

func introClamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}

	if v > hi {
		return hi
	}

	return v
}

func (intro *Intro) generateFrames() {
	if intro.width < 20 || intro.height < 8 {
		return
	}

	logo := "piaf"
	tagline := "A.I. Code Editor"

	centerRow := intro.height / 2
	logoCol := (intro.width-len(logo)*3)/2 + 1
	tagRow := centerRow + 3
	tagCol := (intro.width-len(tagline))/2 + 1

	brandR, brandG, brandB := 108, 80, 255
	highR, highG, highB := 254, 135, 255

	sparkChars := []byte{'*', '.', '+', '~', '`'}
	totalFrames := 32

	clear := ansiEnterAlternate + ansiHideCursor + ansiCursorHome + ansiClearDown

	for frameIdx := 0; frameIdx < totalFrames; frameIdx++ {
		var buf []byte
		buf = append(buf, clear...)
		progress := float64(frameIdx) / float64(totalFrames-1)

		for row := 1; row <= intro.height; row++ {
			dy := float64(row-centerRow) / float64(intro.height)
			brightness := math.Exp(-8 * dy * dy * (1.2 - progress*0.7))

			bgR := introClamp(int(float64(brandR)*brightness*0.12), 0, 255)
			bgG := introClamp(int(float64(brandG)*brightness*0.12), 0, 255)
			bgB := introClamp(int(float64(brandB)*brightness*0.12), 0, 255)

			buf = append(buf, fmt.Sprintf(ansiCursorPos, row, 1)...)
			buf = append(buf, fmt.Sprintf("\033[48;2;%d;%d;%dm", bgR, bgG, bgB)...)
			buf = append(buf, fmt.Sprintf("%*s", intro.width, "")...)
			buf = append(buf, ansiReset...)
		}

		particleCount := int(progress * 12)
		for particleIdx := 0; particleIdx < particleCount; particleIdx++ {
			seed := uint32(frameIdx*97 + particleIdx*31)
			seed = seed ^ (seed << 13)
			seed = seed ^ (seed >> 17)
			seed = seed ^ (seed << 5)

			angle := float64(seed%360) * math.Pi / 180.0
			dist := 3.0 + float64(seed%uint32(intro.width/3))
			particleRow := centerRow + int(math.Sin(angle)*dist*0.4)
			particleCol := intro.width/2 + int(math.Cos(angle)*dist)

			if particleRow < 1 || particleRow > intro.height || particleCol < 1 || particleCol > intro.width {
				continue
			}

			fade := 1.0 - float64(particleIdx)/float64(particleCount+1)
			sparkR := introClamp(introLerp(brandR, highR, fade), 0, 255)
			sparkG := introClamp(introLerp(brandG, highG, fade), 0, 255)
			sparkB := introClamp(introLerp(brandB, highB, fade), 0, 255)

			buf = append(buf, fmt.Sprintf(ansiCursorPos, particleRow, particleCol)...)
			buf = append(buf, fmt.Sprintf("\033[38;2;%d;%d;%dm", sparkR, sparkG, sparkB)...)
			buf = append(buf, sparkChars[seed%uint32(len(sparkChars))])
			buf = append(buf, ansiReset...)
		}

		revealCount := int(float64(len(logo)) * progress)
		sweepPos := progress * float64(len(logo)+2) // sweep cursor position

		if revealCount > 0 || progress > 0.1 {
			buf = append(buf, fmt.Sprintf(ansiCursorPos, centerRow, logoCol)...)

			for charIdx := 0; charIdx < len(logo); charIdx++ {
				charProgress := float64(charIdx) / float64(len(logo))

				if charProgress > progress+0.05 {
					buf = append(buf, ' ', ' ', ' ')
					continue
				}

				dist := math.Abs(float64(charIdx) - sweepPos)
				glow := math.Exp(-dist * dist * 0.5)
				charT := math.Min(progress*float64(len(logo))-float64(charIdx), 1.0)

				if charT < 0 {
					charT = 0
				}

				letterR := introLerp(highR, brandR, charT*(1-glow*0.5))
				letterG := introLerp(highG, brandG, charT*(1-glow*0.5))
				letterB := introLerp(highB, brandB, charT*(1-glow*0.5))

				glowBgR := introClamp(int(float64(brandR)*glow*0.3), 0, 80)
				glowBgG := introClamp(int(float64(brandG)*glow*0.3), 0, 60)
				glowBgB := introClamp(int(float64(brandB)*glow*0.3), 0, 120)

				buf = append(buf, fmt.Sprintf("\033[48;2;%d;%d;%dm", glowBgR, glowBgG, glowBgB)...)
				buf = append(buf, fmt.Sprintf("\033[38;2;%d;%d;%dm", letterR, letterG, letterB)...)
				buf = append(buf, ansiBold...)
				buf = append(buf, ' ')
				buf = append(buf, logo[charIdx])
				buf = append(buf, ' ')
				buf = append(buf, ansiReset...)
			}
		}

		if progress > 0.5 {
			tagProgress := (progress - 0.5) * 2.0
			tagReveal := int(float64(len(tagline)) * tagProgress)

			if tagReveal > len(tagline) {
				tagReveal = len(tagline)
			}

			if tagReveal > 0 {
				buf = append(buf, fmt.Sprintf(ansiCursorPos, tagRow, tagCol)...)

				for tagIdx := 0; tagIdx < tagReveal; tagIdx++ {
					charFade := float64(tagIdx) / float64(len(tagline))
					tagCharR := introLerp(highR, highR/2, charFade)
					tagCharG := introLerp(highG, highG/2, charFade)
					tagCharB := introLerp(highB, highB/2, charFade)

					buf = append(buf, fmt.Sprintf("\033[38;2;%d;%d;%dm", tagCharR, tagCharG, tagCharB)...)
					buf = append(buf, tagline[tagIdx])
				}

				buf = append(buf, ansiReset...)
			}
		}

		if progress > 0.25 {
			lineProgress := math.Min((progress-0.25)/0.5, 1.0)
			lineWidth := int(float64(intro.width/3) * lineProgress)
			lineStart := intro.width/2 - lineWidth/2

			if lineStart < 1 {
				lineStart = 1
			}

			buf = append(buf, fmt.Sprintf(ansiCursorPos, centerRow+1, lineStart)...)

			for lineIdx := 0; lineIdx < lineWidth; lineIdx++ {
				lineT := float64(lineIdx) / float64(lineWidth+1)
				lineR := introLerp(brandR, highR, lineT)
				lineG := introLerp(brandG, highG, lineT)
				lineB := introLerp(brandB, highB, lineT)
				bright := 0.3 + 0.7*math.Sin(lineT*math.Pi)

				lineR = int(float64(lineR) * bright)
				lineG = int(float64(lineG) * bright)
				lineB = int(float64(lineB) * bright)

				buf = append(buf, fmt.Sprintf("\033[38;2;%d;%d;%dm", lineR, lineG, lineB)...)
				buf = append(buf, "\u2500"...)
			}

			buf = append(buf, ansiReset...)
		}

		intro.frames = append(intro.frames, buf)
	}

	for holdIdx := 0; holdIdx < 6; holdIdx++ {
		var buf []byte
		buf = append(buf, clear...)

		pulse := 0.8 + 0.2*math.Sin(float64(holdIdx)*math.Pi/3)

		for row := 1; row <= intro.height; row++ {
			dy := float64(row-centerRow) / float64(intro.height)
			brightness := math.Exp(-6 * dy * dy)

			bgR := introClamp(int(float64(brandR)*brightness*0.12*pulse), 0, 255)
			bgG := introClamp(int(float64(brandG)*brightness*0.12*pulse), 0, 255)
			bgB := introClamp(int(float64(brandB)*brightness*0.12*pulse), 0, 255)

			buf = append(buf, fmt.Sprintf(ansiCursorPos, row, 1)...)
			buf = append(buf, fmt.Sprintf("\033[48;2;%d;%d;%dm", bgR, bgG, bgB)...)
			buf = append(buf, fmt.Sprintf("%*s", intro.width, "")...)
			buf = append(buf, ansiReset...)
		}

		buf = append(buf, fmt.Sprintf(ansiCursorPos, centerRow, logoCol)...)

		for charIdx := 0; charIdx < len(logo); charIdx++ {
			letterR := introClamp(int(float64(brandR)*pulse), 0, 255)
			letterG := introClamp(int(float64(brandG)*pulse), 0, 255)
			letterB := introClamp(int(float64(brandB)*pulse), 0, 255)

			buf = append(buf, fmt.Sprintf("\033[38;2;%d;%d;%dm", letterR, letterG, letterB)...)
			buf = append(buf, ansiBold...)
			buf = append(buf, ' ')
			buf = append(buf, logo[charIdx])
			buf = append(buf, ' ')
			buf = append(buf, ansiReset...)
		}

		lineWidth := intro.width / 3
		lineStart := intro.width/2 - lineWidth/2

		if lineStart < 1 {
			lineStart = 1
		}

		buf = append(buf, fmt.Sprintf(ansiCursorPos, centerRow+1, lineStart)...)

		for lineIdx := 0; lineIdx < lineWidth; lineIdx++ {
			lineT := float64(lineIdx) / float64(lineWidth+1)
			lineR := introLerp(brandR, highR, lineT)
			lineG := introLerp(brandG, highG, lineT)
			lineB := introLerp(brandB, highB, lineT)

			buf = append(buf, fmt.Sprintf("\033[38;2;%d;%d;%dm", lineR, lineG, lineB)...)
			buf = append(buf, "\u2500"...)
		}

		buf = append(buf, ansiReset...)

		buf = append(buf, fmt.Sprintf(ansiCursorPos, tagRow, tagCol)...)
		buf = append(buf, fmt.Sprintf("\033[38;2;%d;%d;%dm", highR/2, highG/2, highB/2)...)
		buf = append(buf, tagline...)
		buf = append(buf, ansiReset...)

		intro.frames = append(intro.frames, buf)
	}
}
