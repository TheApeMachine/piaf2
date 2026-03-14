package tui

import (
	"io"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestNewIntro(t *testing.T) {
	convey.Convey("Given NewIntro", t, func() {
		convey.Convey("When called with standard dimensions", func() {
			intro := NewIntro(80, 24)

			convey.Convey("It should return a non-nil Intro", func() {
				convey.So(intro, convey.ShouldNotBeNil)
			})

			convey.Convey("It should generate animation frames", func() {
				convey.So(len(intro.frames), convey.ShouldBeGreaterThan, 0)
			})
		})

		convey.Convey("When called with a tiny terminal", func() {
			intro := NewIntro(10, 4)

			convey.Convey("It should generate no frames", func() {
				convey.So(len(intro.frames), convey.ShouldEqual, 0)
			})
		})
	})
}

func TestIntroRead(t *testing.T) {
	convey.Convey("Given an Intro with standard dimensions", t, func() {
		intro := NewIntro(80, 24)

		convey.Convey("When Read is called", func() {
			buf := make([]byte, 8192)
			total := 0

			for {
				number, err := intro.Read(buf[total:])
				total += number

				if err == io.EOF {
					break
				}
			}

			convey.Convey("It should produce ANSI output containing the logo", func() {
				output := string(buf[:total])
				convey.So(total, convey.ShouldBeGreaterThan, 0)
				convey.So(output, convey.ShouldContainSubstring, "piaf")
				convey.So(output, convey.ShouldContainSubstring, "A.I. Code Editor")
			})
		})
	})

	convey.Convey("Given a tiny Intro with no frames", t, func() {
		intro := NewIntro(10, 4)

		convey.Convey("When Read is called", func() {
			buf := make([]byte, 256)
			number, err := intro.Read(buf)

			convey.Convey("It should return 0 and EOF immediately", func() {
				convey.So(number, convey.ShouldEqual, 0)
				convey.So(err, convey.ShouldEqual, io.EOF)
			})
		})
	})
}

func TestIntroWrite(t *testing.T) {
	convey.Convey("Given an Intro", t, func() {
		intro := NewIntro(80, 24)

		convey.Convey("When Write is called", func() {
			number, err := intro.Write([]byte("ignored"))

			convey.Convey("It should discard input and return len", func() {
				convey.So(number, convey.ShouldEqual, 7)
				convey.So(err, convey.ShouldBeNil)
			})
		})
	})
}

func TestIntroClose(t *testing.T) {
	convey.Convey("Given an Intro", t, func() {
		intro := NewIntro(80, 24)

		convey.Convey("When Close is called", func() {
			err := intro.Close()

			convey.Convey("It should return nil", func() {
				convey.So(err, convey.ShouldBeNil)
			})
		})
	})
}

func BenchmarkIntroRead(b *testing.B) {
	intro := NewIntro(80, 24)
	buf := make([]byte, 8192)

	for b.Loop() {
		intro.frameIndex = 0
		intro.readOff = 0

		for {
			_, err := intro.Read(buf)
			if err == io.EOF {
				break
			}
		}
	}
}
