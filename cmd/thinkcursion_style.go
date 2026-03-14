package cmd

import (
	"fmt"
	"strings"
	"time"
)

const (
	tcReset    = "\033[0m"
	tcBold     = "\033[1m"
	tcDim      = "\033[2m"
	tcFgBrand  = "\033[38;2;108;80;255m"
	tcFgHigh   = "\033[38;2;254;135;255m"
	tcFgGray   = "\033[90m"
	tcFgWhite  = "\033[97m"
	tcFgGreen  = "\033[32m"
	tcFgYellow = "\033[33m"
	tcFgCyan   = "\033[36m"
	tcBgSubtle = "\033[48;2;22;18;48m"
	tcDash     = "\u2500"
	tcDotSep   = "\u2022"
	tcArrowR   = "\u25B8"
)

/*
tcHeader renders the branded thinkcursion header block.
*/
func tcHeader(prompt string, names []string, started time.Time) string {
	width := 60

	topRule := tcFgBrand + tcDim + strings.Repeat(tcDash, width) + tcReset
	botRule := topRule

	title := tcBgSubtle + tcFgHigh + tcBold + " thinkcursion " + tcReset
	personas := tcFgGray + strings.Join(names, tcFgBrand+tcDim+" "+tcDotSep+" "+tcReset+tcFgGray) + tcReset
	promptLine := tcFgWhite + prompt + tcReset
	timeLine := tcFgGray + tcDim + started.Format(time.RFC3339) + tcReset

	return fmt.Sprintf(
		"\n%s\n%s\n%s\n\n  %s%s %s\n  %s%s %s\n  %s%s %s\n%s\n\n",
		topRule,
		title,
		"",
		tcFgHigh, tcArrowR, promptLine,
		tcFgBrand+tcDim, tcArrowR, personas,
		tcFgGray+tcDim, tcArrowR, timeLine,
		botRule,
	)
}

/*
tcRoundBanner renders a subtle round separator.
*/
func tcRoundBanner(round int) string {
	label := fmt.Sprintf(" round %d ", round)
	padWidth := 30 - len(label)

	if padWidth < 2 {
		padWidth = 2
	}

	left := strings.Repeat(tcDash, padWidth/2)
	right := strings.Repeat(tcDash, padWidth-padWidth/2)

	return "\n" + tcFgBrand + tcDim + left + tcReset +
		tcBgSubtle + tcFgBrand + label + tcReset +
		tcFgBrand + tcDim + right + tcReset + "\n"
}

/*
tcPersonaTurn renders a persona's name as a styled label before their response.
*/
func tcPersonaTurn(name string) string {
	return tcFgHigh + tcBold + name + tcReset + tcFgGray + tcDim + " " + tcDash + tcDash + tcDash + tcReset + "\n"
}

/*
tcResponse renders the persona's response body with subtle styling.
*/
func tcResponse(name, text string) string {
	return tcFgWhite + text + tcReset + "\n"
}

/*
tcToolCall renders a tool usage notice in a dim, unobtrusive style.
*/
func tcToolCall(persona, tool string, args map[string]any) string {
	argParts := make([]string, 0, len(args))

	for key, val := range args {
		argParts = append(argParts, fmt.Sprintf("%s=%v", key, val))
	}

	argStr := strings.Join(argParts, " ")

	return tcFgGray + tcDim + "  " + tcArrowR + " " + persona + " " + tcFgCyan + tool + tcReset + tcFgGray + tcDim + " " + argStr + tcReset + "\n"
}

/*
tcError renders an error message for a persona turn.
*/
func tcError(name string, err error) string {
	return tcFgYellow + tcDim + "  " + name + ": " + err.Error() + tcReset + "\n"
}

/*
tcEnded renders the discussion end marker.
*/
func tcEnded(round int) string {
	width := 60
	rule := tcFgBrand + tcDim + strings.Repeat(tcDash, width) + tcReset
	label := tcFgHigh + tcDim + fmt.Sprintf("  ended at round %d", round) + tcReset

	return "\n" + rule + "\n" + label + "\n" + rule + "\n"
}
