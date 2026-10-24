//go:build windows

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DRAG0NM/Pare/internal/advisor"
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

	preview, err := app.PrepareExecution([]ChosenAction{{"f1", 0}, {"f1", 1}, {"f1", 2}, {"f1", 3}, {"missing", 0}}, nil)
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
