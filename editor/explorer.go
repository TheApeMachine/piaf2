package editor

import (
	"os"
	"path/filepath"
	"sort"
)

/*
Explorer displays a directory listing for file navigation.
Produces lines for the Ex-style file manager view.
*/
type Explorer struct {
	path    string
	entries []string
	cursor  int
}

/*
NewExplorer creates an Explorer at the given path.
Path of "" or "." uses the current working directory.
*/
func NewExplorer(path string) *Explorer {
	explorer := &Explorer{path: path}
	explorer.refresh()
	return explorer
}

/*
Refresh reloads the directory listing.
*/
func (explorer *Explorer) Refresh() {
	explorer.refresh()
}

/*
refresh rebuilds the directory listing and sorts entries.
*/
func (explorer *Explorer) refresh() {
	dir := explorer.path
	if dir == "" || dir == "." {
		dir, _ = os.Getwd()
		explorer.path = dir
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		explorer.entries = []string{".."}
		explorer.cursor = 0
		return
	}

	names := make([]string, 0, len(entries)+1)
	names = append(names, "..")

	for _, entry := range entries {
		name := entry.Name()
		if name == "." {
			continue
		}
		if entry.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}

	sort.Slice(names[1:], func(i, j int) bool {
		left := names[1+i]
		right := names[1+j]
		leftDir := len(left) > 0 && left[len(left)-1] == '/'
		rightDir := len(right) > 0 && right[len(right)-1] == '/'
		if leftDir != rightDir {
			return leftDir
		}
		return left < right
	})

	explorer.entries = names
	if explorer.cursor >= len(explorer.entries) {
		explorer.cursor = len(explorer.entries) - 1
	}
	if explorer.cursor < 0 {
		explorer.cursor = 0
	}
}

/*
Lines returns the display lines for the explorer view.
*/
func (explorer *Explorer) Lines() []string {
	return explorer.entries
}

/*
Cursor returns the current selection index.
*/
func (explorer *Explorer) Cursor() int {
	return explorer.cursor
}

/*
MoveUp moves the cursor up.
*/
func (explorer *Explorer) MoveUp() {
	if explorer.cursor > 0 {
		explorer.cursor--
	}
}

/*
MoveDown moves the cursor down.
*/
func (explorer *Explorer) MoveDown() {
	if explorer.cursor < len(explorer.entries)-1 {
		explorer.cursor++
	}
}

/*
Enter performs the default action on the selected entry.
Returns (targetPath, isDir, loadFile).
- If loadFile is true, caller should load targetPath into the editor.
- If isDir and not loadFile, caller should descend (already done).
*/
func (explorer *Explorer) Enter() (targetPath string, isDir bool, loadFile bool) {
	if len(explorer.entries) == 0 {
		return "", false, false
	}

	name := explorer.entries[explorer.cursor]

	if name == ".." {
		parent := filepath.Dir(explorer.path)
		if parent == explorer.path {
			return "", false, false
		}
		explorer.path = parent
		explorer.refresh()
		return "", true, false
	}

	target := filepath.Join(explorer.path, name)
	if name[len(name)-1] == '/' {
		explorer.path = target
		explorer.refresh()
		return "", true, false
	}

	info, err := os.Stat(target)
	if err != nil {
		return "", false, false
	}

	if info.IsDir() {
		explorer.path = target
		explorer.refresh()
		return "", true, false
	}

	return target, false, true
}
