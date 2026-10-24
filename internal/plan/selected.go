package plan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/probes"
)

const (
	KindProtection = "protection"
	KindSource     = "source"
	KindExists     = "exists"

	// SelectedRulesVersion marks plans built from paths the AI or the user picked.
	SelectedRulesVersion = "selected/1"
)

// Selection is one path to quarantine and why it was picked, for example
// "AI 建议：罗技 G HUB 更新包残留" or "手动添加".
type Selection struct {
	Path   string
	Source string
}

var errNoSelection = errors.New("没有选择任何路径")

// BuildSelected turns AI-suggested or hand-picked paths into a plan. Unlike rule plans,
// a selected path may be a whole directory. Every item still needs the same kind of
// evidence before it is executable: not protected, not a system file, a known identity,
// and a live check that it is unchanged (files: also not in use; directories: the
// rename fails while anything inside is open).
func BuildSelected(ctx context.Context, scan model.ScanReport, selections []Selection, prober probes.Prober, now time.Time, ttl time.Duration) (model.Plan, error) {
	if len(selections) == 0 {
		return model.Plan{}, errNoSelection
	}
	now = now.UTC().Truncate(time.Second)
	byPath := make(map[string]model.Node, len(scan.Nodes))
	paths := map[string]int{}
	for _, node := range scan.Nodes {
		byPath[strings.ToLower(node.Path)] = node
		if node.Kind == "file" && node.FileID != "" {
			paths[node.FileID]++
		}
	}

	items := []model.PlanItem{}
	seen := map[string]bool{}
	for _, selection := range selections {
		if err := ctx.Err(); err != nil {
			return model.Plan{}, err
		}
		clean := filepath.Clean(selection.Path)
		if seen[strings.ToLower(clean)] {
			continue
		}
		seen[strings.ToLower(clean)] = true
		items = append(items, selectedItem(ctx, scan, byPath, paths, clean, selection.Source, prober))
	}
	dropNested(items)
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })

	result := model.Plan{
		Schema:       model.PlanSchema,
		CreatedAt:    now,
		ExpiresAt:    now.Add(ttl),
		Volume:       scan.Volume,
		Scan:         model.PlanSource{StartedAt: scan.StartedAt, CompletedAt: scan.CompletedAt, Complete: scan.Complete},
		RulesVersion: SelectedRulesVersion,
		Items:        items,
		Totals:       totals(items),
	}
	digest, err := Digest(result)
	if err != nil {
		return model.Plan{}, err
	}
	result.Digest = digest
	return result, nil
}

