package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

/*
thinkcursionBackend implements provider.DiscussionToolBackend
so that thinkcursion agents can read files, browse directories,
and use shared memory during their discussion loops.
*/
type thinkcursionBackend struct {
	root   string
	memory []string
	mu     sync.Mutex
}

func (tk *thinkcursionBackend) resolve(target string) (string, bool) {
	if target == "" {
		target = "."
	}
	resolved := filepath.Clean(filepath.Join(tk.root, target))
	if !strings.HasPrefix(resolved, tk.root) {
		return "", false
	}
	return resolved, true
}

func (tk *thinkcursionBackend) Browse(target string) string {
	resolved, ok := tk.resolve(target)
	if !ok {
		return "Blocked: path escapes workspace."
	}
	entries, err := os.ReadDir(resolved)
	if err != nil {
		return err.Error()
	}
	var dirs, files []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name()+"/")
		} else {
			files = append(files, e.Name())
		}
	}
	return fmt.Sprintf("Directories: %v\nFiles: %v", dirs, files)
}

func (tk *thinkcursionBackend) Read(target string, start, end int) string {
	resolved, ok := tk.resolve(target)
	if !ok {
		return "Blocked: path escapes workspace."
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return err.Error()
	}
	lines := strings.Split(string(data), "\n")
	if start > 0 {
		start--
	}
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	if start >= len(lines) || start >= end {
		return "Invalid line range"
	}
	return strings.Join(lines[start:end], "\n")
}

func (tk *thinkcursionBackend) Remember(content string) string {
	tk.mu.Lock()
	defer tk.mu.Unlock()
	
	tk.memory = append(tk.memory, content)
	return "Memory stored."
}

func (tk *thinkcursionBackend) Recall(filter string) string {
	tk.mu.Lock()
	defer tk.mu.Unlock()
	
	var matches []string
	lower := strings.ToLower(filter)
	for _, m := range tk.memory {
		if strings.Contains(strings.ToLower(m), lower) {
			matches = append(matches, m)
		}
	}
	if len(matches) == 0 {
		return "Memory recall: no matches."
	}
	
	return "Memory recall:\n" + strings.Join(matches, "\n")
}

func (tk *thinkcursionBackend) Forget(filter string) string {
	tk.mu.Lock()
	defer tk.mu.Unlock()
	
	var keep []string
	lower := strings.ToLower(filter)
	for _, m := range tk.memory {
		if !strings.Contains(strings.ToLower(m), lower) {
			keep = append(keep, m)
		}
	}
	tk.memory = keep
	return fmt.Sprintf("System: Memory items matching '%s' erased.", filter)
}

func (tk *thinkcursionBackend) Search(query, target string) string {
	resolved, ok := tk.resolve(target)
	if !ok {
		return "Blocked: path escapes workspace."
	}

	cmdGit := exec.Command("git", "grep", "-I", "-n", query)
	cmdGit.Dir = resolved
	if out, err := cmdGit.CombinedOutput(); err == nil {
		res := string(out)
		if len(res) > 4000 {
			res = res[:4000] + "\n... (truncated to 4000 characters)"
		}
		if len(res) == 0 {
			return "Search returned 0 results."
		}
		return res
	}

	cmdGrep := exec.Command("grep", "-rn", query, resolved)
	outGrep, errGrep := cmdGrep.CombinedOutput()
	if errGrep != nil && len(outGrep) == 0 {
		return "Search found nothing or failed."
	}

	res := string(outGrep)
	if len(res) > 4000 {
		res = res[:4000] + "\n... (truncated to 4000 characters)"
	}
	if len(res) == 0 {
		return "Search returned 0 results."
	}
	
	return res
}
