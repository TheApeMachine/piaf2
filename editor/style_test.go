package editor

import (
	"strings"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestStyleChatLines(t *testing.T) {
	convey.Convey("Given styleChatLines", t, func() {

		convey.Convey("When styling user messages", func() {
			lines := styleChatLines([]string{"You: hello"}, 80)

			convey.Convey("It should wrap in bold brand", func() {
				convey.So(lines[0], convey.ShouldStartWith, styleBold+styleFgBrand)
				convey.So(lines[0], convey.ShouldEndWith, styleReset)
				convey.So(lines[0], convey.ShouldContainSubstring, "You: hello")
			})
		})

		convey.Convey("When styling system messages", func() {
			lines := styleChatLines([]string{"System: engaged."}, 80)

			convey.Convey("It should wrap in yellow", func() {
				convey.So(lines[0], convey.ShouldStartWith, styleFgYellow)
				convey.So(lines[0], convey.ShouldEndWith, styleReset)
			})
		})

		convey.Convey("When styling pipeline info", func() {
			lines := styleChatLines([]string{"Pipeline: A -> B -> C"}, 80)

			convey.Convey("It should wrap in dim highlight", func() {
				convey.So(lines[0], convey.ShouldStartWith, styleDim+styleFgHighlight)
				convey.So(lines[0], convey.ShouldEndWith, styleReset)
			})
		})

		convey.Convey("When styling AI role labels", func() {
			lines := styleChatLines([]string{"Discussion OpenAI: response text"}, 80)

			convey.Convey("It should color the label and reset for the body", func() {
				convey.So(lines[0], convey.ShouldStartWith, styleBold+styleFgBrand)
				convey.So(lines[0], convey.ShouldContainSubstring, styleReset+" response text")
			})
		})

		convey.Convey("When styling separator lines", func() {
			lines := styleChatLines([]string{"---"}, 60)

			convey.Convey("It should render a horizontal rule", func() {
				convey.So(lines[0], convey.ShouldStartWith, styleFgBrand+styleDim)
				convey.So(lines[0], convey.ShouldContainSubstring, separatorChar)
				convey.So(lines[0], convey.ShouldEndWith, styleReset)
			})
		})

		convey.Convey("When styling progress reports", func() {
			lines := styleChatLines([]string{"Progress: done"}, 80)

			convey.Convey("It should wrap in green", func() {
				convey.So(lines[0], convey.ShouldStartWith, styleFgGreen)
				convey.So(lines[0], convey.ShouldEndWith, styleReset)
			})
		})

		convey.Convey("When styling plain content", func() {
			lines := styleChatLines([]string{"just regular text"}, 80)

			convey.Convey("It should pass through unchanged", func() {
				convey.So(lines[0], convey.ShouldEqual, "just regular text")
			})
		})

		convey.Convey("When styling empty lines", func() {
			lines := styleChatLines([]string{""}, 80)

			convey.Convey("It should pass through unchanged", func() {
				convey.So(lines[0], convey.ShouldEqual, "")
			})
		})

		convey.Convey("When styling welcome text", func() {
			lines := styleChatLines([]string{"Discussion window ready."}, 80)

			convey.Convey("It should wrap in dim", func() {
				convey.So(lines[0], convey.ShouldStartWith, styleDim)
				convey.So(lines[0], convey.ShouldEndWith, styleReset)
			})
		})

		convey.Convey("When styling implementation complete", func() {
			lines := styleChatLines([]string{"Implementation complete. Review the summary."}, 80)

			convey.Convey("It should wrap in bold green", func() {
				convey.So(lines[0], convey.ShouldStartWith, styleBold+styleFgGreen)
				convey.So(lines[0], convey.ShouldEndWith, styleReset)
			})
		})

		convey.Convey("When styling multiple roles", func() {
			input := []string{
				"Project Manager [Claude]: board ready",
				"Architect [OpenAI]: plan ready",
				"Team Lead [Gemini]: team assigned",
				"Developer 1 [Claude]: code done",
				"QA [OpenAI]: Decision: PASS",
				"Review [Gemini]: accept",
			}
			lines := styleChatLines(input, 80)

			convey.Convey("It should apply distinct colors per role", func() {
				convey.So(lines[0], convey.ShouldContainSubstring, styleFgBrand)
				convey.So(lines[1], convey.ShouldContainSubstring, styleFgHighlight)
				convey.So(lines[2], convey.ShouldContainSubstring, styleFgBrand)
				convey.So(lines[3], convey.ShouldContainSubstring, styleFgGreen)
				convey.So(lines[4], convey.ShouldContainSubstring, styleFgYellow)
				convey.So(lines[5], convey.ShouldContainSubstring, styleFgHighlight)
			})
		})
	})
}

func TestStyleExplorerLines(t *testing.T) {
	convey.Convey("Given styleExplorerLines", t, func() {

		convey.Convey("When styling directory entries", func() {
			lines := styleExplorerLines([]string{"docs/", "src/", "main.go", ".."})

			convey.Convey("It should color directories bold brand", func() {
				convey.So(lines[0], convey.ShouldStartWith, styleBold+styleFgBrand)
				convey.So(lines[0], convey.ShouldEndWith, styleReset)
				convey.So(lines[1], convey.ShouldStartWith, styleBold+styleFgBrand)
			})

			convey.Convey("It should leave regular files unchanged", func() {
				convey.So(lines[2], convey.ShouldEqual, "main.go")
			})

			convey.Convey("It should dim the parent entry", func() {
				convey.So(lines[3], convey.ShouldStartWith, styleDim)
				convey.So(lines[3], convey.ShouldEndWith, styleReset)
			})
		})
	})
}

func TestStyleCodeLines(t *testing.T) {
	convey.Convey("Given styleCodeLines", t, func() {

		convey.Convey("When styling Go source", func() {
			lines := styleCodeLines([]string{
				"func main() {",
				"\tmessage := \"hello\"",
				"\treturn 42 // answer",
				"\tvalue := /* inline */ 7",
				"}",
			}, "main.go")

			convey.Convey("It should highlight keywords, strings, numbers and comments", func() {
				convey.So(lines[0], convey.ShouldContainSubstring, styleBold+styleFgMagenta+"func"+styleReset)
				convey.So(lines[1], convey.ShouldContainSubstring, styleFgGreen+"\"hello\""+styleReset)
				convey.So(lines[2], convey.ShouldContainSubstring, styleBold+styleFgMagenta+"return"+styleReset)
				convey.So(lines[2], convey.ShouldContainSubstring, styleFgYellow+"42"+styleReset)
				convey.So(lines[2], convey.ShouldContainSubstring, styleDim+styleFgGray+"// answer"+styleReset)
				convey.So(lines[3], convey.ShouldContainSubstring, styleDim+styleFgGray+"/* inline */"+styleReset)
				convey.So(lines[3], convey.ShouldContainSubstring, styleFgYellow+"7"+styleReset)
			})
		})

		convey.Convey("When styling JSON content", func() {
			lines := styleCodeLines([]string{`{"enabled": true, "count": 3}`}, "config.json")

			convey.Convey("It should highlight strings, literals and numbers", func() {
				convey.So(lines[0], convey.ShouldContainSubstring, styleFgGreen+`"enabled"`+styleReset)
				convey.So(lines[0], convey.ShouldContainSubstring, styleFgYellow+"true"+styleReset)
				convey.So(lines[0], convey.ShouldContainSubstring, styleFgYellow+"3"+styleReset)
			})
		})

		convey.Convey("When styling an unsupported file", func() {
			lines := styleCodeLines([]string{"plain text only"}, "README.md")

			convey.Convey("It should leave the content unchanged", func() {
				convey.So(lines[0], convey.ShouldEqual, "plain text only")
			})
		})
	})
}

func TestStyleCodeLine(t *testing.T) {
	convey.Convey("Given styleCodeLine", t, func() {

		convey.Convey("When a number is followed by identifier text", func() {
			line := styleCodeLine("value := 42abc", &goSyntaxSpec)

			convey.Convey("It should only highlight the numeric portion", func() {
				convey.So(line, convey.ShouldContainSubstring, styleFgYellow+"42"+styleReset+"abc")
			})
		})
	})
}

func TestStyleChatLineSeparatorWidth(t *testing.T) {
	convey.Convey("Given styleChatLine with a separator", t, func() {

		convey.Convey("When width is specified", func() {
			line := styleChatLine("---", 50)

			convey.Convey("It should produce a separator of that width", func() {
				inner := strings.TrimPrefix(line, styleFgBrand+styleDim)
				inner = strings.TrimSuffix(inner, styleReset)
				convey.So(len([]rune(inner)), convey.ShouldEqual, 50)
			})
		})

		convey.Convey("When width is zero", func() {
			line := styleChatLine("---", 0)

			convey.Convey("It should fall back to 40 characters", func() {
				inner := strings.TrimPrefix(line, styleFgBrand+styleDim)
				inner = strings.TrimSuffix(inner, styleReset)
				convey.So(len([]rune(inner)), convey.ShouldEqual, 40)
			})
		})
	})
}

func BenchmarkStyleChatLines(b *testing.B) {
	lines := []string{
		"You: hello world",
		"System: engaged.",
		"Pipeline: A -> B -> C",
		"Discussion OpenAI: response text goes here and is moderately long",
		"",
		"Developer 1 [Claude]: implementation done",
		"---",
		"Progress: step complete",
		"just some regular text continuation",
	}

	for index := 0; index < b.N; index++ {
		styleChatLines(lines, 80)
	}
}

func BenchmarkStyleExplorerLines(b *testing.B) {
	lines := []string{"..", "cmd/", "editor/", "tui/", "main.go", "go.mod", "README.md"}

	for index := 0; index < b.N; index++ {
		styleExplorerLines(lines)
	}
}

func BenchmarkStyleCodeLines(b *testing.B) {
	lines := []string{
		"package main",
		"",
		"import \"fmt\"",
		"",
		"func main() {",
		"\tmessage := \"hello\"",
		"\tfmt.Println(message, 42)",
		"}",
	}

	for index := 0; index < b.N; index++ {
		styleCodeLines(lines, "main.go")
	}
}
