package editor

import (
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

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

var separatorLineCache sync.Map

type syntaxSpec struct {
	lineComment string
	keywords    map[string]struct{}
	builtins    map[string]struct{}
	literals    map[string]struct{}
}

var (
	goSyntaxSpec = syntaxSpec{
		lineComment: "//",
		keywords: keywordSet(
			"break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough",
			"for", "func", "go", "goto", "if", "import", "interface", "map", "package", "range",
			"return", "select", "struct", "switch", "type", "var",
		),
		builtins: keywordSet(
			"any", "append", "bool", "byte", "cap", "close", "comparable", "complex", "complex64",
			"complex128", "copy", "delete", "error", "false", "float32", "float64", "imag", "int",
			"int8", "int16", "int32", "int64", "iota", "len", "make", "new", "nil", "panic", "print",
			"println", "real", "recover", "rune", "string", "true", "uint", "uint8", "uint16", "uint32",
			"uint64", "uintptr",
		),
	}
	jsonSyntaxSpec = syntaxSpec{
		literals: keywordSet("true", "false", "null"),
	}
	yamlSyntaxSpec = syntaxSpec{
		lineComment: "#",
		literals:    keywordSet("true", "false", "null"),
	}
	jsSyntaxSpec = syntaxSpec{
		lineComment: "//",
		keywords: keywordSet(
			"async", "await", "break", "case", "catch", "class", "const", "continue", "default",
			"do", "else", "export", "extends", "finally", "for", "from", "function", "if", "import",
			"interface", "let", "new", "return", "switch", "throw", "try", "type", "var", "while",
		),
		literals: keywordSet("false", "null", "true", "undefined"),
	}
	pythonSyntaxSpec = syntaxSpec{
		lineComment: "#",
		keywords: keywordSet(
			"and", "as", "assert", "break", "class", "continue", "def", "elif", "else", "except",
			"finally", "for", "from", "if", "import", "in", "is", "lambda", "not", "or", "pass",
			"raise", "return", "try", "while", "with", "yield",
		),
		literals: keywordSet("False", "None", "True"),
	}
	shellSyntaxSpec = syntaxSpec{
		lineComment: "#",
		keywords: keywordSet(
			"case", "do", "done", "elif", "else", "esac", "fi", "for", "function", "if", "in",
			"select", "then", "until", "while",
		),
	}
)

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

func styleCodeLines(lines []string, path string) []string {
	spec := syntaxSpecForPath(path)
	if spec == nil {
		return lines
	}

	styled := make([]string, len(lines))

	for index, line := range lines {
		styled[index] = styleCodeLine(line, spec)
	}

	return styled
}

func styleCodeLine(line string, spec *syntaxSpec) string {
	if line == "" || spec == nil {
		return line
	}

	runes := []rune(line)
	var out strings.Builder
	out.Grow(len(line) + 32)

	for index := 0; index < len(runes); {
		if spec.lineComment != "" && hasRunesPrefix(runes[index:], spec.lineComment) {
			out.WriteString(styleDim)
			out.WriteString(styleFgGray)
			out.WriteString(string(runes[index:]))
			out.WriteString(styleReset)
			break
		}

		if hasRunesPrefix(runes[index:], "/*") {
			end := consumeBlockCommentRunes(runes, index)
			out.WriteString(styleDim)
			out.WriteString(styleFgGray)
			out.WriteString(string(runes[index:end]))
			out.WriteString(styleReset)
			index = end
			continue
		}

		if isQuoteRune(runes[index]) {
			end := consumeQuotedRunes(runes, index)
			out.WriteString(styleFgGreen)
			out.WriteString(string(runes[index:end]))
			out.WriteString(styleReset)
			index = end
			continue
		}

		if isNumberStart(runes, index) {
			end := consumeNumberRunes(runes, index)
			out.WriteString(styleFgYellow)
			out.WriteString(string(runes[index:end]))
			out.WriteString(styleReset)
			index = end
			continue
		}

		if isIdentifierStart(runes[index]) {
			end := consumeIdentifierRunes(runes, index)
			word := string(runes[index:end])

			switch {
			case syntaxContains(spec.keywords, word):
				out.WriteString(styleBold)
				out.WriteString(styleFgMagenta)
				out.WriteString(word)
				out.WriteString(styleReset)
			case syntaxContains(spec.builtins, word):
				out.WriteString(styleFgCyan)
				out.WriteString(word)
				out.WriteString(styleReset)
			case syntaxContains(spec.literals, word):
				out.WriteString(styleFgYellow)
				out.WriteString(word)
				out.WriteString(styleReset)
			default:
				out.WriteString(word)
			}

			index = end
			continue
		}

		out.WriteRune(runes[index])
		index++
	}

	return out.String()
}

func styleChatLine(line string, width int) string {
	trimmed := strings.TrimSpace(line)

	if trimmed == "" {
		return line
	}

	if trimmed == "---" {
		return cachedSeparatorLine(width)
	}

	switch trimmed[0] {
	case 'A':
		if strings.HasPrefix(trimmed, "Assignment:") {
			return styleFgBrand + line + styleReset
		}

		if strings.HasPrefix(trimmed, "Architect") {
			return styleRoleLabel(line, styleFgHighlight)
		}

	case 'C':
		if strings.HasPrefix(trimmed, "Channel coordination:") {
			return styleDim + line + styleReset
		}

	case 'D':
		if strings.HasPrefix(trimmed, "Discussion window") {
			return styleDim + line + styleReset
		}

		if strings.HasPrefix(trimmed, "Discussion ") {
			return styleRoleLabel(line, styleFgBrand)
		}

		if strings.HasPrefix(trimmed, "Developer") {
			return styleRoleLabel(line, styleFgGreen)
		}

	case 'I':
		if strings.HasPrefix(trimmed, "Implementation complete.") {
			return styleBold + styleFgGreen + line + styleReset
		}

		if strings.HasPrefix(trimmed, "Implementation window") {
			return styleDim + line + styleReset
		}

	case 'P':
		if strings.HasPrefix(trimmed, "Pipeline:") {
			return styleDim + styleFgHighlight + line + styleReset
		}

		if strings.HasPrefix(trimmed, "Progress:") {
			return styleFgGreen + line + styleReset
		}

		if strings.HasPrefix(trimmed, "Project board:") {
			return styleBold + styleFgBrand + line + styleReset
		}

		if strings.HasPrefix(trimmed, "Press i to") {
			return styleDim + line + styleReset
		}

		if strings.HasPrefix(trimmed, "PM Summary") || strings.HasPrefix(trimmed, "Project Manager") {
			return styleRoleLabel(line, styleFgBrand)
		}

	case 'Q':
		if strings.HasPrefix(trimmed, "QA") {
			return styleRoleLabel(line, styleFgYellow)
		}

	case 'R':
		if strings.HasPrefix(trimmed, "Review:") {
			return styleBold + styleFgHighlight + line + styleReset
		}

		if strings.HasPrefix(trimmed, "Review") {
			return styleRoleLabel(line, styleFgHighlight)
		}

	case 'S':
		if strings.HasPrefix(trimmed, "System:") {
			return styleFgYellow + line + styleReset
		}

	case 'T':
		if strings.HasPrefix(trimmed, "Team:") {
			return styleDim + styleFgHighlight + line + styleReset
		}

		if strings.HasPrefix(trimmed, "Team Lead") {
			return styleRoleLabel(line, styleFgBrand)
		}

	case 'U':
		if strings.HasPrefix(trimmed, "Use prompts") || strings.HasPrefix(trimmed, "Use :accept") {
			return styleDim + line + styleReset
		}

	case 'Y':
		if strings.HasPrefix(trimmed, "You:") {
			return styleBold + styleFgBrand + line + styleReset
		}
	}

	return line
}

func cachedSeparatorLine(width int) string {
	if width <= 0 {
		width = 40
	}

	if line, ok := separatorLineCache.Load(width); ok {
		if cachedLine, ok := line.(string); ok {
			return cachedLine
		}
	}

	line := styleFgBrand + styleDim + strings.Repeat(separatorChar, width) + styleReset
	separatorLineCache.Store(width, line)

	return line
}

func styleRoleLabel(line, styleCode string) string {
	colonIdx := strings.IndexByte(line, ':')
	if colonIdx > 0 {
		label := line[:colonIdx+1]
		body := line[colonIdx+1:]

		return styleBold + styleCode + label + styleReset + body
	}

	return styleBold + styleCode + line + styleReset
}

func keywordSet(words ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(words))

	for _, word := range words {
		set[word] = struct{}{}
	}

	return set
}

