package editor

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	paletteKindCommand = "command"
	paletteKindFile    = "file"
	paletteKindContent = "content"
)

type paletteItem struct {
	kind  string
	label string
	value string
}

/*
Palette provides a universal search overlay for commands and files.
Implements command-palette style navigation with substring filtering.
*/
type Palette struct {
	query     []rune
	results   []paletteItem
	cursor    int
	root      string
	maxFiles  int
	maxContent int
}

type paletteOpts func(*Palette)

/*
PaletteWithRoot sets the workspace root for file search.
*/
func PaletteWithRoot(root string) paletteOpts {
	return func(palette *Palette) {
		palette.root = root
	}
}

/*
NewPalette creates a Palette ready for search.
*/
func NewPalette(opts ...paletteOpts) *Palette {
	palette := &Palette{
		results:     paletteCommands(),
		maxFiles:    500,
		maxContent:  200,
	}

	for _, opt := range opts {
		opt(palette)
	}

	if palette.root == "" {
		palette.root = "."
	}

	return palette
}

func paletteCommands() []paletteItem {
	return []paletteItem{
		{kind: paletteKindCommand, label: "chat – open AI chat", value: "chat"},
		{kind: paletteKindCommand, label: "implement – AI implement mode", value: "implement"},
		{kind: paletteKindCommand, label: "e <file> – edit file", value: "e"},
		{kind: paletteKindCommand, label: "E, Ex – file explorer", value: "E"},
		{kind: paletteKindCommand, label: "q – quit", value: "q"},
		{kind: paletteKindCommand, label: "wq – write and quit", value: "wq"},
		{kind: paletteKindCommand, label: "w – write", value: "w"},
		{kind: paletteKindCommand, label: "q! – quit without saving", value: "q!"},
	}
}

/*
Query returns the current search query.
*/
func (palette *Palette) Query() string {
	return string(palette.query)
}

/*
Results returns the filtered result lines for display.
*/
func (palette *Palette) Results() []string {
	lines := make([]string, 0, len(palette.results))

	for _, item := range palette.results {
		prefix := "  "
		if item.kind == paletteKindCommand {
			prefix = "› "
		} else if item.kind == paletteKindFile {
			prefix = "▸ "
		} else if item.kind == paletteKindContent {
			prefix = "· "
		}

		lines = append(lines, prefix+item.label)
	}

	return lines
}

/*
Cursor returns the selected index.
*/
func (palette *Palette) Cursor() int {
	return palette.cursor
}

/*
MoveUp moves the cursor up.
*/
func (palette *Palette) MoveUp() {
	if palette.cursor > 0 {
		palette.cursor--
	}
}

/*
MoveDown moves the cursor down.
*/
func (palette *Palette) MoveDown() {
	if palette.cursor < len(palette.results)-1 {
		palette.cursor++
	}
}

/*
Selected returns the currently selected item.
*/
func (palette *Palette) Selected() (kind, value string) {
	if len(palette.results) == 0 || palette.cursor < 0 || palette.cursor >= len(palette.results) {
		return "", ""
	}

	item := palette.results[palette.cursor]
	return item.kind, item.value
}

/*
Append adds a rune to the query and refreshes results.
*/
func (palette *Palette) Append(r rune) {
	palette.query = append(palette.query, r)
	palette.refresh()
}

/*
Backspace removes the last rune from the query.
*/
func (palette *Palette) Backspace() {
	if len(palette.query) > 0 {
		palette.query = palette.query[:len(palette.query)-1]
		palette.refresh()
	}
}

func (palette *Palette) refresh() {
	needle := strings.ToLower(string(palette.query))

	all := make([]paletteItem, 0)
	all = append(all, paletteCommands()...)
	all = append(all, palette.searchFiles(needle)...)
	all = append(all, palette.searchContent(needle)...)

	filtered := make([]paletteItem, 0)

	for _, item := range all {
		if needle == "" || strings.Contains(strings.ToLower(item.label), needle) {
			filtered = append(filtered, item)
		}
	}

	palette.results = filtered
	if palette.cursor >= len(palette.results) {
		palette.cursor = len(palette.results) - 1
	}
	if palette.cursor < 0 {
		palette.cursor = 0
	}
}

func (palette *Palette) searchFiles(needle string) []paletteItem {
	root, err := filepath.Abs(palette.root)
	if err != nil {
		return nil
	}

	var items []paletteItem
	count := 0

	filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		if count >= palette.maxFiles {
			return filepath.SkipAll
		}

		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}

			if needle != "" {
				rel, _ := filepath.Rel(root, path)
				if !strings.Contains(strings.ToLower(rel), needle) {
					return nil
				}
				items = append(items, paletteItem{kind: paletteKindFile, label: rel + "/", value: path})
				count++
			}

			return nil
		}

		if needle != "" {
			rel, _ := filepath.Rel(root, path)
			if !strings.Contains(strings.ToLower(rel), needle) {
				return nil
			}
			items = append(items, paletteItem{kind: paletteKindFile, label: rel, value: path})
			count++
		}

		return nil
	})

	return items
}

func (palette *Palette) searchContent(needle string) []paletteItem {
	if needle == "" || len(needle) < 2 {
		return nil
	}

	root, err := filepath.Abs(palette.root)
	if err != nil {
		return nil
	}

	var items []paletteItem
	count := 0

	filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		if count >= palette.maxContent {
			return filepath.SkipAll
		}

		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}

			return nil
		}

		if info.Size() > 256*1024 {
			return nil
		}

		ext := filepath.Ext(path)
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" ||
			ext == ".ico" || ext == ".pdf" || ext == ".woff" || ext == ".woff2" {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		if !utf8Valid(content) {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		lines := strings.Split(string(content), "\n")

		for lineNum, line := range lines {
			if count >= palette.maxContent {
				break
			}

			lower := strings.ToLower(line)
			if strings.Contains(lower, needle) {
				snippet := strings.TrimSpace(line)
				if len(snippet) > 60 {
					snippet = snippet[:57] + "..."
				}

				label := rel + ":" + itoa(lineNum+1) + " " + snippet
				items = append(items, paletteItem{
					kind:  paletteKindContent,
					label: label,
					value: path + ":" + itoa(lineNum+1),
				})
				count++
			}
		}

		return nil
	})

	return items
}

func utf8Valid(b []byte) bool {
	for index := 0; index < len(b); index++ {
		if b[index] < 0x20 && b[index] != '\t' && b[index] != '\n' && b[index] != '\r' {
			return false
		}
	}

	return true
}

func itoa(n int) string {
	if n <= 0 {
		return "0"
	}

	var buf [12]byte
	index := len(buf) - 1

	for n > 0 {
		buf[index] = byte('0' + n%10)
		n /= 10
		index--
	}

	return string(buf[index+1:])
}
