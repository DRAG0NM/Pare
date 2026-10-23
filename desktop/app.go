package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/scanner"
	"github.com/DRAG0NM/Pare/internal/tree"
)

// gapSampleLimit bounds how many coverage gaps are sent to the UI at once.
const gapSampleLimit = 200

type App struct {
	ctx context.Context

	mu     sync.Mutex
	cancel context.CancelFunc
	tree   *tree.Tree
	report model.ScanReport
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// NodeView is one row of the directory tree or one treemap cell.
type NodeView struct {
	Path           string `json:"path"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	LogicalSize    uint64 `json:"logicalSize"`
	AllocationSize uint64 `json:"allocationSize"`
	Files          int    `json:"files"`
	Directories    int    `json:"directories"`
	Incomplete     bool   `json:"incomplete"`
	HasChildren    bool   `json:"hasChildren"`
	// Others counts entries folded into this synthetic "others" item.
	Others int `json:"others"`
}

type NodeTree struct {
	NodeView
	Children []*NodeTree `json:"children"`
}

type ScanSummary struct {
	Root             string            `json:"root"`
	Complete         bool              `json:"complete"`
	Cancelled        bool              `json:"cancelled"`
	Error            string            `json:"error"`
	Seconds          float64           `json:"seconds"`
	Entries          int               `json:"entries"`
	Files            int               `json:"files"`
	Directories      int               `json:"directories"`
	LogicalSize      uint64            `json:"logicalSize"`
	AllocationSize   uint64            `json:"allocationSize"`
	UniqueAllocation uint64            `json:"uniqueAllocation"`
	GapCount         int               `json:"gapCount"`
	Gaps             []model.ScanError `json:"gaps"`
}

// ChooseFolder opens the system folder picker; it returns "" when the user cancels.
func (a *App) ChooseFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择要扫描的文件夹"})
}

// StartScan scans root in the background. Progress arrives as "scan:progress" events
// and the result as one "scan:done" event carrying a ScanSummary.
func (a *App) StartScan(root string) error {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	a.mu.Lock()
	if a.cancel != nil {
		a.mu.Unlock()
		return errors.New("a scan is already running")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	a.mu.Unlock()

	go func() {
		started := time.Now()
		backend := scanner.NewWindowsScannerWithOptions(scanner.Options{
			Progress: func(p scanner.Progress) { runtime.EventsEmit(a.ctx, "scan:progress", p) },
		})
		report, scanErr := backend.Scan(ctx, absolute)
		built := tree.Build(absolute, report)

		summary := ScanSummary{
			Root:             absolute,
			Complete:         report.Complete,
			Cancelled:        errors.Is(scanErr, context.Canceled),
			Seconds:          time.Since(started).Seconds(),
			Entries:          len(report.Nodes),
			Files:            built.Root.Files,
			Directories:      built.Root.Directories,
			LogicalSize:      built.Root.LogicalSize,
			AllocationSize:   built.Root.AllocationSize,
			UniqueAllocation: built.UniqueAllocation,
			GapCount:         len(report.Errors),
			Gaps:             report.Errors[:min(len(report.Errors), gapSampleLimit)],
		}
		if scanErr != nil && !summary.Cancelled {
			summary.Error = scanErr.Error()
		}

		a.mu.Lock()
		a.cancel = nil
		if !summary.Cancelled {
			a.tree, a.report = built, report
		}
		a.mu.Unlock()
		cancel()
		runtime.EventsEmit(a.ctx, "scan:done", summary)
	}()
	return nil
}

func (a *App) CancelScan() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
}

// Children lists a directory's entries, largest first. Past limit, the rest are folded
// into one "others" item so huge folders stay responsive.
func (a *App) Children(path string, limit int) ([]NodeView, error) {
	node, err := a.find(path)
	if err != nil {
		return nil, err
	}
	return views(node.Children, limit), nil
}

// Subtree returns path with its descendants down to depth levels, each level trimmed
// like Children. The treemap draws from it.
func (a *App) Subtree(path string, depth, limit int) (*NodeTree, error) {
	node, err := a.find(path)
	if err != nil {
		return nil, err
	}
	return subtree(node, depth, limit), nil
}

// Reveal opens Explorer with path selected.
func (a *App) Reveal(path string) error {
	if _, err := a.find(path); err != nil {
		return err
	}
	return exec.Command("explorer.exe", "/select,", path).Start()
}

func (a *App) find(path string) (*tree.Node, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.tree == nil {
		return nil, errors.New("no scan result yet")
	}
	node := a.tree.Find(path)
	if node == nil {
		return nil, fmt.Errorf("%s is not in the scan result", path)
	}
	return node, nil
}

func view(node *tree.Node) NodeView {
	return NodeView{
		Path:           node.Path,
		Name:           node.Name,
		Kind:           node.Kind,
		LogicalSize:    node.LogicalSize,
		AllocationSize: node.AllocationSize,
		Files:          node.Files,
		Directories:    node.Directories,
		Incomplete:     node.Incomplete,
		HasChildren:    len(node.Children) != 0,
	}
}

func views(children []*tree.Node, limit int) []NodeView {
	if limit <= 0 || limit > len(children) {
		limit = len(children)
	}
	result := make([]NodeView, 0, limit+1)
	for _, child := range children[:limit] {
		result = append(result, view(child))
	}
	if rest := children[limit:]; len(rest) != 0 {
		others := NodeView{Name: fmt.Sprintf("其他 %d 项", len(rest)), Kind: "others", Others: len(rest)}
		for _, child := range rest {
			others.LogicalSize += child.LogicalSize
			others.AllocationSize += child.AllocationSize
			others.Files += child.Files
			others.Incomplete = others.Incomplete || child.Incomplete
		}
		result = append(result, others)
	}
	return result
}

func subtree(node *tree.Node, depth, limit int) *NodeTree {
	result := &NodeTree{NodeView: view(node), Children: []*NodeTree{}}
	if depth <= 0 {
		return result
	}
	shown := node.Children
	if limit > 0 && len(shown) > limit {
		shown = shown[:limit]
	}
	for _, child := range shown {
		result.Children = append(result.Children, subtree(child, depth-1, limit))
	}
	for _, other := range views(node.Children, limit)[len(shown):] {
		result.Children = append(result.Children, &NodeTree{NodeView: other, Children: []*NodeTree{}})
	}
	return result
}
