package advisor

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/DRAG0NM/Pare/internal/tree"
)

// coverageMinBytes is the size from which an untouched directory is worth a reminder.
const coverageMinBytes = 1 << 30

// coverageLimit caps how many untouched directories one reminder lists.
const coverageLimit = 8

// Unvisited lists the largest directories (at least coverageMinBytes) that no tool call
// and no finding has touched. A directory counts as touched when a visited path equals
// it or lies inside it; descending stops at the first untouched directory, so the list
// names the biggest blind spots rather than everything below them.
func (e *Environment) Unvisited(visited []string) []string {
	if e.Tree == nil || e.Tree.Root == nil {
		return nil
	}
	seen := make([]string, 0, len(visited))
	for _, path := range visited {
		seen = append(seen, strings.ToLower(strings.TrimRight(path, `\`)))
	}
	touched := func(node *tree.Node) bool {
		dir := strings.ToLower(strings.TrimRight(node.Path, `\`))
		for _, path := range seen {
			if path == dir || strings.HasPrefix(path, dir+`\`) {
				return true
			}
		}
		return false
	}
	var blind []*tree.Node
	var walk func(node *tree.Node)
	walk = func(node *tree.Node) {
		for _, child := range node.Children {
			if child.Kind != "directory" || child.AllocationSize < coverageMinBytes || skipCoverage(child.Name) {
				continue
			}
			if touched(child) {
				walk(child)
			} else {
				blind = append(blind, child)
			}
		}
	}
	walk(e.Tree.Root)
	sort.Slice(blind, func(i, j int) bool { return blind[i].AllocationSize > blind[j].AllocationSize })
	if len(blind) > coverageLimit {
		blind = blind[:coverageLimit]
	}
	rows := make([]string, len(blind))
	for i, node := range blind {
		rows[i] = fmt.Sprintf("%s（%s）", node.Path, size(node.AllocationSize))
	}
	return rows
}

// skipCoverage leaves out places the investigator cannot or should not act on: system
// bookkeeping, Pare's own quarantine, and personal libraries, which hold user data.
func skipCoverage(name string) bool {
	switch strings.ToLower(name) {
	case "system volume information", ".pare-quarantine", "documents", "pictures", "videos", "music", "desktop":
		return true
	}
	return false
}

// visitedPath extracts the Windows path a tool call looked at, if any.
func visitedPath(args string) string {
	var parsed struct {
		Path string `json:"path"`
	}
	if json.Unmarshal([]byte(args), &parsed) != nil || len(parsed.Path) < 2 || parsed.Path[1] != ':' {
		return ""
	}
	return parsed.Path
}

const coveragePrompt = `（系统提醒）下面这些较大的位置你还没有查看过：
%s
请用接下来的几步快速看一下（list_dir 或 breakdown 即可），有问题就用 report_finding 报告；确认没有问题就略过，不必说明。之后继续按原计划调查。`
