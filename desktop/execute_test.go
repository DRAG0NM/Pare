//go:build windows

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DRAG0NM/Pare/internal/advisor"
	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/scanner"
)

func TestPrepareExecutionShowsWhatWouldRun(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "cache")
	os.MkdirAll(cache, 0o755)
	os.WriteFile(filepath.Join(cache, "a.bin"), []byte("abc"), 0o600)
	report, err := scanner.NewWindowsScanner().Scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.report, app.root = report, root
	app.findings = []advisor.Finding{{
		ID: "f1", Title: "缓存残留",
		Actions: []advisor.Action{
			{Kind: advisor.ActionQuarantine, Label: "移入隔离区", Paths: []string{cache, `C:\Windows\System32`}},
			{Kind: advisor.ActionTemplate, Label: "删驱动", Template: "driver_delete", Params: map[string]string{"inf": "oem27.inf"}},
			{Kind: advisor.ActionTemplate, Label: "坏参数", Template: "driver_delete", Params: map[string]string{"inf": "../evil.inf"}},
			{Kind: advisor.ActionManual, Label: "手动", Explain: "去软件设置里关掉自动更新"},
		},
	}}

	preview, err := app.PrepareExecution([]ChosenAction{{"f1", 0, false}, {"f1", 1, false}, {"f1", 2, false}, {"f1", 3, false}, {"missing", 0, false}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Actions) != 4 {
		t.Fatalf("actions = %+v", preview.Actions)
	}
	quarantine := preview.Actions[0]
	if !quarantine.Runnable || len(quarantine.Items) != 2 || !quarantine.Items[0].Executable || quarantine.Items[1].Executable || quarantine.Items[1].Reason == "" {
		t.Errorf("quarantine preview = %+v", quarantine)
	}
	if driver := preview.Actions[1]; !driver.Runnable || driver.Steps[0] != "pnputil.exe /delete-driver oem27.inf" || !driver.NeedsAdmin {
		t.Errorf("template preview = %+v", driver)
	}
	if bad := preview.Actions[2]; bad.Runnable || bad.Error == "" {
		t.Errorf("invalid template preview = %+v", bad)
	}
	if manual := preview.Actions[3]; manual.Runnable {
		t.Errorf("manual action marked runnable: %+v", manual)
	}
	if app.prepared == nil || app.prepared.selected == nil {
		t.Fatal("PrepareExecution did not keep the plan for Execute")
	}
}

func TestCheckManifestOnlyAcceptsQuarantineRecords(t *testing.T) {
	app := NewApp()
	for _, path := range []string{`C:\Windows\System32\config\SAM`, `C:\.pare-quarantine\..\Windows\manifest.json`, `C:\.pare-quarantine\b\other.json`} {
		if err := app.checkManifest(path); err == nil {
			t.Errorf("checkManifest(%q) accepted", path)
		}
	}
	if err := app.checkManifest(`C:\.pare-quarantine\20261024-1\manifest.json`); err != nil {
		t.Errorf("valid manifest rejected: %v", err)
	}
}

func TestPreventionActionsAndHistory(t *testing.T) {
	app := NewApp()
	app.historyFile = filepath.Join(t.TempDir(), "history.json")
	app.findings = []advisor.Finding{{
		ID: "f1", Title: "WSL 日志没有上限", Regrowth: "会再长",
		Prevention: []advisor.Action{{Kind: advisor.ActionTemplate, Label: "设上限", Template: "wsl_journal_limit", Params: map[string]string{"distro": "Ubuntu", "size": "200M"}}},
	}}
	preview, err := app.PrepareExecution([]ChosenAction{{FindingID: "f1", ActionIndex: 0, Prevent: true}, {FindingID: "f1", ActionIndex: 3, Prevent: true}}, nil)
	if err != nil || len(preview.Actions) != 1 || preview.Actions[0].Key != "f1:p0" || !preview.Actions[0].Runnable {
		t.Fatalf("prevention preview = %+v, %v", preview.Actions, err)
	}

	app.recordCleanup(ExecutionReport{Volume: `C:\`, FreedBytes: 10, QuarantinedBytes: 300, Results: []ActionResult{{Label: "移入隔离区", Status: "done", Bytes: 300}, {Label: "删驱动", Status: "failed"}}})
	app.recordCleanup(ExecutionReport{})
	history := app.ListHistory()
	if len(history) != 1 || history[0].Items != 1 || history[0].QuarantinedBytes != 300 || len(history[0].Actions) != 2 {
		t.Fatalf("history = %+v", history)
	}
}

// A folder whose contents may go is expanded into its entries; the dialog must list
// them (it used to show an empty, unclickable card), and an empty one must say why.
func TestPrepareExecutionFollowsExpandedFolders(t *testing.T) {
	download := `Z:\Windows\SoftwareDistribution\Download`
	app := NewApp()
	app.report = model.ScanReport{Complete: true, Nodes: []model.Node{
		{Path: download, ParentPath: `Z:\Windows\SoftwareDistribution`, Kind: "directory", FileID: "1"},
		{Path: download + `\a.cab`, ParentPath: download, Kind: "file", FileID: "2", AllocationSize: 100},
		{Path: download + `\b.cab`, ParentPath: download, Kind: "file", FileID: "3", AllocationSize: 200},
	}}
	app.findings = []advisor.Finding{
		{ID: "f1", Title: "更新缓存", Actions: []advisor.Action{{Kind: advisor.ActionQuarantine, Label: "移入隔离区", Paths: []string{download}}}},
		{ID: "f2", Title: "空目录", Actions: []advisor.Action{{Kind: advisor.ActionQuarantine, Label: "移入隔离区", Paths: []string{`Z:\Windows\Temp`}}}},
	}
	preview, err := app.PrepareExecution([]ChosenAction{{"f1", 0, false}, {"f2", 0, false}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := preview.Actions[0]; len(got.Items) != 2 || got.Items[0].Path != download+`\a.cab` || got.Error != "" {
		t.Errorf("expanded folder preview = %+v", got)
	}
	if paths := app.prepared.keyPaths["f1:0"]; len(paths) != 2 {
		t.Errorf("Execute would look for %v", paths)
	}
	if got := preview.Actions[1]; got.Runnable || got.Error == "" {
		t.Errorf("empty folder preview has no reason: %+v", got)
	}
}
