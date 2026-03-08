package editor

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestChatSubmit(t *testing.T) {
	convey.Convey("Given a Chat scoped to a workspace", t, func() {
		root := t.TempDir()
		os.WriteFile(filepath.Join(root, "note.txt"), []byte("hello\nworld"), 0o644)
		os.Mkdir(filepath.Join(root, "docs"), 0o755)

		chat := NewChat(
			ChatWithRoot(root),
			ChatWithRandom(rand.New(rand.NewSource(7))),
		)

		convey.Convey("When a browse prompt is submitted", func() {
			chat.Submit("browse .")

			convey.Convey("It should add a full three-model pipeline to the transcript", func() {
				transcript := strings.Join(chat.history, "\n")

				convey.So(chat.history, convey.ShouldHaveLength, 5)
				convey.So(transcript, convey.ShouldContainSubstring, "Pipeline:")
				convey.So(transcript, convey.ShouldContainSubstring, "OpenAI GPT-5.4")
				convey.So(transcript, convey.ShouldContainSubstring, "Claude Open 4.6")
				convey.So(transcript, convey.ShouldContainSubstring, "Gemini Pro 3.1")
				convey.So(transcript, convey.ShouldContainSubstring, "Tool browse .")
				convey.So(transcript, convey.ShouldContainSubstring, "- docs/")
				convey.So(transcript, convey.ShouldContainSubstring, "- note.txt")
			})
		})

		convey.Convey("When a read prompt targets a file outside the workspace", func() {
			outside := filepath.Join(filepath.Dir(root), "outside.txt")
			os.WriteFile(outside, []byte("secret"), 0o644)

			chat.Submit("read ../outside.txt")

			convey.Convey("It should block the tool access", func() {
				transcript := strings.Join(chat.history, "\n")
				convey.So(transcript, convey.ShouldContainSubstring, "Tool read blocked")
			})
		})
	})
}

func TestChatImplementMode(t *testing.T) {
	convey.Convey("Given a Chat in implementation mode", t, func() {
		chat := NewChat(ChatWithRandom(rand.New(rand.NewSource(11))))
		chat.SetMode("IMPLEMENT")

		convey.Convey("When an implementation prompt is submitted", func() {
			chat.Submit("add a command palette")

			convey.Convey("It should end with accept and reject guidance", func() {
				transcript := strings.Join(chat.history, "\n")
				convey.So(transcript, convey.ShouldContainSubstring, "Accept with :accept or :reject.")
			})
		})
	})
}

func BenchmarkChatSubmit(b *testing.B) {
	root := b.TempDir()
	os.WriteFile(filepath.Join(root, "note.txt"), []byte("hello\nworld"), 0o644)

	chat := NewChat(
		ChatWithRoot(root),
		ChatWithRandom(rand.New(rand.NewSource(13))),
	)

	for b.Loop() {
		chat.history = nil
		chat.Submit("read note.txt")
	}
}
