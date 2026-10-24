package advisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/tree"
)

// Environment is what the tools can see: the scan, its tree and the live file system.
type Environment struct {
	Root       string
	Report     model.ScanReport
	Tree       *tree.Tree
	VolumeUsed uint64
	// Consent asks the user before content of a sensitive path is read. Nil denies.
	Consent func(ctx context.Context, path, reason string) bool
	Now     func() time.Time

	sorted []model.Node
}

const mib = 1 << 20

func (e *Environment) nodes() []model.Node {
	if e.sorted == nil {
		e.sorted = e.Report.Nodes
		if !sort.SliceIsSorted(e.sorted, func(i, j int) bool { return e.sorted[i].Path < e.sorted[j].Path }) {
			e.sorted = append([]model.Node(nil), e.Report.Nodes...)
			sort.Slice(e.sorted, func(i, j int) bool { return e.sorted[i].Path < e.sorted[j].Path })
		}
	}
	return e.sorted
}

// under returns the scanned nodes inside dir (any depth). Sorted paths keep a subtree
// contiguous, so this is two binary searches.
func (e *Environment) under(dir string) []model.Node {
	nodes := e.nodes()
	prefix := strings.TrimRight(dir, `\`) + `\`
	start := sort.Search(len(nodes), func(i int) bool { return nodes[i].Path >= prefix })
	end := start + sort.Search(len(nodes)-start, func(i int) bool { return !strings.HasPrefix(nodes[start+i].Path, prefix) })
	return nodes[start:end]
}

func (e *Environment) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func (e *Environment) scope(path string) string {
	if path == "" {
		return e.Root
	}
	return filepath.Clean(path)
}

func size(n uint64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(n)/(1<<30))
	case n >= mib:
		return fmt.Sprintf("%.1f MB", float64(n)/mib)
	default:
		return fmt.Sprintf("%.0f KB", float64(n)/1024)
	}
}

func decode[T any](args json.RawMessage) (T, error) {
	var value T
	if len(args) != 0 {
		if err := json.Unmarshal(args, &value); err != nil {
			return value, fmt.Errorf("参数格式不对：%w", err)
		}
	}
	return value, nil
}

func lines(rows []string) string {
	if len(rows) == 0 {
		return "（没有结果）"
	}
	return strings.Join(rows, "\n")
}

// Tools returns every investigation tool. All of them are read-only.
func (e *Environment) Tools() []Tool {
	return []Tool{
		{
			Name:        "scan_overview",
			Description: "扫描总览：总占用、顶层目录、以及本地预先算出的异常线索。调查开始时先调用一次。",
			Parameters:  `{"type":"object","properties":{}}`,
			Run:         func(context.Context, json.RawMessage) (string, error) { return e.overview(), nil },
		},
		{
			Name:        "list_dir",
			Description: "列出目录的子项（按占用从大到小），包含文件数和是否有读不到的部分。不在扫描结果里的目录会现场读取。",
			Parameters:  `{"type":"object","properties":{"path":{"type":"string"},"limit":{"type":"integer","description":"默认 40，最多 200"}},"required":["path"]}`,
			Run:         e.listDir,
		},
		{
			Name:        "find_large_files",
			Description: "找出目录下（含子目录）大于指定大小的文件，按大小排序。",
			Parameters:  `{"type":"object","properties":{"path":{"type":"string"},"min_mb":{"type":"number","description":"默认 100"},"limit":{"type":"integer","description":"默认 40"}},"required":["path"]}`,
			Run:         e.largeFiles,
		},
		{
			Name:        "same_size_files",
			Description: "找出目录下大小完全相同、成批出现的文件。大量同尺寸文件常见于更新器反复下载、转储或缓存泄漏。",
			Parameters:  `{"type":"object","properties":{"path":{"type":"string"},"min_mb":{"type":"number","description":"默认 5"},"min_count":{"type":"integer","description":"默认 5"}},"required":["path"]}`,
			Run:         e.sameSize,
		},
		{
			Name:        "breakdown",
			Description: "目录下文件的扩展名分布和修改时间分布（7 天/30 天/90 天/1 年/更久），以及最新最旧的文件。",
			Parameters:  `{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`,
			Run:         e.breakdown,
		},
		{
			Name:        "file_head",
			Description: "读取文件开头的字节，返回十六进制和其中可读的字符串，用来辨认无扩展名文件的真实类型和来源。",
			Parameters:  `{"type":"object","properties":{"path":{"type":"string"},"bytes":{"type":"integer","description":"默认 512，最多 4096"}},"required":["path"]}`,
			Run:         e.fileHead,
		},
		{
			Name:        "read_text",
			Description: "读取文本文件（配置、日志、清单等）开头的内容。",
			Parameters:  `{"type":"object","properties":{"path":{"type":"string"},"max_kb":{"type":"integer","description":"默认 8，最多 64"}},"required":["path"]}`,
			Run:         e.readText,
		},
		{
			Name:        "path_info",
			Description: "现场查看一个文件或目录：是否还存在、当前大小（目录会实时统计）、修改时间、属性，文件是否正被其他程序占用。清理前用它确认最新状态。",
			Parameters:  `{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`,
			Run:         e.pathInfo,
		},
		{
			Name:        "installed_programs",
			Description: "列出已安装的软件（名称、版本、发布者、安装位置、大小）。filter 按名称或发布者筛选。",
			Parameters:  `{"type":"object","properties":{"filter":{"type":"string"}}}`,
			Run:         e.installed,
		},
		{
			Name:        "running_processes",
			Description: "列出正在运行的进程和可执行文件路径。filter 按名称或路径筛选。",
			Parameters:  `{"type":"object","properties":{"filter":{"type":"string"}}}`,
			Run:         e.processes,
		},
		{
			Name:        "run_diagnostic",
			Description: "运行白名单里的只读诊断命令。可用的 name：" + diagnosticNames() + "。",
			Parameters:  `{"type":"object","properties":{"name":{"type":"string"},"distro":{"type":"string","description":"WSL 发行版名"},"path":{"type":"string","description":"需要路径的诊断用；WSL 内用 Linux 路径"}},"required":["name"]}`,
			Run:         e.diagnostic,
		},
	}
}

func (e *Environment) overview() string {
	rows := []string{fmt.Sprintf("扫描根目录：%s；扫描是否完整：%v；读不到的位置：%d 处", e.Root, e.Report.Complete, len(e.Report.Errors))}
	if e.Tree != nil {
		root := e.Tree.Root
		rows = append(rows, fmt.Sprintf("按路径统计占用 %s，按文件去重后 %s，文件 %d 个，文件夹 %d 个", size(root.AllocationSize), size(e.Tree.UniqueAllocation), root.Files, root.Directories))
		if e.VolumeUsed > 0 {
			rows = append(rows, fmt.Sprintf("所在卷已用 %s（与扫描到的差额通常是系统保留、NTFS 元数据、卷影副本）", size(e.VolumeUsed)))
		}
		rows = append(rows, "", "顶层目录：")
		for i, child := range root.Children {
			if i == 20 {
				break
			}
			rows = append(rows, fmt.Sprintf("  %s  %s（%d 个文件）", size(child.AllocationSize), child.Path, child.Files))
		}
	}
	rows = append(rows, "", "异常线索（本地规则算出，需要你核实）：")
	rows = append(rows, e.hints()...)
	return lines(rows)
}

func (e *Environment) listDir(_ context.Context, raw json.RawMessage) (string, error) {
	args, err := decode[struct {
		Path  string `json:"path"`
		Limit int    `json:"limit"`
	}](raw)
	if err != nil {
		return "", err
	}
	limit := args.Limit
	if limit <= 0 || limit > 200 {
		limit = 40
	}
	path := e.scope(args.Path)
	if e.Tree != nil {
		if node := e.Tree.Find(path); node != nil {
			rows := []string{fmt.Sprintf("%s：%s，%d 个文件，%d 个子目录%s", path, size(node.AllocationSize), node.Files, node.Directories, incompleteNote(node.Incomplete))}
			for i, child := range node.Children {
				if i == limit {
					rows = append(rows, fmt.Sprintf("……还有 %d 项", len(node.Children)-limit))
					break
				}
				rows = append(rows, fmt.Sprintf("  %-9s %s  %s%s", child.Kind, size(child.AllocationSize), child.Name, countNote(child)))
			}
			return lines(rows), nil
		}
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}
	rows := []string{path + "（不在扫描结果中，现场读取，目录大小未统计）"}
	for i, entry := range entries {
		if i == limit {
			rows = append(rows, fmt.Sprintf("……还有 %d 项", len(entries)-limit))
			break
		}
		info, err := entry.Info()
		if err != nil {
			rows = append(rows, "  ?  "+entry.Name())
			continue
		}
		kind := "file"
		if entry.IsDir() {
			kind = "directory"
		}
		rows = append(rows, fmt.Sprintf("  %-9s %s  %s  %s", kind, size(uint64(max(info.Size(), 0))), info.ModTime().Format("2006-01-02"), entry.Name()))
	}
	return lines(rows), nil
}

func incompleteNote(incomplete bool) string {
	if incomplete {
		return "（其中有读不到的位置，大小是下限）"
	}
	return ""
}

func countNote(node *tree.Node) string {
	note := ""
	if node.Kind == "directory" {
		note = fmt.Sprintf("（%d 个文件）", node.Files)
	}
	if node.Incomplete {
		note += "（不完整）"
	}
	return note
}

func (e *Environment) largeFiles(_ context.Context, raw json.RawMessage) (string, error) {
	args, err := decode[struct {
		Path  string  `json:"path"`
		MinMB float64 `json:"min_mb"`
		Limit int     `json:"limit"`
	}](raw)
	if err != nil {
		return "", err
	}
	if args.MinMB <= 0 {
		args.MinMB = 100
	}
	if args.Limit <= 0 || args.Limit > 200 {
		args.Limit = 40
	}
	threshold := model.ByteCount(args.MinMB * mib)
	var found []model.Node
	for _, node := range e.under(e.scope(args.Path)) {
		if node.Kind == "file" && node.AllocationSize >= threshold {
			found = append(found, node)
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].AllocationSize > found[j].AllocationSize })
	rows := []string{fmt.Sprintf("共 %d 个文件 ≥ %.0f MB", len(found), args.MinMB)}
	for i, node := range found {
		if i == args.Limit {
			break
		}
		rows = append(rows, fmt.Sprintf("  %s  %s  %s", size(uint64(node.AllocationSize)), node.LastWriteTime.Format("2006-01-02"), node.Path))
	}
	return lines(rows), nil
}

type cluster struct {
	size    model.ByteCount
	count   int
	dirs    map[string]int
	samples []string
	newest  time.Time
	oldest  time.Time
}

func (e *Environment) clusters(nodes []model.Node, minSize model.ByteCount, minCount int) []*cluster {
	bySize := map[model.ByteCount]*cluster{}
	for _, node := range nodes {
		if node.Kind != "file" || node.LogicalSize < minSize {
			continue
		}
		c := bySize[node.LogicalSize]
		if c == nil {
			c = &cluster{size: node.LogicalSize, dirs: map[string]int{}, oldest: node.LastWriteTime}
			bySize[node.LogicalSize] = c
		}
		c.count++
		c.dirs[node.ParentPath]++
		if len(c.samples) < 3 {
			c.samples = append(c.samples, filepath.Base(node.Path))
		}
		if node.LastWriteTime.After(c.newest) {
			c.newest = node.LastWriteTime
		}
		if node.LastWriteTime.Before(c.oldest) {
			c.oldest = node.LastWriteTime
		}
	}
	var result []*cluster
	for _, c := range bySize {
		if c.count >= minCount {
			result = append(result, c)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return uint64(result[i].size)*uint64(result[i].count) > uint64(result[j].size)*uint64(result[j].count)
	})
	return result
}

func (c *cluster) describe() string {
	type dirCount struct {
		dir   string
		count int
	}
	var dirs []dirCount
	for dir, count := range c.dirs {
		dirs = append(dirs, dirCount{dir, count})
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].count > dirs[j].count })
	where := dirs[0].dir
	if len(dirs) > 1 {
		where += fmt.Sprintf(" 等 %d 个目录", len(dirs))
	}
	return fmt.Sprintf("%d 个文件都是 %s（合计 %s），%s ~ %s，位于 %s，例如 %s",
		c.count, size(uint64(c.size)), size(uint64(c.size)*uint64(c.count)), c.oldest.Format("2006-01-02"), c.newest.Format("2006-01-02"), where, strings.Join(c.samples, "、"))
}

func (e *Environment) sameSize(_ context.Context, raw json.RawMessage) (string, error) {
	args, err := decode[struct {
		Path     string  `json:"path"`
		MinMB    float64 `json:"min_mb"`
		MinCount int     `json:"min_count"`
	}](raw)
	if err != nil {
		return "", err
	}
	if args.MinMB <= 0 {
		args.MinMB = 5
	}
	if args.MinCount <= 1 {
		args.MinCount = 5
	}
	var rows []string
	for i, c := range e.clusters(e.under(e.scope(args.Path)), model.ByteCount(args.MinMB*mib), args.MinCount) {
		if i == 20 {
			break
		}
		rows = append(rows, c.describe())
	}
	return lines(rows), nil
}

func (e *Environment) breakdown(_ context.Context, raw json.RawMessage) (string, error) {
	args, err := decode[struct {
		Path string `json:"path"`
	}](raw)
	if err != nil {
		return "", err
	}
	now := e.now()
	type bucket struct {
		count int
		bytes uint64
	}
	extensions := map[string]*bucket{}
	ages := []struct {
		label string
		limit time.Duration
		bucket
	}{{label: "7 天内", limit: 7 * 24 * time.Hour}, {label: "30 天内", limit: 30 * 24 * time.Hour}, {label: "90 天内", limit: 90 * 24 * time.Hour}, {label: "1 年内", limit: 365 * 24 * time.Hour}, {label: "1 年以上", limit: 1 << 62}}
	var newest, oldest model.Node
	files := 0
	for _, node := range e.under(e.scope(args.Path)) {
		if node.Kind != "file" {
			continue
		}
		files++
		ext := strings.ToLower(filepath.Ext(node.Path))
		if ext == "" {
			ext = "（无扩展名）"
		}
		if extensions[ext] == nil {
			extensions[ext] = &bucket{}
		}
		extensions[ext].count++
		extensions[ext].bytes += uint64(node.AllocationSize)
		age := now.Sub(node.LastWriteTime)
		for i := range ages {
			if age < ages[i].limit {
				ages[i].count++
				ages[i].bytes += uint64(node.AllocationSize)
				break
			}
		}
		if files == 1 || node.LastWriteTime.After(newest.LastWriteTime) {
			newest = node
		}
		if files == 1 || node.LastWriteTime.Before(oldest.LastWriteTime) {
			oldest = node
		}
	}
	if files == 0 {
		return "目录下没有扫描到文件", nil
	}
	type extRow struct {
		ext string
		*bucket
	}
	var exts []extRow
	for ext, b := range extensions {
		exts = append(exts, extRow{ext, b})
	}
	sort.Slice(exts, func(i, j int) bool { return exts[i].bytes > exts[j].bytes })
	rows := []string{fmt.Sprintf("共 %d 个文件。扩展名（按占用）：", files)}
	for i, row := range exts {
		if i == 15 {
			break
		}
		rows = append(rows, fmt.Sprintf("  %s  %d 个  %s", size(row.bytes), row.count, row.ext))
	}
	rows = append(rows, "最后修改时间：")
	for _, age := range ages {
		rows = append(rows, fmt.Sprintf("  %s：%d 个，%s", age.label, age.count, size(age.bytes)))
	}
	rows = append(rows, "最新："+newest.LastWriteTime.Format("2006-01-02 15:04")+" "+newest.Path, "最旧："+oldest.LastWriteTime.Format("2006-01-02 15:04")+" "+oldest.Path)
	return lines(rows), nil
}

var errDenied = errors.New("用户没有同意读取这个敏感文件的内容（可以只根据文件名和大小判断）")

func (e *Environment) allowContent(ctx context.Context, path, reason string) error {
	if !IsSensitive(path) {
		return nil
	}
	if e.Consent != nil && e.Consent(ctx, path, reason) {
		return nil
	}
	return errDenied
}

func (e *Environment) fileHead(ctx context.Context, raw json.RawMessage) (string, error) {
	args, err := decode[struct {
		Path  string `json:"path"`
		Bytes int    `json:"bytes"`
	}](raw)
	if err != nil {
		return "", err
	}
	if args.Bytes <= 0 || args.Bytes > 4096 {
		args.Bytes = 512
	}
	if err := e.allowContent(ctx, args.Path, "读取文件开头以辨认类型"); err != nil {
		return "", err
	}
	data, err := readHead(args.Path, args.Bytes)
	if err != nil {
		return "", err
	}
	hex := fmt.Sprintf("% x", data[:min(len(data), 64)])
	return lines([]string{"前 64 字节：" + hex, "可读字符串：" + strings.Join(printable(data, 4), " | ")}), nil
}

// printable extracts runs of printable ASCII, like the strings(1) tool.
func printable(data []byte, minLen int) []string {
	var result []string
	start := -1
	for i := 0; i <= len(data); i++ {
		if i < len(data) && data[i] >= 0x20 && data[i] < 0x7f {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 && i-start >= minLen {
			result = append(result, string(data[start:i]))
		}
		start = -1
	}
	return result
}

func readHead(path string, n int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	buffer := make([]byte, n)
	read, err := file.Read(buffer)
	if err != nil && read == 0 {
		return nil, err
	}
	return buffer[:read], nil
}

func (e *Environment) readText(ctx context.Context, raw json.RawMessage) (string, error) {
	args, err := decode[struct {
		Path  string `json:"path"`
		MaxKB int    `json:"max_kb"`
	}](raw)
	if err != nil {
		return "", err
	}
	if args.MaxKB <= 0 || args.MaxKB > 64 {
		args.MaxKB = 8
	}
	if err := e.allowContent(ctx, args.Path, "读取文本内容"); err != nil {
		return "", err
	}
	data, err := readHead(args.Path, args.MaxKB*1024)
	if err != nil {
		return "", err
	}
	return decodeText(data), nil
}

func (e *Environment) pathInfo(ctx context.Context, raw json.RawMessage) (string, error) {
	args, err := decode[struct {
		Path string `json:"path"`
	}](raw)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(args.Path)
	if errors.Is(err, os.ErrNotExist) {
		return args.Path + " 已经不存在", nil
	}
	if err != nil {
		return "", err
	}
	rows := []string{fmt.Sprintf("%s：修改时间 %s，属性 %s", args.Path, info.ModTime().Format("2006-01-02 15:04:05"), info.Mode())}
	if info.IsDir() {
		total, files, truncated := measure(ctx, args.Path, 300000)
		note := ""
		if truncated {
			note = "（条目太多或超时，统计不完整）"
		}
		rows = append(rows, fmt.Sprintf("目录当前大小 %s，%d 个文件%s", size(total), files, note))
	} else {
		rows = append(rows, "文件大小 "+size(uint64(info.Size())), "占用状态："+lockState(args.Path))
	}
	return lines(rows), nil
}

// measure sums file sizes under dir, stopping after limit entries or when ctx ends.
func measure(ctx context.Context, dir string, limit int) (total uint64, files int, truncated bool) {
	seen := 0
	stop := errors.New("stop")
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		seen++
		if seen > limit || ctx.Err() != nil {
			truncated = true
			return stop
		}
		// WalkDir does not descend into links, so reparse points are never followed.
		if err != nil || !entry.Type().IsRegular() {
			return nil
		}
		if info, err := entry.Info(); err == nil {
			total += uint64(max(info.Size(), 0))
			files++
		}
		return nil
	})
	return total, files, truncated
}

func (e *Environment) installed(_ context.Context, raw json.RawMessage) (string, error) {
	args, err := decode[struct {
		Filter string `json:"filter"`
	}](raw)
	if err != nil {
		return "", err
	}
	programs, err := installedPrograms()
	if err != nil {
		return "", err
	}
	filter := strings.ToLower(args.Filter)
	var rows []string
	for _, p := range programs {
		if filter != "" && !strings.Contains(strings.ToLower(p), filter) {
			continue
		}
		rows = append(rows, p)
		if len(rows) == 150 {
			rows = append(rows, "……结果太多，请用 filter 缩小范围")
			break
		}
	}
	return lines(rows), nil
}

func (e *Environment) processes(_ context.Context, raw json.RawMessage) (string, error) {
	args, err := decode[struct {
		Filter string `json:"filter"`
	}](raw)
	if err != nil {
		return "", err
	}
	list, err := runningProcesses()
	if err != nil {
		return "", err
	}
	filter := strings.ToLower(args.Filter)
	var rows []string
	for _, p := range list {
		if filter == "" || strings.Contains(strings.ToLower(p), filter) {
			rows = append(rows, p)
		}
	}
	return lines(rows), nil
}
