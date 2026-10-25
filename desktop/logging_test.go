package main

import (
	"os"
	"strings"
	"testing"
)

// TestMain keeps settings, history and logs written by tests out of the real %APPDATA%.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "pare-test-appdata")
	if err != nil {
		panic(err)
	}
	os.Setenv("APPDATA", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestLogExportCollectsLogAndHistory(t *testing.T) {
	app := NewApp()
	app.logFolder = t.TempDir()
	app.historyFile = t.TempDir() + `\history.json`
	app.logf("执行 f1:0：缓存 / 移入隔离区 → %s", "done")
	app.recordCleanup(ExecutionReport{Volume: `C:\`, Results: []ActionResult{{FindingTitle: "缓存", Label: "移入隔离区", Status: "done"}}})
	text := app.buildLogExport()
	for _, want := range []string{"Pare " + appVersion, "执行 f1:0：缓存 / 移入隔离区 → done", "[done] 缓存 / 移入隔离区", "已保存 Key：false"} {
		if !strings.Contains(text, want) {
			t.Errorf("export missing %q:\n%s", want, text)
		}
	}
}
