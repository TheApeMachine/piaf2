package cmd

import (
	"io"
	"os"

	"github.com/spf13/cobra"
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
		app := tui.NewApp()
		defer app.Close()

		buf := make([]byte, 256)

		io.Copy(os.Stdout, app)

		for {
			count, readErr := os.Stdin.Read(buf)

			if count > 0 {
				app.Write(buf[:count])
				io.Copy(os.Stdout, app)
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
