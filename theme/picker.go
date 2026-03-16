package theme

import (
	"fmt"
	"io"
	"strings"
)

const (
	ansiReset = "\033[0m"
	ansiBold  = "\033[1m"
	ansiDim   = "\033[2m"

	pickerBoxTopLeft     = "\u256D"
	pickerBoxTopRight    = "\u256E"
	pickerBoxBottomLeft  = "\u2570"
	pickerBoxBottomRight = "\u256F"
	pickerBoxHorizontal  = "\u2500"
	pickerBoxVertical    = "\u2502"
	pickerBlock          = "\u2588"
)

/*
ColorRole identifies which part of the theme is being edited.
*/
type ColorRole int

const (
	RoleUIBrand ColorRole = iota
	RoleUIHighlight
	RoleUIBgPopup
	RoleUIBgSelected
	RoleUIBgSubtleBrand
	RoleUIBgSubtleHigh
	RoleUIFgDim
	RoleUIFgBorder
	RoleUIFgSearchBox
	RoleSyntaxKeyword
	RoleSyntaxBuiltin
	RoleSyntaxString
	RoleSyntaxNumber
	RoleSyntaxComment
	RoleSyntaxLiteral
	roleCount
)

var roleNames = [roleCount]string{
	"UI: Brand",
	"UI: Highlight",
	"UI: Popup Background",
	"UI: Selected Background",
	"UI: Subtle Brand Background",
	"UI: Subtle Highlight Background",
	"UI: Dim Foreground",
	"UI: Border Foreground",
	"UI: Search Box Foreground",
	"Syntax: Keyword",
	"Syntax: Builtin",
	"Syntax: String",
	"Syntax: Number",
	"Syntax: Comment",
	"Syntax: Literal",
}

/*
Picker provides an interactive color editing overlay.
Implements io.ReadWriteCloser for consistency with the project philosophy.
*/
type Picker struct {
	cursor  int
	channel int
	theme   *Theme
	output  []byte
	readOff int
}

/*
NewPicker creates a color picker bound to the given theme.
*/
func NewPicker(theme *Theme) *Picker {
	return &Picker{theme: theme}
}

/*
Cursor returns the currently selected role index.
*/
func (picker *Picker) Cursor() int { return picker.cursor }

/*
MoveUp moves the cursor up one role.
*/
func (picker *Picker) MoveUp() {
	if picker.cursor > 0 {
		picker.cursor--
	}
}

/*
MoveDown moves the cursor down one role.
*/
func (picker *Picker) MoveDown() {
	if picker.cursor < int(roleCount)-1 {
		picker.cursor++
	}
}

/*
CycleChannel switches between R, G, B editing channels.
*/
func (picker *Picker) CycleChannel() {
	picker.channel = (picker.channel + 1) % 3
}

/*
Channel returns the currently selected channel (0=R, 1=G, 2=B).
*/
func (picker *Picker) Channel() int { return picker.channel }

/*
Increase bumps the active channel of the selected role up by step.
*/
func (picker *Picker) Increase(step int) {
	color := picker.colorPtr()

	switch picker.channel {
	case 0:
		color.R = clampAdd(color.R, step)
	case 1:
		color.G = clampAdd(color.G, step)
	case 2:
		color.B = clampAdd(color.B, step)
	}
}

/*
Decrease reduces the active channel of the selected role down by step.
*/
func (picker *Picker) Decrease(step int) {
	color := picker.colorPtr()

	switch picker.channel {
	case 0:
		color.R = clampSub(color.R, step)
	case 1:
		color.G = clampSub(color.G, step)
	case 2:
		color.B = clampSub(color.B, step)
	}
}

/*
SelectedRole returns the ColorRole at the cursor.
*/
func (picker *Picker) SelectedRole() ColorRole {
	return ColorRole(picker.cursor)
}

/*
RoleName returns the display name for the selected role.
*/
func (picker *Picker) RoleName() string {
	return roleNames[picker.cursor]
}

