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

const (
	boxTopLeft     = "\u256D"
	boxTopRight    = "\u256E"
	boxBottomLeft  = "\u2570"
	boxBottomRight = "\u256F"
	boxHorizontal  = "\u2500"
	boxVertical    = "\u2502"

	styleBgPopup     = "\033[48;2;18;14;38m"
	styleBgSelected  = "\033[48;2;38;28;78m"
	styleFgDim       = "\033[38;2;80;70;100m"
	styleFgBorder    = "\033[38;2;60;50;120m"
	styleFgSearchBox = "\033[38;2;200;190;230m"
)

/*
stylePaletteOverlay renders a centered popup over the background lines,
showing the search input and filtered results in a bordered box.
*/
func stylePaletteOverlay(bgLines []string, palette *Palette, width, height int) []string {
	popupWidth := width * 3 / 5
	if popupWidth < 40 {
		popupWidth = 40
	}

	if popupWidth > width-4 {
		popupWidth = width - 4
	}

	maxResults := height / 3
	if maxResults < 4 {
		maxResults = 4
	}

	results := palette.Results()
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	popupHeight := len(results) + 4
	startRow := (height-popupHeight)/2 - 1

	if startRow < 0 {
		startRow = 0
	}

	startCol := (width - popupWidth) / 2
	innerWidth := popupWidth - 2

	for len(bgLines) < height-1 {
		bgLines = append(bgLines, "")
	}

	out := make([]string, len(bgLines))

	for row := range bgLines {
		rel := row - startRow

		if rel < 0 || rel >= popupHeight {
			out[row] = styleDim + bgLines[row] + styleReset
			continue
		}

		bg := bgLines[row]
		bgRunes := []rune(bg)

		var line strings.Builder
		line.Grow(popupWidth + 128)

		if startCol > 0 && len(bgRunes) > 0 {
			end := startCol

			if end > len(bgRunes) {
				end = len(bgRunes)
			}

			line.WriteString(styleDim)
			line.WriteString(string(bgRunes[:end]))
			line.WriteString(styleReset)
		}

		switch {
		case rel == 0:
			line.WriteString(styleFgBorder)
			line.WriteString(boxTopLeft)
			line.WriteString(strings.Repeat(boxHorizontal, innerWidth))
			line.WriteString(boxTopRight)
			line.WriteString(styleReset)

		case rel == 1:
			query := palette.Query()
			prompt := styleFgBrand + styleBold + " / " + styleReset + styleBgPopup + styleFgSearchBox + query

			pad := innerWidth - 3 - runeWidth(query)
			if pad < 0 {
				pad = 0
			}

			line.WriteString(styleBgPopup + styleFgBorder + boxVertical + styleReset)
			line.WriteString(styleBgPopup)
			line.WriteString(prompt)
			line.WriteString(strings.Repeat(" ", pad))
			line.WriteString(styleReset)
			line.WriteString(styleBgPopup + styleFgBorder + boxVertical + styleReset)

		case rel == 2:
			line.WriteString(styleFgBorder)
			line.WriteString(boxVertical)
			line.WriteString(styleDim)
			line.WriteString(strings.Repeat(boxHorizontal, innerWidth))
			line.WriteString(styleReset)
			line.WriteString(styleFgBorder)
			line.WriteString(boxVertical)
			line.WriteString(styleReset)

		case rel == popupHeight-1:
			line.WriteString(styleFgBorder)
			line.WriteString(boxBottomLeft)
			line.WriteString(strings.Repeat(boxHorizontal, innerWidth))
			line.WriteString(boxBottomRight)
			line.WriteString(styleReset)

		default:
			resultIdx := rel - 3

			if resultIdx >= 0 && resultIdx < len(results) {
				text := results[resultIdx]

				if len(text) > innerWidth-1 {
					text = text[:innerWidth-1]
				}

				pad := innerWidth - 1 - runeWidth(text)
				if pad < 0 {
					pad = 0
				}

				isSelected := resultIdx == palette.Cursor()

				bg := styleBgPopup
				if isSelected {
					bg = styleBgSelected
				}

				fg := styleFgDim
				if isSelected {
					fg = styleFgHighlight
				}

				line.WriteString(bg + styleFgBorder + boxVertical + styleReset)
				line.WriteString(bg + fg)
				line.WriteString(" ")
				line.WriteString(text)
				line.WriteString(strings.Repeat(" ", pad))
				line.WriteString(styleReset)
				line.WriteString(bg + styleFgBorder + boxVertical + styleReset)
			} else {
				line.WriteString(styleBgPopup + styleFgBorder + boxVertical + styleReset)
				line.WriteString(styleBgPopup)
				line.WriteString(strings.Repeat(" ", innerWidth))
				line.WriteString(styleReset)
				line.WriteString(styleBgPopup + styleFgBorder + boxVertical + styleReset)
			}
		}

		after := startCol + popupWidth
		if after < len(bgRunes) {
			line.WriteString(styleDim)
			line.WriteString(string(bgRunes[after:]))
			line.WriteString(styleReset)
		}

		out[row] = line.String()
	}

	return out
}

func runeWidth(s string) int {
	count := 0

	for range s {
		count++
	}

	return count
}
