package advisor

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/tree"
)

var (
	virtualDiskExt = map[string]bool{".vhdx": true, ".vhd": true, ".vmdk": true, ".vdi": true, ".qcow2": true, ".avhdx": true}
	dumpExt        = map[string]bool{".dmp": true, ".hdmp": true, ".mdmp": true}
	versionName    = regexp.MustCompile(`^v?\d+(\.\d+){1,3}([-+._][0-9A-Za-z.]+)?$`)
	platformName   = regexp.MustCompile(`(?i)(darwin|linux|win32|windows|android|freebsd|macos)[-_](x64|x86_64|arm64|ia32|arm|aarch64|universal)`)
)

type treeNode = tree.Node

type pathTotal struct {
	path  string
	bytes uint64
	count int
}

func topTotals(totals map[string]*pathTotal, minBytes uint64, limit int) []*pathTotal {
	var list []*pathTotal
	for _, total := range totals {
		if total.bytes >= minBytes {
			list = append(list, total)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].bytes > list[j].bytes })
	if len(list) > limit {
		list = list[:limit]
	}
	return list
}

func add(totals map[string]*pathTotal, key string, bytes uint64) {
	if totals[key] == nil {
		totals[key] = &pathTotal{path: key}
	}
	totals[key].bytes += bytes
	totals[key].count++
}

// hints lists patterns worth a closer look. They are leads, not verdicts: the prompt tells
// the model to verify each one with tools before reporting it.
func (e *Environment) hints() []string {
	nodes := e.nodes()
	var rows []string

	var big []model.Node
	disks := map[string]*pathTotal{}
	dumps := map[string]*pathTotal{}
	zeros := map[string]*pathTotal{}
	platforms := map[string]*pathTotal{}
	for _, node := range nodes {
		if node.Kind != "file" {
			continue
		}
		bytes := uint64(node.AllocationSize)
		ext := strings.ToLower(filepath.Ext(node.Path))
		if bytes >= 1<<30 {
			big = append(big, node)
		}
		if virtualDiskExt[ext] && bytes >= 512*mib {
			add(disks, node.Path, bytes)
		}
		if dumpExt[ext] {
			add(dumps, node.ParentPath, bytes)
		}
		if node.LogicalSize == 0 {
			add(zeros, node.ParentPath, 0)
		}
		if strings.Contains(strings.ToLower(node.Path), `\node_modules\`) {
			if match := platformName.FindStringIndex(node.Path); match != nil {
				// Group by the folder that holds the platform-specific packages.
				end := strings.LastIndex(node.Path[:match[0]], `\`)
				add(platforms, node.Path[:max(end, 0)], bytes)
			}
		}
	}

	sort.Slice(big, func(i, j int) bool { return big[i].AllocationSize > big[j].AllocationSize })
	for i, node := range big {
		if i == 10 {
			break
		}
		rows = append(rows, fmt.Sprintf("- 超大单文件：%s %s（修改于 %s）", size(uint64(node.AllocationSize)), node.Path, node.LastWriteTime.Format("2006-01-02")))
	}
	for _, disk := range topTotals(disks, 0, 10) {
		rows = append(rows, fmt.Sprintf("- 虚拟磁盘：%s %s（虚拟磁盘通常只增不减，删掉里面的文件不会自动缩小）", size(disk.bytes), disk.path))
	}
	for _, c := range e.clusters(nodes, 10*mib, 8) {
		if uint64(c.size)*uint64(c.count) < 500*mib {
			break
		}
		rows = append(rows, "- 同尺寸文件扎堆："+c.describe())
		if len(rows) > 40 {
			break
		}
	}
	for _, zero := range topTotals(zeros, 0, 20) {
		if zero.count < 300 {
			continue
		}
		rows = append(rows, fmt.Sprintf("- 大量零字节文件：%s 里有 %d 个（常见于程序反复创建临时文件后没清理）", zero.path, zero.count))
	}
	for _, dump := range topTotals(dumps, 100*mib, 10) {
		rows = append(rows, fmt.Sprintf("- 崩溃转储：%s 里 %d 个 dump，合计 %s", dump.path, dump.count, size(dump.bytes)))
	}
	for _, platform := range topTotals(platforms, 100*mib, 10) {
		rows = append(rows, fmt.Sprintf("- 多平台原生二进制：%s 下有为其他系统/架构准备的包，合计 %s（本机通常只需要 win32-x64 或 linux-x64 一份）", platform.path, size(platform.bytes)))
	}
	rows = append(rows, e.versionHints()...)
	if len(rows) == 0 {
		rows = append(rows, "- 没有命中本地异常规则；请从占用最大的目录入手调查")
	}
	return rows
}

// versionHints finds folders that keep several versions side by side (old app versions
// that updaters forgot to remove).
func (e *Environment) versionHints() []string {
	if e.Tree == nil {
		return nil
	}
	var rows []string
	var visit func(depth int, children []*treeNode)
	visit = func(depth int, children []*treeNode) {
		if depth > 7 || len(rows) >= 10 {
			return
		}
		for _, node := range children {
			versions, total := 0, uint64(0)
			for _, child := range node.Children {
				if child.Kind == "directory" && versionName.MatchString(child.Name) {
					versions++
					total += child.AllocationSize
				}
			}
			if versions >= 3 && total >= 300*mib {
				rows = append(rows, fmt.Sprintf("- 多版本并存：%s 下有 %d 个版本目录，合计 %s（旧版本可能是更新残留，也可能仍被其他程序依赖，需核实）", node.Path, versions, size(total)))
				continue
			}
			if node.AllocationSize >= 300*mib {
				visit(depth+1, node.Children)
			}
		}
	}
	visit(0, e.Tree.Root.Children)
	return rows
}
