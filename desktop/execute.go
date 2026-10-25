package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/DRAG0NM/Pare/internal/actions"
	"github.com/DRAG0NM/Pare/internal/advisor"
	"github.com/DRAG0NM/Pare/internal/executor"
	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/plan"
	"github.com/DRAG0NM/Pare/internal/probes"
)

// ChosenAction is one action the user picked for a finding.
type ChosenAction struct {
	FindingID   string `json:"findingId"`
	ActionIndex int    `json:"actionIndex"`
	// Prevent picks from the finding's prevention actions instead of its actions.
	Prevent bool `json:"prevent"`
}

type QuarantineItemView struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Bytes      uint64 `json:"bytes"`
	Executable bool   `json:"executable"`
	Reason     string `json:"reason"`
}

// PreparedAction is what the confirmation dialog shows for one chosen action.
type PreparedAction struct {
	Key          string               `json:"key"`
	FindingTitle string               `json:"findingTitle"`
	Kind         string               `json:"kind"`
	Label        string               `json:"label"`
	Explain      string               `json:"explain"`
	Bytes        uint64               `json:"bytes"`
	Items        []QuarantineItemView `json:"items"`
	Steps        []string             `json:"steps"`
	Warnings     []string             `json:"warnings"`
	NeedsAdmin   bool                 `json:"needsAdmin"`
	// Runnable is false for manual steps, config changes and invalid templates.
	Runnable bool   `json:"runnable"`
	Error    string `json:"error"`
}

type ExecutionPreview struct {
	Actions []PreparedAction `json:"actions"`
}

type ActionResult struct {
	Key          string `json:"key"`
	FindingTitle string `json:"findingTitle"`
	Kind         string `json:"kind"`
	Label        string `json:"label"`
	Status       string `json:"status"` // done, partial, skipped, failed
	Detail       string `json:"detail"`
	Output       string `json:"output"`
	// Bytes is what was moved into quarantine, or the estimate for a template or
	// command that finished.
	Bytes uint64 `json:"bytes"`
}

// RegrowthTip repeats what a handled finding said about growing back, so the summary can
// offer its prevention actions.
type RegrowthTip struct {
	FindingID    string `json:"findingId"`
	FindingTitle string `json:"findingTitle"`
	Text         string `json:"text"`
}

type ExecutionReport struct {
	Time             time.Time      `json:"time"`
	Volume           string         `json:"volume"`
	FreeBefore       uint64         `json:"freeBefore"`
	FreeAfter        uint64         `json:"freeAfter"`
	Results          []ActionResult `json:"results"`
	Manifests        []string       `json:"manifests"`
	FreedBytes       int64          `json:"freedBytes"`
	QuarantinedBytes uint64         `json:"quarantinedBytes"`
	Tips             []RegrowthTip  `json:"tips"`
	Error            string         `json:"error"`
}

// preparedRun keeps what PrepareExecution built, so Execute runs exactly what was shown.
type preparedRun struct {
	actions  []PreparedAction
	selected *model.Plan
	rules    *model.Plan
	keyPaths map[string][]string
	runs     map[string]actions.Prepared
	findings map[string]advisor.Finding // by action key
}

const rulesKey = "rules"