func syntaxSpecForPath(path string) *syntaxSpec {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return &goSyntaxSpec
	case ".json":
		return &jsonSyntaxSpec
	case ".yaml", ".yml":
		return &yamlSyntaxSpec
	case ".js", ".jsx", ".ts", ".tsx":
		return &jsSyntaxSpec
	case ".py":
		return &pythonSyntaxSpec
	case ".sh", ".bash", ".zsh":
		return &shellSyntaxSpec
	default:
		return nil
	}
}

func syntaxContains(set map[string]struct{}, word string) bool {
	if len(set) == 0 {
		return false
	}

	_, ok := set[word]

	return ok
}

func hasRunesPrefix(runes []rune, prefix string) bool {
	if len(runes) < len(prefix) {
		return false
	}

	for index, expected := range prefix {
		if runes[index] != expected {
			return false
		}
	}

	return true
}

func isQuoteRune(r rune) bool {
	return r == '"' || r == '\'' || r == '`'
}

func consumeQuotedRunes(runes []rune, start int) int {
	quote := runes[start]

	for index := start + 1; index < len(runes); index++ {
		if quote != '`' && runes[index] == '\\' {
			index++
			continue
		}

		if runes[index] == quote {
			return index + 1
		}
	}

	return len(runes)
}

func consumeBlockCommentRunes(runes []rune, start int) int {
	for index := start + 2; index < len(runes)-1; index++ {
		if runes[index] == '*' && runes[index+1] == '/' {
			return index + 2
		}
	}

	return len(runes)
}

