package editor

import "strings"

const (
	styleReset     = "\033[0m"
	styleBold      = "\033[1m"
	styleDim       = "\033[2m"
	styleFgRed     = "\033[31m"
	styleFgGreen   = "\033[32m"
	styleFgYellow  = "\033[33m"
	styleFgBlue    = "\033[34m"
	styleFgMagenta = "\033[35m"
	styleFgCyan    = "\033[36m"
	styleFgGray    = "\033[90m"

	styleFgBrand     = "\033[38;2;108;80;255m"
	styleFgHighlight = "\033[38;2;254;135;255m"
	styleBgBrand     = "\033[48;2;108;80;255m"

	separatorChar = "\u2500"
)

var chatRoleStyles = []struct {
	prefix string
	color  string
}{
	{"Discussion ", styleFgBrand},
	{"PM Summary", styleFgBrand},
	{"Project Manager", styleFgBrand},
	{"Architect", styleFgHighlight},
	{"Team Lead", styleFgBrand},
	{"Developer", styleFgGreen},
	{"QA", styleFgYellow},
	{"Review", styleFgHighlight},
}

/*
styleChatLines applies ANSI color codes to wrapped chat lines.
Operates on already-wrapped text so line widths are unaffected.
*/
func styleChatLines(lines []string, width int) []string {
	styled := make([]string, len(lines))

	for index, line := range lines {
		styled[index] = styleChatLine(line, width)
	}

	return styled
}

func styleChatLine(line string, width int) string {
	trimmed := strings.TrimSpace(line)

	if trimmed == "" {
		return line
	}

	if trimmed == "---" {
		separatorWidth := width
		if separatorWidth <= 0 {
			separatorWidth = 40
		}

		return styleFgBrand + styleDim + strings.Repeat(separatorChar, separatorWidth) + styleReset
	}

	if strings.HasPrefix(trimmed, "You:") {
		return styleBold + styleFgBrand + line + styleReset
	}

	if strings.HasPrefix(trimmed, "System:") {
		return styleFgYellow + line + styleReset
	}

	if strings.HasPrefix(trimmed, "Pipeline:") || strings.HasPrefix(trimmed, "Team:") {
		return styleDim + styleFgHighlight + line + styleReset
	}

	if strings.HasPrefix(trimmed, "Progress:") {
		return styleFgGreen + line + styleReset
	}

	if strings.HasPrefix(trimmed, "Assignment:") {
		return styleFgBrand + line + styleReset
	}

	if strings.HasPrefix(trimmed, "Channel coordination:") {
		return styleDim + line + styleReset
	}

	if strings.HasPrefix(trimmed, "Review:") {
		return styleBold + styleFgHighlight + line + styleReset
	}

	if strings.HasPrefix(trimmed, "Project board:") {
		return styleBold + styleFgBrand + line + styleReset
	}

	if strings.HasPrefix(trimmed, "Implementation complete.") {
		return styleBold + styleFgGreen + line + styleReset
	}

	if strings.HasPrefix(trimmed, "Discussion window") ||
		strings.HasPrefix(trimmed, "Implementation window") ||
		strings.HasPrefix(trimmed, "Press i to") ||
		strings.HasPrefix(trimmed, "Use prompts") ||
		strings.HasPrefix(trimmed, "Use :accept") {
		return styleDim + line + styleReset
	}

	for _, role := range chatRoleStyles {
		if strings.HasPrefix(trimmed, role.prefix) {
			colonIdx := strings.Index(line, ":")
			if colonIdx > 0 {
				label := line[:colonIdx+1]
				body := line[colonIdx+1:]

				return styleBold + role.color + label + styleReset + body
			}

			return styleBold + role.color + line + styleReset
		}
	}

	return line
}

/*
styleExplorerLines applies ANSI color codes to explorer entries.
Directories are bold blue, the parent entry is dim.
*/
func styleExplorerLines(lines []string) []string {
	styled := make([]string, len(lines))

	for index, line := range lines {
		switch {
		case strings.HasSuffix(line, "/"):
			styled[index] = styleBold + styleFgBrand + line + styleReset
		case line == "..":
			styled[index] = styleDim + line + styleReset
		default:
			styled[index] = line
		}
	}

	return styled
}