// PrepareExecution validates the chosen finding actions and the rule candidates the user
// ticked, and returns everything the confirmation dialog needs. Nothing runs here.
func (a *App) PrepareExecution(chosen []ChosenAction, rulePaths []string) (ExecutionPreview, error) {
	a.mu.Lock()
	report, findings, rulePlan := a.report, a.findings, a.plan
	a.mu.Unlock()
	byID := map[string]advisor.Finding{}
	for _, finding := range findings {
		byID[finding.ID] = finding
	}

	run := &preparedRun{keyPaths: map[string][]string{}, runs: map[string]actions.Prepared{}, findings: map[string]advisor.Finding{}}
	var selections []plan.Selection
	for _, choice := range chosen {
		finding, ok := byID[choice.FindingID]
		list, prefix := finding.Actions, ":"
		if choice.Prevent {
			list, prefix = finding.Prevention, ":p"
		}
		if !ok || choice.ActionIndex < 0 || choice.ActionIndex >= len(list) {
			continue
		}
		action := list[choice.ActionIndex]
		if choice.Prevent && action.Kind == advisor.ActionQuarantine {
			continue
		}
		view := PreparedAction{
			Key:          choice.FindingID + prefix + strconv.Itoa(choice.ActionIndex),
			FindingTitle: finding.Title,
			Kind:         action.Kind,
			Label:        action.Label,
			Explain:      action.Explain,
			Bytes:        action.Bytes,
			Items:        []QuarantineItemView{},
			Steps:        []string{},
			Warnings:     []string{},
		}
		switch action.Kind {
		case advisor.ActionQuarantine:
			for _, path := range action.Paths {
				selections = append(selections, plan.Selection{Path: path, Source: "AI 建议：" + finding.Title})
			}
			run.keyPaths[view.Key] = action.Paths
		case advisor.ActionTemplate, advisor.ActionCommand:
			var prepared actions.Prepared
			var err error
			if action.Kind == advisor.ActionTemplate {
				prepared, err = actions.Prepare(action.Template, action.Params)
			} else {
				prepared, err = actions.PrepareCommand(action.Command)
			}
			if err != nil {
				view.Error = err.Error()
			} else {
				view.Steps, view.Warnings, view.NeedsAdmin, view.Runnable = prepared.Display(), prepared.Warnings, prepared.NeedsAdmin, true
				run.runs[view.Key] = prepared
			}
		default:
			view.Steps = action.Paths
		}
		run.findings[view.Key] = finding
		run.actions = append(run.actions, view)
	}

	if len(selections) != 0 {
		built, err := plan.BuildSelected(context.Background(), report, selections, probes.NewWindowsProber(), time.Now(), planTTL)
		if err != nil {
			return ExecutionPreview{}, err
		}
		run.selected = &built
		byPath := map[string]model.PlanItem{}
		for _, item := range built.Items {
			byPath[strings.ToLower(filepath.Clean(item.Path))] = item
		}
		for i := range run.actions {
			view := &run.actions[i]
			paths, ok := run.keyPaths[view.Key]
			if !ok {
				continue
			}
			// A folder whose contents may go (Windows\SoftwareDistribution\Download and
			// the like) was expanded into the entries inside it; follow those, so the
			// dialog and Execute both work with the real items.
			var resolved []string
			for _, path := range paths {
				clean := strings.ToLower(filepath.Clean(path))
				if item, found := byPath[clean]; found {
					resolved = append(resolved, item.Path)
					continue
				}
				if plan.ContentsOnly(path) {
					for _, item := range built.Items {
						if strings.ToLower(filepath.Dir(item.Path)) == clean {
							resolved = append(resolved, item.Path)
						}
					}
				}
			}
			run.keyPaths[view.Key] = resolved
			for _, path := range resolved {
				item := byPath[strings.ToLower(filepath.Clean(path))]
				view.Items = append(view.Items, itemView(item))
				view.Runnable = view.Runnable || item.Executable
			}
			if len(view.Items) == 0 {
				view.Error = "扫描结果里没有可以处理的内容，可能已经清理过或扫描后有变化，请重新扫描后再试"
			}
		}
	}

	if len(rulePaths) != 0 && rulePlan != nil {
		view := PreparedAction{Key: rulesKey, FindingTitle: "常规缓存（规则）", Kind: advisor.ActionQuarantine, Label: "移入隔离区", Items: []QuarantineItemView{}, Steps: []string{}, Warnings: []string{}}
		wanted := map[string]bool{}
		for _, path := range rulePaths {
			wanted[path] = true
		}
		for _, item := range rulePlan.Items {
			if wanted[item.Path] {
				view.Items = append(view.Items, itemView(item))
				view.Bytes += uint64(item.AllocationSize)
				view.Runnable = view.Runnable || item.Executable
			}
		}
		run.rules = rulePlan
		run.actions = append(run.actions, view)
	}

	a.mu.Lock()
	a.prepared = run
	a.mu.Unlock()
	for _, action := range run.actions {
		a.logf("待确认 %s：%s / %s（%s），%d 项，可执行 %v，错误 %q", action.Key, action.FindingTitle, action.Label, action.Kind, len(action.Items), action.Runnable, action.Error)
		for _, item := range action.Items {
			if !item.Executable {
				a.logf("    不可执行 %s：%s", item.Path, item.Reason)
			}
		}
	}
	return ExecutionPreview{Actions: run.actions}, nil
}

func itemView(item model.PlanItem) QuarantineItemView {
	view := QuarantineItemView{Path: item.Path, Kind: item.Kind, Bytes: uint64(item.AllocationSize), Executable: item.Executable}
	if view.Kind == "" {
		view.Kind = "file"
	}
	for _, evidence := range item.Evidence {
		if evidence.Status != model.EvidencePass {
			view.Reason = evidence.Detail
			break
		}
	}
	return view
}

