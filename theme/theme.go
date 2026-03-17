package theme

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

/*
Color holds an RGB triplet.
*/
type Color struct {
	R uint8 `yaml:"r"`
	G uint8 `yaml:"g"`
	B uint8 `yaml:"b"`
}

/*
Fg returns the ANSI 24-bit foreground escape for this Color.
*/
func (color Color) Fg() string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", color.R, color.G, color.B)
}

/*
Bg returns the ANSI 24-bit background escape for this Color.
*/
func (color Color) Bg() string {
	return fmt.Sprintf("\033[48;2;%d;%d;%dm", color.R, color.G, color.B)
}

/*
Theme defines all color roles for the editor UI and syntax highlighting.
Implements io.ReadWriteCloser: Write accepts YAML bytes to load a theme,
Read yields YAML bytes of the current theme, Close is a no-op.
*/
type Theme struct {
	Name string `yaml:"name"`

	UI     UIColors     `yaml:"ui"`
	Syntax SyntaxColors `yaml:"syntax"`

	output     []byte
	readOffset int
	err        error
}

/*
UIColors holds all user-interface color roles.
*/
type UIColors struct {
	Brand         Color `yaml:"brand"`
	Highlight     Color `yaml:"highlight"`
	BgPopup       Color `yaml:"bgPopup"`
	BgSelected    Color `yaml:"bgSelected"`
	BgSubtleBrand Color `yaml:"bgSubtleBrand"`
	BgSubtleHigh  Color `yaml:"bgSubtleHighlight"`
	FgDim         Color `yaml:"fgDim"`
	FgBorder      Color `yaml:"fgBorder"`
	FgSearchBox   Color `yaml:"fgSearchBox"`
	Separator     Color `yaml:"separator"`
	StatusBrand   Color `yaml:"statusBrand"`
}

/*
SyntaxColors holds all syntax highlighting color roles.
*/
type SyntaxColors struct {
	Keyword  Color `yaml:"keyword"`
	Builtin  Color `yaml:"builtin"`
	String   Color `yaml:"string"`
	Number   Color `yaml:"number"`
	Comment  Color `yaml:"comment"`
	Literal  Color `yaml:"literal"`
	Operator Color `yaml:"operator"`
}

var (
	active *Theme
	mu     sync.RWMutex
)

/*
Active returns the currently active theme. Thread-safe.
*/
func Active() *Theme {
	mu.RLock()
	defer mu.RUnlock()

	if active == nil {
		active = Default()
	}

	return active
}

/*
SetActive replaces the global theme. Thread-safe.
*/
func SetActive(theme *Theme) {
	mu.Lock()
	defer mu.Unlock()

	active = theme
}

/*
Default returns the built-in default theme matching the original piaf brand colors.
*/
func Default() *Theme {
	return &Theme{
		Name: "default",
		UI: UIColors{
			Brand:         Color{108, 80, 255},
			Highlight:     Color{254, 135, 255},
			BgPopup:       Color{18, 14, 38},
			BgSelected:    Color{38, 28, 78},
			BgSubtleBrand: Color{22, 18, 48},
			BgSubtleHigh:  Color{38, 18, 38},
			FgDim:         Color{80, 70, 100},
			FgBorder:      Color{60, 50, 120},
			FgSearchBox:   Color{200, 190, 230},
			Separator:     Color{108, 80, 255},
			StatusBrand:   Color{254, 135, 255},
		},
		Syntax: SyntaxColors{
			Keyword:  Color{170, 85, 255},
			Builtin:  Color{0, 175, 215},
			String:   Color{80, 200, 120},
			Number:   Color{230, 190, 80},
			Comment:  Color{100, 100, 100},
			Literal:  Color{230, 190, 80},
			Operator: Color{200, 200, 200},
		},
	}
}

/*
NewTheme instantiates a new Theme with default values.
*/
func NewTheme() *Theme {
	return Default()
}

/*
Read implements the io.Reader interface.
Yields YAML representation of the theme.
*/
func (theme *Theme) Read(p []byte) (n int, err error) {
	if theme.output == nil {
		theme.output, theme.err = yaml.Marshal(theme)
		theme.readOffset = 0
	}

	if theme.err != nil {
		return 0, theme.err
	}

	if theme.readOffset >= len(theme.output) {
		return 0, io.EOF
	}

	n = copy(p, theme.output[theme.readOffset:])
	theme.readOffset += n

	return n, nil
}

/*
Write implements the io.Writer interface.
Accepts YAML bytes to deserialize into this theme.
*/
func (theme *Theme) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	incoming := &Theme{}
	if err := yaml.Unmarshal(p, incoming); err != nil {
		return 0, err
	}

	theme.Name = incoming.Name
	theme.UI = incoming.UI
	theme.Syntax = incoming.Syntax
	theme.output = nil
	theme.readOffset = 0

	return len(p), nil
}

