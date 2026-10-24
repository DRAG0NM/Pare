//go:build windows

package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/plan"
	"github.com/DRAG0NM/Pare/internal/probes"
	"github.com/DRAG0NM/Pare/internal/scanner"
)

func scanTemp(t *testing.T, root string) model.ScanReport {
	t.Helper()
	report, err := scanner.NewWindowsScanner().Scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSelectedDirectoriesQuarantineUndoAndPurge(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "cache")
	busy := filepath.Join(root, "busy")
	single := filepath.Join(root, "old.zip")
	write(t, filepath.Join(cache, "a", "1.bin"), "one")
	write(t, filepath.Join(cache, "2.bin"), "two")
	write(t, filepath.Join(busy, "locked.db"), "in use")
	write(t, single, "zip")
	report := scanTemp(t, root)

	selected, err := plan.BuildSelected(context.Background(), report, []plan.Selection{
		{Path: cache, Source: "AI 建议：缓存"},
		{Path: filepath.Join(cache, "a"), Source: "AI 建议：重复"},
		{Path: busy, Source: "手动添加"},
		{Path: single, Source: "手动添加"},
		{Path: `C:\Windows\System32`, Source: "AI 建议：不该出现"},
	}, probes.NewWindowsProber(), time.Now(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	executable := map[string]bool{}
	for _, item := range selected.Items {
		executable[item.Path] = item.Executable
	}
	if !executable[cache] || !executable[busy] || !executable[single] || executable[filepath.Join(cache, "a")] || executable[`C:\Windows\System32`] {
		t.Fatalf("executable = %v; want nested and protected paths blocked", executable)
	}
	for _, item := range selected.Items {
		if item.Path == cache && (item.Kind != "directory" || item.LogicalSize != 6) {
			t.Fatalf("cache item = %+v", item)
		}
	}

	pointer, _ := windows.UTF16PtrFromString(filepath.Join(busy, "locked.db"))
	handle, err := windows.CreateFile(pointer, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	quarantineRoot := filepath.Join(t.TempDir(), "q")
	execution, manifest, err := Apply(context.Background(), selected, nil, selected.Digest, quarantineRoot, time.Now())
	windows.CloseHandle(handle)
	if err != nil {
		t.Fatal(err)
	}
	got := states(execution)
	if got["cache"] != model.StateQuarantined || got["old.zip"] != model.StateQuarantined || got["busy"] != model.StateSkipped {
		t.Fatalf("states = %v; want busy skipped because a file inside is open", got)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatal("cache directory still in place after quarantine")
	}
	if _, err := os.Stat(filepath.Join(busy, "locked.db")); err != nil {
		t.Fatalf("busy directory was disturbed: %v", err)
	}

	restored, err := Undo(context.Background(), manifest, time.Now())
	if err != nil || states(restored)["cache"] != model.StateRestored {
		t.Fatalf("undo = %v, %v", states(restored), err)
	}
	if data, _ := os.ReadFile(filepath.Join(cache, "a", "1.bin")); string(data) != "one" {
		t.Fatalf("restored content = %q", data)
	}

	// Quarantine again, then purge the batch for real.
	again, err := plan.BuildSelected(context.Background(), scanTemp(t, root), []plan.Selection{{Path: cache, Source: "AI 建议：缓存"}}, probes.NewWindowsProber(), time.Now(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	_, manifest, err = Apply(context.Background(), again, nil, again.Digest, quarantineRoot, time.Now().Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	purged, err := Purge(context.Background(), manifest, time.Now())
	if err != nil || states(purged)["cache"] != model.StatePurged {
		t.Fatalf("purge = %v, %v", states(purged), err)
	}
	if _, err := os.Stat(purged.Items[0].QuarantinePath); !os.IsNotExist(err) {
		t.Fatal("purged directory still exists in quarantine")
	}
	if _, err := os.Stat(manifest); err != nil {
		t.Fatal("manifest should stay as a record after purging")
	}

	batches := ListBatches([]string{quarantineRoot})
	if len(batches) != 2 || batches[0].Purged != 1 || batches[1].Restored != 2 || batches[1].Other != 1 {
		t.Fatalf("batches = %+v", batches)
	}
}

func TestPurgeLeavesPathsOutsideTheBatch(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "keep.txt")
	write(t, outside, "keep")
	batch := filepath.Join(t.TempDir(), "batch")
	manifest := filepath.Join(batch, ManifestName)
	os.MkdirAll(filepath.Join(batch, "files"), 0o700)
	tampered := model.Execution{Schema: model.ExecutionSchema, Items: []model.ExecutionItem{{OriginalPath: outside, QuarantinePath: outside, State: model.StateQuarantined}}}
	if err := writeManifest(manifest, &tampered); err != nil {
		t.Fatal(err)
	}
	result, err := Purge(context.Background(), manifest, time.Now())
	if err != nil || result.Items[0].State != model.StateQuarantined || !strings.Contains(result.Items[0].Detail, "not inside") {
		t.Fatalf("purge of outside path = %+v, %v", result.Items, err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal("file outside the batch was deleted")
	}
}