// Execute runs the accepted actions of the last preview, in order: quarantines first
// (reversible), then templates and commands. Progress arrives as "exec:progress" events
// and the report as one "exec:done" event.
func (a *App) Execute(accepted []string) error {
	a.mu.Lock()
	if a.executing {
		a.mu.Unlock()
		return errors.New("已有操作在执行")
	}
	run, root := a.prepared, a.root
	if run == nil {
		a.mu.Unlock()
		return errors.New("没有待执行的方案，请重新确认")
	}
	// A preview runs at most once; a new run needs a fresh confirmation.
	a.prepared = nil
	a.executing = true
	a.mu.Unlock()

	ok := map[string]bool{}
	for _, key := range accepted {
		ok[key] = true
	}
	go func() {
		report := a.execute(run, ok, root)
		a.mu.Lock()
		a.executing = false
		a.mu.Unlock()
		runtime.EventsEmit(a.ctx, "exec:done", report)
	}()
	return nil
}

func (a *App) execute(run *preparedRun, accepted map[string]bool, root string) ExecutionReport {
	volume := filepath.VolumeName(root) + `\`
	_, freeBefore, _ := volumeSpace(volume)
	report := ExecutionReport{Time: time.Now(), Volume: volume, FreeBefore: freeBefore, Results: []ActionResult{}, Manifests: []string{}, Tips: []RegrowthTip{}}
	byKey := map[string]PreparedAction{}
	for _, action := range run.actions {
		byKey[action.Key] = action
	}
	// newResult fills in what the summary shows about the action behind key.
	newResult := func(key string) ActionResult {
		action := byKey[key]
		return ActionResult{Key: key, FindingTitle: action.FindingTitle, Kind: action.Kind, Label: action.Label}
	}
	progress := func(result ActionResult) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "exec:progress", result)
		}
	}

	quarantine := func(p *model.Plan, keys []string, paths []string) {
		executable := map[string]bool{}
		for _, item := range p.Items {
			if item.Executable {
				executable[strings.ToLower(item.Path)] = true
			}
		}
		var selected []string
		seen := map[string]bool{}
		for _, path := range paths {
			for _, item := range p.Items {
				if strings.EqualFold(filepath.Clean(item.Path), filepath.Clean(path)) && executable[strings.ToLower(item.Path)] && !seen[strings.ToLower(item.Path)] {
					seen[strings.ToLower(item.Path)] = true
					selected = append(selected, item.Path)
				}
			}
		}
		if len(selected) == 0 {
			for _, key := range keys {
				result := newResult(key)
				result.Status, result.Detail = "skipped", "没有可以执行的项目"
				report.Results = append(report.Results, result)
				progress(result)
			}
			return
		}
		execution, manifest, err := executor.Apply(context.Background(), *p, selected, p.Digest, executor.DefaultQuarantineRoot(selected[0]), time.Now())
		if manifest != "" {
			report.Manifests = append(report.Manifests, manifest)
		}
		states := map[string]model.ExecutionItem{}
		for _, item := range execution.Items {
			states[strings.ToLower(item.OriginalPath)] = item
		}
		for _, key := range keys {
			result := newResult(key)
			moved, total := 0, 0
			var problems []string
			for _, path := range run.keyPaths[key] {
				item, found := states[strings.ToLower(filepath.Clean(path))]
				if !found {
					continue
				}
				total++
				if item.State == model.StateQuarantined {
					moved++
					result.Bytes += uint64(item.AllocationSize)
				} else {
					problems = append(problems, filepath.Base(item.OriginalPath)+"："+item.Detail)
				}
			}
			switch {
			case err != nil:
				result.Status, result.Detail = "failed", err.Error()
			case total > 0 && moved == total:
				result.Status, result.Detail = "done", fmt.Sprintf("已移入隔离区 %d 项", moved)
			case moved > 0:
				result.Status, result.Detail = "partial", fmt.Sprintf("移入 %d 项，%d 项未处理：%s", moved, total-moved, strings.Join(problems, "；"))
			default:
				result.Status, result.Detail = "skipped", strings.Join(problems, "；")
			}
			report.QuarantinedBytes += result.Bytes
			report.Results = append(report.Results, result)
			progress(result)
		}
	}

	if run.selected != nil {
		var keys, paths []string
		for _, action := range run.actions {
			if action.Kind == advisor.ActionQuarantine && action.Key != rulesKey && accepted[action.Key] {
				keys = append(keys, action.Key)
				paths = append(paths, run.keyPaths[action.Key]...)
			}
		}
		if len(keys) != 0 {
			quarantine(run.selected, keys, paths)
		}
	}
	if run.rules != nil && accepted[rulesKey] {
		var paths []string
		for _, action := range run.actions {
			if action.Key == rulesKey {
				for _, item := range action.Items {
					paths = append(paths, item.Path)
				}
			}
		}
		run.keyPaths[rulesKey] = paths
		quarantine(run.rules, []string{rulesKey}, paths)
	}
	for _, action := range run.actions {
		prepared, ok := run.runs[action.Key]
		if !ok || !accepted[action.Key] {
			continue
		}
		running := newResult(action.Key)
		running.Status = "running"
		progress(running)
		output, err := actions.Run(context.Background(), prepared)
		result := newResult(action.Key)
		result.Status, result.Output, result.Bytes = "done", output, action.Bytes
		if err != nil {
			result.Status, result.Detail, result.Bytes = "failed", err.Error(), 0
		}
		report.Results = append(report.Results, result)
		progress(result)
	}

	_, freeAfter, _ := volumeSpace(volume)
	report.FreeAfter = freeAfter
	report.FreedBytes = int64(freeAfter) - int64(freeBefore)
	seen := map[string]bool{}
	for _, result := range report.Results {
		finding, ok := run.findings[result.Key]
		if !ok || seen[finding.ID] || strings.Contains(result.Key, ":p") || (result.Status != "done" && result.Status != "partial") || strings.TrimSpace(finding.Regrowth) == "" {
			continue
		}
		seen[finding.ID] = true
		report.Tips = append(report.Tips, RegrowthTip{FindingID: finding.ID, FindingTitle: finding.Title, Text: finding.Regrowth})
	}
	for _, result := range report.Results {
		a.logf("执行 %s：%s / %s（%s）→ %s，%d 字节，%s", result.Key, result.FindingTitle, result.Label, result.Kind, result.Status, result.Bytes, result.Detail)
		if result.Output != "" {
			a.logf("    输出：%s", clipLog(result.Output))
		}
	}
	a.logf("执行结束：%s 可用 %d → %d 字节，隔离 %d 字节", report.Volume, report.FreeBefore, report.FreeAfter, report.QuarantinedBytes)
	a.recordCleanup(report)
	return report
}

// quarantineRoots lists Pare quarantine folders on local drives.
func (a *App) quarantineRoots() []string {
	var roots []string
	for _, drive := range a.Drives() {
		roots = append(roots, executor.DefaultQuarantineRoot(drive.Root))
	}
	return roots
}

func (a *App) ListQuarantine() []executor.BatchSummary {
	batches := executor.ListBatches(a.quarantineRoots())
	if batches == nil {
		return []executor.BatchSummary{}
	}
	return batches
}

// checkManifest only accepts manifests inside a Pare quarantine root.
func (a *App) checkManifest(manifest string) error {
	clean := strings.ToLower(filepath.Clean(manifest))
	for _, root := range a.quarantineRoots() {
		if strings.HasPrefix(clean, strings.ToLower(root)+`\`) && filepath.Base(clean) == executor.ManifestName {
			return nil
		}
	}
	return errors.New("不是 Pare 隔离区里的记录")
}

func (a *App) UndoBatch(manifest string) (model.Execution, error) {
	if err := a.checkManifest(manifest); err != nil {
		return model.Execution{}, err
	}
	execution, err := executor.Undo(context.Background(), manifest, time.Now())
	a.logf("撤回 %s：%d 项，错误 %v", manifest, len(execution.Items), err)
	return execution, err
}

type PurgeResult struct {
	Execution  model.Execution `json:"execution"`
	FreedBytes int64           `json:"freedBytes"`
}

// PurgeBatch permanently deletes a batch's quarantined items and reports the space freed.
func (a *App) PurgeBatch(manifest string) (PurgeResult, error) {
	if err := a.checkManifest(manifest); err != nil {
		return PurgeResult{}, err
	}
	volume := filepath.VolumeName(manifest) + `\`
	_, before, _ := volumeSpace(volume)
	execution, err := executor.Purge(context.Background(), manifest, time.Now())
	_, after, _ := volumeSpace(volume)
	result := PurgeResult{Execution: execution, FreedBytes: int64(after) - int64(before)}
	a.logf("清空 %s：释放 %d 字节，错误 %v", manifest, result.FreedBytes, err)
	a.recordPurge(volume, execution, result.FreedBytes)
	return result, err
}