func isNumberStart(runes []rune, index int) bool {
	if !unicode.IsDigit(runes[index]) {
		return false
	}

	if index == 0 {
		return true
	}

	return !isIdentifierPart(runes[index-1])
}

func consumeNumberRunes(runes []rune, start int) int {
	for index := start + 1; index < len(runes); index++ {
		r := runes[index]
		if !(unicode.IsDigit(r) || unicode.IsLetter(r) || r == '.' || r == '_') {
			return index
		}
	}

	return len(runes)
}

func isIdentifierStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentifierPart(r rune) bool {
	return isIdentifierStart(r) || unicode.IsDigit(r)
}

func consumeIdentifierRunes(runes []rune, start int) int {
	for index := start + 1; index < len(runes); index++ {
		if !isIdentifierPart(runes[index]) {
			return index
		}
	}

	return len(runes)
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

	leftPad := (width - popupWidth) / 2
	innerWidth := popupWidth - 2

	for len(bgLines) < height-1 {
		bgLines = append(bgLines, "")
	}

	out := make([]string, len(bgLines))
	margin := strings.Repeat(" ", leftPad)

	for row := range bgLines {
		rel := row - startRow

		if rel < 0 || rel >= popupHeight {
			out[row] = styleDim + bgLines[row] + styleReset
			continue
		}

		var line strings.Builder
		line.Grow(popupWidth + 128)
		line.WriteString(margin)

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

			pad := innerWidth - 3 - runeCount(query)
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

				if runeCount(text) > innerWidth-1 {
					trimmed := []rune(text)

					if len(trimmed) > innerWidth-1 {
						trimmed = trimmed[:innerWidth-1]
					}

					text = string(trimmed)
				}

				pad := innerWidth - 1 - runeCount(text)
				if pad < 0 {
					pad = 0
				}

				isSelected := resultIdx == palette.Cursor()

				rowBg := styleBgPopup
				if isSelected {
					rowBg = styleBgSelected
				}

				fg := styleFgDim
				if isSelected {
					fg = styleFgHighlight
				}

				line.WriteString(rowBg + styleFgBorder + boxVertical + styleReset)
				line.WriteString(rowBg + fg)
				line.WriteString(" ")
				line.WriteString(text)
				line.WriteString(strings.Repeat(" ", pad))
				line.WriteString(styleReset)
				line.WriteString(rowBg + styleFgBorder + boxVertical + styleReset)
			} else {
				line.WriteString(styleBgPopup + styleFgBorder + boxVertical + styleReset)
				line.WriteString(styleBgPopup)
				line.WriteString(strings.Repeat(" ", innerWidth))
				line.WriteString(styleReset)
				line.WriteString(styleBgPopup + styleFgBorder + boxVertical + styleReset)
			}
		}

		out[row] = line.String()
	}

	return out
}

func runeCount(s string) int {
	count := 0

	for range s {
		count++
	}

	return count
}
