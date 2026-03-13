package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/theapemachine/piaf/core"
	"github.com/theapemachine/piaf/errnie"
	"github.com/theapemachine/piaf/provider"
)

/*
thinkcursionCmd represents the thinkcursion command.
It loads personas from config, takes a prompt via args, and loops them
in an infinite round-robin discussion until interrupted.
*/
var thinkcursionCmd = &cobra.Command{
	Use:   "thinkcursion [prompt]",
	Short: "Recursive multi-persona discussion",
	Long:  thinkcursionLong,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt := strings.Join(args, " ")

		config, err := core.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		personas := config.AI.Thinkcursion.Personas
		if len(personas) == 0 {
			return fmt.Errorf("no personas defined in ai.thinkcursion.personas")
		}

		outPath, _ := cmd.Flags().GetString("output")
		outPath = expandHome(outPath)

		if dir := filepath.Dir(outPath); dir != "." {
			os.MkdirAll(dir, 0755)
		}

		outFile, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("opening output file: %w", err)
		}
		defer outFile.Close()

		names := sortedPersonaNames(personas)
		agents := buildAgents(names, personas)

		root, _ := os.Getwd()
		if root == "" {
			root = "."
		}
		backend := &thinkcursionBackend{root: root}
		baseExecutor := provider.WithToolLimit(provider.NewDiscussionToolExecutor(backend), 8)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

		go func() {
			<-sigCh
			errnie.Info("interrupted, finishing current turn…")
			cancel()
		}()

		header := fmt.Sprintf("=== Thinkcursion ===\nPrompt: %s\nPersonas: %s\nStarted: %s\n\n",
			prompt, strings.Join(names, ", "), time.Now().Format(time.RFC3339))
		outFile.WriteString(header)
		fmt.Print(header)

		transcript := []string{"[User] " + prompt}
		round := 0

		for {
			round++
			errnie.Info("round", "n", round)

			for _, agent := range agents {
				if ctx.Err() != nil {
					fmt.Fprintf(outFile, "\n=== Discussion ended at round %d ===\n", round)
					errnie.Info("discussion ended", "rounds", round)
					return nil
				}

				loggedExecutor := func(name string, args map[string]any) (string, error) {
					errnie.Info("tool call", "persona", agent.name, "tool", name, "args", args)
					res, err := baseExecutor(name, args)

					msg := fmt.Sprintf("\n> %s used tool %s: %v\n", agent.name, name, args)
					outFile.WriteString(msg)
					fmt.Print("\033[90m" + msg + "\033[0m") // Dark gray terminal output

					return res, err
				}

				request := &provider.Request{
					Mode:         "DISCUSS",
					Message:      prompt,
					Transcript:   transcript,
					SystemPrompt: agent.system,
					Tools:        provider.DiscussionTools(),
					ToolExecutor: loggedExecutor,
				}

				errnie.Info("turn", "persona", agent.name)
				response, genErr := agent.provider.Generate(ctx, request)

				if genErr != nil {
					if ctx.Err() != nil {
						fmt.Fprintf(outFile, "\n=== Discussion ended at round %d ===\n", round)
						return nil
					}

					errnie.Error(genErr, "persona", agent.name)
					entry := fmt.Sprintf("[%s] (error: %v)\n\n", agent.name, genErr)
					outFile.WriteString(entry)
					transcript = append(transcript, entry)
					continue
				}

				entry := fmt.Sprintf("[%s]\n%s\n\n", agent.name, response)
				outFile.WriteString(entry)
				outFile.Sync()
				fmt.Print(entry)

				transcript = append(transcript, fmt.Sprintf("[%s] %s", agent.name, response))
			}
		}
	},
}

/*
agent binds a persona name, system prompt, and provider together.
*/
type agent struct {
	name     string
	system   string
	provider provider.Provider
}

/*
sortedPersonaNames returns persona keys in stable sorted order.
*/
func sortedPersonaNames(personas map[string]core.PersonaConfig) []string {
	names := make([]string, 0, len(personas))

	for name := range personas {
		names = append(names, name)
	}

	sort.Strings(names)
	return names
}

/*
buildAgents creates one agent per persona from config.
Each agent gets its own OpenAI-compatible provider pointed at the persona's
baseURL and model, with retry wrapping.
*/
func buildAgents(names []string, personas map[string]core.PersonaConfig) []agent {
	agents := make([]agent, 0, len(names))

	for _, name := range names {
		persona := personas[name]

		inner := provider.NewOpenAIProvider(
			provider.OpenAIWithBaseURL(persona.BaseURL),
			provider.OpenAIWithModel(persona.Model),
		)

		agents = append(agents, agent{
			name:     name,
			system:   persona.System,
			provider: provider.WithRetry(inner, 3),
		})
	}

	return agents
}

/*
expandHome replaces a leading ~ with the user's home directory.
*/
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	return filepath.Join(home, path[1:])
}

func init() {
	rootCmd.AddCommand(thinkcursionCmd)

	thinkcursionCmd.Flags().StringP(
		"output", "o",
		"~/.piaf/thinkcursion.md",
		"Path to write the discussion transcript",
	)
}

const thinkcursionLong = `Thinkcursion sets up AI agents from your config's personas
and puts them in an infinite round-robin discussion loop.

Each persona gets its own system prompt, model, and base URL
as defined in ai.thinkcursion.personas in config.yml.

The full discussion transcript is written to the output file
(default ~/.piaf/thinkcursion.md) and also printed to stdout.

Usage:
  piaf thinkcursion "What is the nature of consciousness?"
  piaf thinkcursion -o discussion.md "Debate P vs NP"

Press Ctrl+C to gracefully end the discussion.
`