func (picker *Picker) colorPtr() *Color {
	switch ColorRole(picker.cursor) {
	case RoleUIBrand:
		return &picker.theme.UI.Brand
	case RoleUIHighlight:
		return &picker.theme.UI.Highlight
	case RoleUIBgPopup:
		return &picker.theme.UI.BgPopup
	case RoleUIBgSelected:
		return &picker.theme.UI.BgSelected
	case RoleUIBgSubtleBrand:
		return &picker.theme.UI.BgSubtleBrand
	case RoleUIBgSubtleHigh:
		return &picker.theme.UI.BgSubtleHigh
	case RoleUIFgDim:
		return &picker.theme.UI.FgDim
	case RoleUIFgBorder:
		return &picker.theme.UI.FgBorder
	case RoleUIFgSearchBox:
		return &picker.theme.UI.FgSearchBox
	case RoleSyntaxKeyword:
		return &picker.theme.Syntax.Keyword
	case RoleSyntaxBuiltin:
		return &picker.theme.Syntax.Builtin
	case RoleSyntaxString:
		return &picker.theme.Syntax.String
	case RoleSyntaxNumber:
		return &picker.theme.Syntax.Number
	case RoleSyntaxComment:
		return &picker.theme.Syntax.Comment
	case RoleSyntaxLiteral:
		return &picker.theme.Syntax.Literal
	default:
		return &picker.theme.UI.Brand
	}
}

/*
Read implements the io.Reader interface.
*/
func (picker *Picker) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

/*
Write implements the io.Writer interface.
*/
func (picker *Picker) Write(p []byte) (n int, err error) {
	return len(p), nil
}

/*
Close implements the io.Closer interface.
*/
func (picker *Picker) Close() error {
	return nil
}

