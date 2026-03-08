package cmd

import (
	"bytes"
	"embed"
	"io"
	"os"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
	"github.com/theapemachine/piaf/editor"
	"github.com/theapemachine/piaf/tui"
)

/*
Embed a mini filesystem into the binary to hold the default config file.
This will be written to the home directory of the user running the service,
which allows a developer to easily override the config file.
*/
//go:embed cfg/*
var embedded embed.FS

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

		config, _ := Load(embedded)
		systemPrompt := ""
		if config != nil {
			systemPrompt = config.AI.Persona.Research.Manager
		}

		streamCh := make(chan struct{}, 16)
		quitRead, quitWrite := io.Pipe()
		ed := editor.NewEditor(
			editor.EditorWithSize(width, height),
			editor.EditorWithPath(path),
			editor.EditorWithStreamUpdates(streamCh),
			editor.EditorWithSystemPrompt(systemPrompt),
		)
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
