package cmd

import (
	"bytes"
	"io"
	"os"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
	"github.com/theapemachine/piaf/core"
	"github.com/theapemachine/piaf/editor"
	"github.com/theapemachine/piaf/tui"
)

/*
Root command definition.
*/
var rootCmd = &cobra.Command{
	Use:   "piaf",
	Short: "Piaf is an A.I. powered code editor.",
	Long:  rootLong,
	Run: func(cmd *cobra.Command, args []string) {
		if !term.IsTerminal(os.Stdin.Fd()) {
			return
		}

		oldState, err := term.MakeRaw(os.Stdin.Fd())
		if err != nil {
			return
		}
		defer term.Restore(os.Stdin.Fd(), oldState)

		width, height, err := term.GetSize(os.Stdout.Fd())
		if err != nil {
			width, height = 80, 24
		}

		path := "."
		if len(args) > 0 && args[0] != "." {
			path = args[0]
		}

		config, _ := core.Load()
		systemPrompt := ""
		var chatTimeout time.Duration
		var chatDumpPath string
		if config != nil {
			systemPrompt = config.AI.Persona.Research.Manager
			if config.AI.Chat.TimeoutSeconds > 0 {
				chatTimeout = time.Duration(config.AI.Chat.TimeoutSeconds) * time.Second
			}
			chatDumpPath = config.AI.Chat.DumpFile
		}

		streamCh := make(chan struct{}, 16)
		quitRead, quitWrite := io.Pipe()
		opts := []editor.EditorOpt{
			editor.EditorWithSize(width, height),
			editor.EditorWithPath(path),
			editor.EditorWithStreamUpdates(streamCh),
			editor.EditorWithSystemPrompt(systemPrompt),
		}
		if chatTimeout > 0 {
			opts = append(opts, editor.EditorWithChatTimeout(chatTimeout))
		}
		if chatDumpPath != "" {
			opts = append(opts, editor.EditorWithChatDumpPath(chatDumpPath))
		}
		ed := editor.NewEditor(opts...)
		mux := tui.NewInputMux(
			tui.InputMuxWithStdin(os.Stdin),
			tui.InputMuxWithRefresh(streamCh),
			tui.InputMuxWithQuit(quitRead),
		)
		app := tui.NewApp(tui.AppWithEditor(ed), tui.AppWithQuitWriter(quitWrite))
		defer app.Close()

		buf := make([]byte, 256)
		io.Copy(os.Stdout, app)

		for {
			count, readErr := mux.Read(buf)
			if readErr != nil {
				return
			}
			if count > 0 {
				if buf[0] == tui.SentinelQuit {
					return
				}
				if buf[0] != tui.SentinelRefresh && bytes.Contains(buf[:count], []byte{0x03}) {
					return
				}
				app.Write(buf[:count])
			}

			io.Copy(os.Stdout, app)

			if app.Closed() {
				return
			}
		}
	},
}

/*
Executes the root command.
*/
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

const rootLong = `
Piaf is an A.I. powered code editor.
`