func selectedItem(ctx context.Context, scan model.ScanReport, byPath map[string]model.Node, paths map[string]int, path, source string, prober probes.Prober) model.PlanItem {
	item := model.PlanItem{Path: path, RuleID: "selected", RuleVersion: 1, Risk: "E2", Action: ActionQuarantine}
	item.Evidence = append(item.Evidence, model.Evidence{Kind: KindSource, Status: model.EvidencePass, Detail: source})

	if protected, reason := Protected(path); protected {
		item.Evidence = append(item.Evidence, model.Evidence{Kind: KindProtection, Status: model.EvidenceFail, Detail: "受保护的位置：" + reason})
		return item
	}
	item.Evidence = append(item.Evidence, model.Evidence{Kind: KindProtection, Status: model.EvidencePass, Detail: "不在受保护的位置"})

	node, found := byPath[strings.ToLower(path)]
	if !found {
		item.Evidence = append(item.Evidence, model.Evidence{Kind: KindFileIdentity, Status: model.EvidenceUnknown, Detail: "扫描结果里没有这个路径，请重新扫描后再选"})
		return item
	}
	item.Path, item.FileID, item.Attributes, item.LastWriteTime = node.Path, node.FileID, node.Attributes, node.LastWriteTime

	if node.Attributes&fileAttributeSystem != 0 {
		item.Evidence = append(item.Evidence, model.Evidence{Kind: KindAttributes, Status: model.EvidenceFail, Detail: "system file"})
	} else {
		item.Evidence = append(item.Evidence, model.Evidence{Kind: KindAttributes, Status: model.EvidencePass, Detail: "not a system file"})
	}

	switch node.Kind {
	case "directory":
		item.Kind = "directory"
		item.LogicalSize, item.AllocationSize = sumUnder(scan.Nodes, node.Path)
		if node.FileID == "" {
			item.Evidence = append(item.Evidence, model.Evidence{Kind: KindFileIdentity, Status: model.EvidenceUnknown, Detail: "scan has no file ID"})
		} else {
			item.Evidence = append(item.Evidence, model.Evidence{Kind: KindFileIdentity, Status: model.EvidencePass, Detail: "目录身份已记录"})
		}
		item.Evidence = append(item.Evidence, directoryExists(node.Path))
	case "file":
		item.LogicalSize, item.AllocationSize = node.LogicalSize, node.AllocationSize
		switch {
		case node.FileID == "":
			item.Evidence = append(item.Evidence, model.Evidence{Kind: KindFileIdentity, Status: model.EvidenceUnknown, Detail: "scan has no file ID"})
		case paths[node.FileID] > 1:
			item.Evidence = append(item.Evidence, model.Evidence{Kind: KindFileIdentity, Status: model.EvidenceFail, Detail: fmt.Sprintf("file ID appears at %d paths (hard link)", paths[node.FileID])})
		default:
			item.Evidence = append(item.Evidence, model.Evidence{Kind: KindFileIdentity, Status: model.EvidencePass, Detail: "single path for file ID"})
		}
		if allPass(item.Evidence) {
			item.Evidence = append(item.Evidence, prober.Probe(ctx, node)...)
		}
	default:
		item.Evidence = append(item.Evidence, model.Evidence{Kind: KindFileIdentity, Status: model.EvidenceFail, Detail: "链接或特殊文件，不处理"})
		return item
	}
	item.Executable = selectedExecutable(item)
	return item
}

func directoryExists(path string) model.Evidence {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return model.Evidence{Kind: KindExists, Status: model.EvidenceFail, Detail: "file no longer exists"}
	case err != nil:
		return model.Evidence{Kind: KindExists, Status: model.EvidenceError, Detail: err.Error()}
	case !info.IsDir() || info.Mode()&os.ModeSymlink != 0:
		return model.Evidence{Kind: KindExists, Status: model.EvidenceFail, Detail: "已不是普通目录"}
	}
	return model.Evidence{Kind: KindExists, Status: model.EvidencePass, Detail: "目录仍然存在"}
}

func selectedExecutable(item model.PlanItem) bool {
	required := []string{KindSource, KindProtection, KindAttributes, KindFileIdentity, probes.KindUnchanged, probes.KindUnlocked}
	if item.Kind == "directory" {
		required = []string{KindSource, KindProtection, KindAttributes, KindFileIdentity, KindExists}
	}
	if !allPass(item.Evidence) {
		return false
	}
	seen := map[string]bool{}
	for _, entry := range item.Evidence {
		seen[entry.Kind] = true
	}
	for _, kind := range required {
		if !seen[kind] {
			return false
		}
	}
	return true
}

// dropNested blocks items inside another selected directory: moving the parent already
// moves them, and moving both would fail halfway.
func dropNested(items []model.PlanItem) {
	for i := range items {
		for j := range items {
			if i != j && items[j].Kind == "directory" && strings.HasPrefix(strings.ToLower(items[i].Path), strings.ToLower(items[j].Path)+`\`) {
				items[i].Executable = false
				items[i].Evidence = append(items[i].Evidence, model.Evidence{Kind: KindSource, Status: model.EvidenceFail, Detail: "已包含在另一个选中的目录里：" + items[j].Path})
			}
		}
	}
}

func sumUnder(nodes []model.Node, dir string) (model.ByteCount, model.ByteCount) {
	prefix := strings.ToLower(dir) + `\`
	var logical, allocation model.ByteCount
	for _, node := range nodes {
		if node.Kind == "file" && strings.HasPrefix(strings.ToLower(node.Path), prefix) {
			logical += node.LogicalSize
			allocation += node.AllocationSize
		}
	}
	return logical, allocation
}