/*
Close implements the io.Closer interface.
*/
func (theme *Theme) Close() error {
	theme.output = nil
	theme.readOffset = 0

	return nil
}

/*
ThemesDir returns the path to the user's themes directory.
*/
var ThemesDir = func() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	return filepath.Join(home, ".piaf", "themes")
}

/*
Save writes the theme to disk as a YAML file.
*/
func (theme *Theme) Save() error {
	dir := ThemesDir()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(theme)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, theme.Name+".yml")

	return os.WriteFile(path, data, 0o644)
}

/*
Load reads a named theme from the themes directory.
*/
func Load(name string) (*Theme, error) {
	path := filepath.Join(ThemesDir(), name+".yml")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	loaded := &Theme{}
	if err := yaml.Unmarshal(data, loaded); err != nil {
		return nil, err
	}

	return loaded, nil
}

/*
List returns the names of all themes available in the themes directory.
*/
func List() []string {
	dir := ThemesDir()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{"default"}
	}

	names := make([]string, 0, len(entries)+1)
	names = append(names, "default")

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext == ".yml" || ext == ".yaml" {
			name := entry.Name()[:len(entry.Name())-len(ext)]

			if name != "default" {
				names = append(names, name)
			}
		}
	}

	return names
}

/*
Rename changes the theme name and re-saves under the new name.
Removes the old file if it existed.
*/
func (theme *Theme) Rename(newName string) error {
	oldPath := filepath.Join(ThemesDir(), theme.Name+".yml")
	theme.Name = newName

	if err := theme.Save(); err != nil {
		return err
	}

	return os.Remove(oldPath)
}

/*
FgBrand returns the ANSI foreground escape for the brand color.
*/
func (theme *Theme) FgBrand() string { return theme.UI.Brand.Fg() }

/*
FgHighlight returns the ANSI foreground escape for the highlight color.
*/
func (theme *Theme) FgHighlight() string { return theme.UI.Highlight.Fg() }

/*
BgBrand returns the ANSI background escape for the brand color.
*/
func (theme *Theme) BgBrand() string { return theme.UI.Brand.Bg() }

/*
BgHighlight returns the ANSI background escape for the highlight color.
*/
func (theme *Theme) BgHighlight() string { return theme.UI.Highlight.Bg() }

/*
BgPopup returns the ANSI background escape for popup dialogs.
*/
func (theme *Theme) BgPopup() string { return theme.UI.BgPopup.Bg() }

/*
BgSelected returns the ANSI background escape for selected items.
*/
func (theme *Theme) BgSelected() string { return theme.UI.BgSelected.Bg() }

/*
BgSubtleBrand returns the ANSI background for subtle brand accents.
*/
func (theme *Theme) BgSubtleBrand() string { return theme.UI.BgSubtleBrand.Bg() }

/*
BgSubtleHigh returns the ANSI background for subtle highlight accents.
*/
func (theme *Theme) BgSubtleHigh() string { return theme.UI.BgSubtleHigh.Bg() }

/*
FgDim returns the ANSI foreground for dimmed text.
*/
func (theme *Theme) FgDim() string { return theme.UI.FgDim.Fg() }

/*
FgBorder returns the ANSI foreground for border lines.
*/
func (theme *Theme) FgBorder() string { return theme.UI.FgBorder.Fg() }

/*
FgSearchBox returns the ANSI foreground for the search box.
*/
func (theme *Theme) FgSearchBox() string { return theme.UI.FgSearchBox.Fg() }

/*
SyntaxKeyword returns the ANSI foreground for syntax keywords.
*/
func (theme *Theme) SyntaxKeyword() string { return theme.Syntax.Keyword.Fg() }

/*
SyntaxBuiltin returns the ANSI foreground for built-in types.
*/
func (theme *Theme) SyntaxBuiltin() string { return theme.Syntax.Builtin.Fg() }

/*
SyntaxString returns the ANSI foreground for string literals.
*/
func (theme *Theme) SyntaxString() string { return theme.Syntax.String.Fg() }

/*
SyntaxNumber returns the ANSI foreground for numeric literals.
*/
func (theme *Theme) SyntaxNumber() string { return theme.Syntax.Number.Fg() }

/*
SyntaxComment returns the ANSI foreground for comments.
*/
func (theme *Theme) SyntaxComment() string { return theme.Syntax.Comment.Fg() }

/*
SyntaxLiteral returns the ANSI foreground for literal values.
*/
func (theme *Theme) SyntaxLiteral() string { return theme.Syntax.Literal.Fg() }
