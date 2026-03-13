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
	separatorChar  = "\u2500"
)

var chatRoleStyles = []struct {
	prefix string
	color  string
}{
	{"Discussion ", styleFgGreen},
	{"PM Summary", styleFgBlue},
	{"Project Manager", styleFgBlue},
	{"Architect", styleFgBlue},
	{"Team Lead", styleFgCyan},
	{"Developer", styleFgGreen},
	{"QA", styleFgYellow},
	{"Review", styleFgMagenta},
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

		return styleFgGray + styleDim + strings.Repeat(separatorChar, separatorWidth) + styleReset
	}

	if strings.HasPrefix(trimmed, "You:") {
		return styleBold + styleFgCyan + line + styleReset
	}

	if strings.HasPrefix(trimmed, "System:") {
		return styleFgYellow + line + styleReset
	}

	if strings.HasPrefix(trimmed, "Pipeline:") || strings.HasPrefix(trimmed, "Team:") {
		return styleDim + styleFgMagenta + line + styleReset
	}

	if strings.HasPrefix(trimmed, "Progress:") {
		return styleFgGreen + line + styleReset
	}

	if strings.HasPrefix(trimmed, "Assignment:") {
		return styleFgCyan + line + styleReset
	}

	if strings.HasPrefix(trimmed, "Channel coordination:") {
		return styleDim + line + styleReset
	}

	if strings.HasPrefix(trimmed, "Review:") {
		return styleBold + styleFgMagenta + line + styleReset
	}

	if strings.HasPrefix(trimmed, "Project board:") {
		return styleBold + styleFgBlue + line + styleReset
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
			styled[index] = styleBold + styleFgBlue + line + styleReset
		case line == "..":
			styled[index] = styleDim + line + styleReset
		default:
			styled[index] = line
		}
	}

	return styled
}
