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
	Key    string `json:"key"`
	Label  string `json:"label"`
	Status string `json:"status"` // done, partial, skipped, failed
	Detail string `json:"detail"`
	Output string `json:"output"`
}

type ExecutionReport struct {
	Results    []ActionResult `json:"results"`
	Manifests  []string       `json:"manifests"`
	FreedBytes int64          `json:"freedBytes"`
	Error      string         `json:"error"`
}

// preparedRun keeps what PrepareExecution built, so Execute runs exactly what was shown.
type preparedRun struct {
	actions  []PreparedAction
	selected *model.Plan
	rules    *model.Plan
	keyPaths map[string][]string
	runs     map[string]actions.Prepared
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

	run := &preparedRun{keyPaths: map[string][]string{}, runs: map[string]actions.Prepared{}}
	var selections []plan.Selection
	for _, choice := range chosen {
		finding, ok := byID[choice.FindingID]
		if !ok || choice.ActionIndex < 0 || choice.ActionIndex >= len(finding.Actions) {
			continue
		}
		action := finding.Actions[choice.ActionIndex]
		view := PreparedAction{
			Key:          choice.FindingID + ":" + strconv.Itoa(choice.ActionIndex),
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
			for _, path := range run.keyPaths[view.Key] {
				item, ok := byPath[strings.ToLower(filepath.Clean(path))]
				if !ok {
					continue
				}
				view.Items = append(view.Items, itemView(item))
				view.Runnable = view.Runnable || item.Executable
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
	run, root := a.prepared, a.root
	a.prepared = nil
	if a.executing {
		a.mu.Unlock()
		return errors.New("已有操作在执行")
	}
	if run == nil {
		a.mu.Unlock()
		return errors.New("没有待执行的方案，请重新确认")
	}
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
	report := ExecutionReport{Results: []ActionResult{}, Manifests: []string{}}
	volume := filepath.VolumeName(root) + `\`
	_, freeBefore, _ := volumeSpace(volume)
	progress := func(result ActionResult) {
		runtime.EventsEmit(a.ctx, "exec:progress", result)
	}

	quarantine := func(p *model.Plan, keys []string, paths []string) {
		label := map[string]string{}
		for _, action := range run.actions {
			label[action.Key] = action.Label
		}
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
				result := ActionResult{Key: key, Label: label[key], Status: "skipped", Detail: "没有可以执行的项目"}
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
			result := ActionResult{Key: key, Label: label[key]}
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
		progress(ActionResult{Key: action.Key, Label: action.Label, Status: "running"})
		output, err := actions.Run(context.Background(), prepared)
		result := ActionResult{Key: action.Key, Label: action.Label, Status: "done", Output: output}
		if err != nil {
			result.Status, result.Detail = "failed", err.Error()
		}
		report.Results = append(report.Results, result)
		progress(result)
	}

	_, freeAfter, _ := volumeSpace(volume)
	report.FreedBytes = int64(freeAfter) - int64(freeBefore)
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
	return executor.Undo(context.Background(), manifest, time.Now())
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
	return PurgeResult{Execution: execution, FreedBytes: int64(after) - int64(before)}, err
}