/*
Overlay renders the color picker popup over the given background lines.
*/
func (picker *Picker) Overlay(bgLines []string, width, height int) []string {
	popupWidth := 52
	if popupWidth > width-4 {
		popupWidth = width - 4
	}

	innerWidth := popupWidth - 2
	popupHeight := int(roleCount) + 6
	startRow := (height - popupHeight) / 2

	if startRow < 0 {
		startRow = 0
	}

	leftPad := (width - popupWidth) / 2
	margin := strings.Repeat(" ", leftPad)

	theme := picker.theme
	borderFg := theme.FgBorder()
	popupBg := theme.BgPopup()
	brandFg := theme.FgBrand()
	highFg := theme.FgHighlight()
	dimFg := theme.FgDim()

	for len(bgLines) < height-1 {
		bgLines = append(bgLines, "")
	}

	out := make([]string, len(bgLines))

	for row := range bgLines {
		rel := row - startRow

		if rel < 0 || rel >= popupHeight {
			out[row] = ansiDim + bgLines[row] + ansiReset
			continue
		}

		var line strings.Builder
		line.Grow(popupWidth + 256)
		line.WriteString(margin)

		switch {
		case rel == 0:
			line.WriteString(borderFg)
			line.WriteString(pickerBoxTopLeft)
			line.WriteString(strings.Repeat(pickerBoxHorizontal, innerWidth))
			line.WriteString(pickerBoxTopRight)
			line.WriteString(ansiReset)

		case rel == 1:
			title := " Theme: " + theme.Name + " "
			pad := innerWidth - len([]rune(title))
			if pad < 0 {
				pad = 0
			}

			line.WriteString(popupBg + borderFg + pickerBoxVertical + ansiReset)
			line.WriteString(popupBg + brandFg + ansiBold + title + ansiReset)
			line.WriteString(popupBg + strings.Repeat(" ", pad) + ansiReset)
			line.WriteString(popupBg + borderFg + pickerBoxVertical + ansiReset)

		case rel == 2:
			channelLabels := [3]string{"R", "G", "B"}
			channelColors := [3]string{"\033[31m", "\033[32m", "\033[34m"}
			hint := " Channel: "

			for idx, label := range channelLabels {
				if idx == picker.channel {
					hint += channelColors[idx] + ansiBold + "[" + label + "]" + ansiReset + popupBg + " "
				} else {
					hint += dimFg + popupBg + " " + label + "  "
				}
			}

			hint += " (Tab to switch)"
			pad := innerWidth - visibleLen(hint)
			if pad < 0 {
				pad = 0
			}

			line.WriteString(popupBg + borderFg + pickerBoxVertical + ansiReset)
			line.WriteString(popupBg + dimFg + hint + ansiReset)
			line.WriteString(popupBg + strings.Repeat(" ", pad) + ansiReset)
			line.WriteString(popupBg + borderFg + pickerBoxVertical + ansiReset)

		case rel == 3:
			line.WriteString(borderFg + pickerBoxVertical + ansiReset)
			line.WriteString(ansiDim + strings.Repeat(pickerBoxHorizontal, innerWidth) + ansiReset)
			line.WriteString(borderFg + pickerBoxVertical + ansiReset)

		case rel == popupHeight-2:
			hint := " j/k:move  h/l:adjust  Tab:channel  Enter:done "
			pad := innerWidth - len([]rune(hint))
			if pad < 0 {
				pad = 0
			}

			line.WriteString(popupBg + borderFg + pickerBoxVertical + ansiReset)
			line.WriteString(popupBg + dimFg + hint + ansiReset)
			line.WriteString(popupBg + strings.Repeat(" ", pad) + ansiReset)
			line.WriteString(popupBg + borderFg + pickerBoxVertical + ansiReset)

		case rel == popupHeight-1:
			line.WriteString(borderFg)
			line.WriteString(pickerBoxBottomLeft)
			line.WriteString(strings.Repeat(pickerBoxHorizontal, innerWidth))
			line.WriteString(pickerBoxBottomRight)
			line.WriteString(ansiReset)

		default:
			roleIdx := rel - 4

			if roleIdx >= 0 && roleIdx < int(roleCount) {
				color := picker.roleColor(ColorRole(roleIdx))
				name := roleNames[roleIdx]
				isSelected := roleIdx == picker.cursor

				rowBg := popupBg
				fg := dimFg
				if isSelected {
					rowBg = theme.BgSelected()
					fg = highFg
				}

				swatch := color.Fg() + pickerBlock + pickerBlock + ansiReset
				rgb := fmt.Sprintf("(%3d,%3d,%3d)", color.R, color.G, color.B)
				content := fmt.Sprintf(" %s %s  %s", swatch, name, rgb)
				pad := innerWidth - visibleLen(content)
				if pad < 0 {
					pad = 0
				}

				line.WriteString(rowBg + borderFg + pickerBoxVertical + ansiReset)
				line.WriteString(rowBg + fg)
				line.WriteString(content)
				line.WriteString(strings.Repeat(" ", pad))
				line.WriteString(ansiReset)
				line.WriteString(rowBg + borderFg + pickerBoxVertical + ansiReset)
			} else {
				line.WriteString(popupBg + borderFg + pickerBoxVertical + ansiReset)
				line.WriteString(popupBg + strings.Repeat(" ", innerWidth) + ansiReset)
				line.WriteString(popupBg + borderFg + pickerBoxVertical + ansiReset)
			}
		}

		out[row] = line.String()
	}

	return out
}

func (picker *Picker) roleColor(role ColorRole) Color {
	saved := picker.cursor
	picker.cursor = int(role)
	color := *picker.colorPtr()
	picker.cursor = saved

	return color
}

func clampAdd(val uint8, step int) uint8 {
	result := int(val) + step
	if result > 255 {
		return 255
	}

	return uint8(result)
}

func clampSub(val uint8, step int) uint8 {
	result := int(val) - step
	if result < 0 {
		return 0
	}

	return uint8(result)
}

func visibleLen(s string) int {
	count := 0
	inEscape := false

	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}

		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}

		count++
	}

	return count
}
