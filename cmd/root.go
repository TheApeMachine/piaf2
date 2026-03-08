package cmd

import (
	"bytes"
	"io"
	"os"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
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

		ed := editor.NewEditor(
			editor.EditorWithSize(width, height),
			editor.EditorWithPath(path),
		)
		app := tui.NewApp(tui.AppWithEditor(ed))
		defer app.Close()

		buf := make([]byte, 256)

		io.Copy(os.Stdout, app)

		for {
			count, readErr := os.Stdin.Read(buf)

			if count > 0 {
				if bytes.Contains(buf[:count], []byte{0x03}) {
					break
				}

				app.Write(buf[:count])
				io.Copy(os.Stdout, app)

				if app.QuitRequested() {
					break
				}
			}

			if readErr != nil {
				break
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
