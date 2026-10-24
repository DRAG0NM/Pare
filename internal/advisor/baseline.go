package advisor

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// baseline runs a few quick, read-only system checks before the model starts, so
// system-level waste (non-sparse virtual disks, duplicate driver packages, shadow copies)
// is always on the table instead of depending on the model remembering to look.
func (e *Environment) baseline(ctx context.Context) []string {
	if e.SkipSystemChecks {
		return nil
	}
	var rows []string
	run := func(argv ...string) (string, bool) {
		runCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		text, err := runCommand(runCtx, argv)
		return text, err == nil && strings.TrimSpace(text) != ""
	}

	disks := 0
	for _, node := range e.nodes() {
		if node.Kind != "file" || !virtualDiskExt[strings.ToLower(filepath.Ext(node.Path))] || node.AllocationSize < 512*mib || disks == 5 {
			continue
		}
		disks++
		state := "未能检查是否为稀疏文件"
		if text, ok := run("fsutil.exe", "sparse", "queryflag", node.Path); ok {
			state = "fsutil 输出：" + firstLine(text)
		}
		rows = append(rows, fmt.Sprintf("- 虚拟磁盘 %s（%s）：%s", node.Path, size(uint64(node.AllocationSize)), state))
	}
	if text, ok := run("wsl.exe", "--list", "--verbose"); ok {
		rows = append(rows, "- WSL 发行版（wsl --list --verbose）："+strings.Join(nonEmptyLines(text), "；"))
	}
	if text, ok := run("pnputil.exe", "/enum-drivers"); ok {
		for _, line := range duplicateDrivers(text) {
			rows = append(rows, "- 重复驱动包："+line)
		}
	}
	if text, ok := run("vssadmin.exe", "list", "shadowstorage"); ok {
		var used []string
		for _, line := range nonEmptyLines(text) {
			if strings.Contains(line, "已用") || strings.Contains(line, "已使用") || strings.Contains(line, "Used Shadow") || strings.Contains(line, "卷影副本存储卷") || strings.Contains(line, "For volume") {
				used = append(used, line)
			}
		}
		if len(used) != 0 {
			rows = append(rows, "- 卷影副本存储："+strings.Join(used, "；"))
		}
	}
	return rows
}

func firstLine(text string) string {
	if lines := nonEmptyLines(text); len(lines) != 0 {
		return lines[0]
	}
	return ""
}

func nonEmptyLines(text string) []string {
	var result []string
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	return result
}

var (
	publishedLine = regexp.MustCompile(`^(?:Published Name|发布名称)\s*[:：]\s*(\S+)`)
	originalLine  = regexp.MustCompile(`^(?:Original Name|原始名称)\s*[:：]\s*(\S+)`)
	versionLine   = regexp.MustCompile(`^(?:Driver Version|驱动程序版本)\s*[:：]\s*(\S+)`)
)

// duplicateDrivers groups `pnputil /enum-drivers` packages by original INF name and lists
// names installed more than once, oldest first. Only the newest package of a device is
// normally in use; older ones are update leftovers worth checking.
func duplicateDrivers(text string) []string {
	type pkg struct{ published, date string }
	groups := map[string][]pkg{}
	var published, original, date string
	flush := func() {
		if published != "" && original != "" {
			groups[strings.ToLower(original)] = append(groups[strings.ToLower(original)], pkg{published, date})
		}
		published, original, date = "", "", ""
	}
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if match := publishedLine.FindStringSubmatch(line); match != nil {
			flush()
			published = match[1]
		} else if match := originalLine.FindStringSubmatch(line); match != nil {
			original = match[1]
		} else if match := versionLine.FindStringSubmatch(line); match != nil {
			date = match[1]
		}
	}
	flush()

	var rows []string
	for name, pkgs := range groups {
		if len(pkgs) < 2 {
			continue
		}
		sort.Slice(pkgs, func(i, j int) bool { return sortableDate(pkgs[i].date) < sortableDate(pkgs[j].date) })
		parts := make([]string, len(pkgs))
		for i, p := range pkgs {
			parts[i] = p.published + " " + p.date
		}
		rows = append(rows, fmt.Sprintf("%s 共 %d 个版本（%s）", name, len(pkgs), strings.Join(parts, "、")))
	}
	sort.Strings(rows)
	return rows
}

// sortableDate turns pnputil's MM/DD/YYYY into YYYYMMDD; other formats sort as given.
func sortableDate(date string) string {
	parts := strings.Split(date, "/")
	if len(parts) == 3 && len(parts[2]) == 4 {
		return parts[2] + parts[0] + parts[1]
	}
	return date
}
