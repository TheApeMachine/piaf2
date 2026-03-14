package cmd

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/smartystreets/goconvey/convey"
)

func TestTcHeader(t *testing.T) {
	convey.Convey("Given tcHeader", t, func() {

		convey.Convey("When rendering a header", func() {
			header := tcHeader("What is consciousness?", []string{"philosopher", "scientist"}, time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC))

			convey.Convey("It should contain the branded title", func() {
				convey.So(header, convey.ShouldContainSubstring, "thinkcursion")
			})

			convey.Convey("It should contain the prompt", func() {
				convey.So(header, convey.ShouldContainSubstring, "What is consciousness?")
			})

			convey.Convey("It should contain persona names", func() {
				convey.So(header, convey.ShouldContainSubstring, "philosopher")
				convey.So(header, convey.ShouldContainSubstring, "scientist")
			})

			convey.Convey("It should contain ANSI styling", func() {
				convey.So(header, convey.ShouldContainSubstring, tcFgHigh)
				convey.So(header, convey.ShouldContainSubstring, tcReset)
			})

			convey.Convey("It should contain separator dashes", func() {
				convey.So(header, convey.ShouldContainSubstring, tcDash)
			})
		})
	})
}

func TestTcRoundBanner(t *testing.T) {
	convey.Convey("Given tcRoundBanner", t, func() {

		convey.Convey("When rendering round 1", func() {
			banner := tcRoundBanner(1)

			convey.Convey("It should contain the round number", func() {
				convey.So(banner, convey.ShouldContainSubstring, "round 1")
			})

			convey.Convey("It should contain brand color", func() {
				convey.So(banner, convey.ShouldContainSubstring, tcFgBrand)
			})

			convey.Convey("It should contain separator dashes", func() {
				convey.So(banner, convey.ShouldContainSubstring, tcDash)
			})
		})

		convey.Convey("When rendering round 42", func() {
			banner := tcRoundBanner(42)

			convey.Convey("It should contain the round number", func() {
				convey.So(banner, convey.ShouldContainSubstring, "round 42")
			})
		})
	})
}

func TestTcPersonaTurn(t *testing.T) {
	convey.Convey("Given tcPersonaTurn", t, func() {

		convey.Convey("When rendering a persona name", func() {
			turn := tcPersonaTurn("philosopher")

			convey.Convey("It should contain the persona name", func() {
				convey.So(turn, convey.ShouldContainSubstring, "philosopher")
			})

			convey.Convey("It should use highlight color for the label", func() {
				convey.So(turn, convey.ShouldContainSubstring, tcFgHigh)
				convey.So(turn, convey.ShouldContainSubstring, tcBold)
			})
		})
	})
}

func TestTcResponse(t *testing.T) {
	convey.Convey("Given tcResponse", t, func() {

		convey.Convey("When rendering a response", func() {
			resp := tcResponse("philosopher", "I think, therefore I am.")

			convey.Convey("It should contain the response text", func() {
				convey.So(resp, convey.ShouldContainSubstring, "I think, therefore I am.")
			})

			convey.Convey("It should end with reset and newline", func() {
				convey.So(resp, convey.ShouldEndWith, tcReset+"\n")
			})
		})
	})
}

func TestTcToolCall(t *testing.T) {
	convey.Convey("Given tcToolCall", t, func() {

		convey.Convey("When rendering a tool call", func() {
			call := tcToolCall("philosopher", "read", map[string]any{"file": "notes.md"})

			convey.Convey("It should contain the persona name", func() {
				convey.So(call, convey.ShouldContainSubstring, "philosopher")
			})

			convey.Convey("It should contain the tool name in cyan", func() {
				convey.So(call, convey.ShouldContainSubstring, tcFgCyan+"read")
			})

			convey.Convey("It should contain the arguments", func() {
				convey.So(call, convey.ShouldContainSubstring, "file=notes.md")
			})

			convey.Convey("It should use dim styling", func() {
				convey.So(call, convey.ShouldContainSubstring, tcDim)
			})
		})
	})
}

func TestTcError(t *testing.T) {
	convey.Convey("Given tcError", t, func() {

		convey.Convey("When rendering an error", func() {
			errMsg := tcError("scientist", errors.New("timeout"))

			convey.Convey("It should contain the persona name", func() {
				convey.So(errMsg, convey.ShouldContainSubstring, "scientist")
			})

			convey.Convey("It should contain the error text", func() {
				convey.So(errMsg, convey.ShouldContainSubstring, "timeout")
			})

			convey.Convey("It should use yellow dim styling", func() {
				convey.So(errMsg, convey.ShouldStartWith, tcFgYellow)
			})
		})
	})
}

func TestTcEnded(t *testing.T) {
	convey.Convey("Given tcEnded", t, func() {

		convey.Convey("When rendering the end marker", func() {
			ended := tcEnded(5)

			convey.Convey("It should contain the round number", func() {
				convey.So(ended, convey.ShouldContainSubstring, "round 5")
			})

			convey.Convey("It should contain separator dashes", func() {
				convey.So(strings.Count(ended, tcDash), convey.ShouldBeGreaterThan, 10)
			})

			convey.Convey("It should use highlight for the label", func() {
				convey.So(ended, convey.ShouldContainSubstring, tcFgHigh)
			})
		})
	})
}

func BenchmarkTcHeader(b *testing.B) {
	names := []string{"philosopher", "scientist", "mathematician"}
	now := time.Now()

	for b.Loop() {
		tcHeader("benchmark prompt", names, now)
	}
}

func BenchmarkTcRoundBanner(b *testing.B) {
	for b.Loop() {
		tcRoundBanner(42)
	}
}

func BenchmarkTcPersonaTurn(b *testing.B) {
	for b.Loop() {
		tcPersonaTurn("philosopher")
	}
}

func BenchmarkTcToolCall(b *testing.B) {
	args := map[string]any{"file": "main.go", "start": 1, "end": 50}

	for b.Loop() {
		tcToolCall("philosopher", "read", args)
	}
}
