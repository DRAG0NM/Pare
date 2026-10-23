//go:build windows

package executor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/plan"
	"github.com/DRAG0NM/Pare/internal/probes"
	"github.com/DRAG0NM/Pare/internal/rules"
	"github.com/DRAG0NM/Pare/internal/scanner"
)

// buildPlan writes old files under the test's temporary directory, which lies inside the
// user Temp directory that the built-in user-temp rule covers, and plans them for real.
func buildPlan(t *testing.T, names ...string) (string, model.Plan) {
	t.Helper()
	root := t.TempDir()
	old := time.Now().Add(-30 * 24 * time.Hour)
	for _, name := range names {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("content of "+name), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	scan, err := scanner.NewWindowsScanner().Scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := plan.Build(context.Background(), scan, rules.Builtin(), probes.NewWindowsProber(), time.Now(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if result.Totals.Executable != len(names) {
		t.Fatalf("executable = %d, want %d; is the test temp dir outside AppData\\Local\\Temp? %#v", result.Totals.Executable, len(names), result.Items)
	}
	return root, result
}

func states(execution model.Execution) map[string]model.ExecutionState {
	result := map[string]model.ExecutionState{}
	for _, item := range execution.Items {
		result[filepath.Base(item.OriginalPath)] = item.State
	}
	return result
}

func TestApplyAndUndo(t *testing.T) {
	root, reviewed := buildPlan(t, "a.tmp", "b.tmp", "changed.tmp", "locked.tmp")
	quarantineRoot := filepath.Join(t.TempDir(), "quarantine")

	if _, _, err := Apply(context.Background(), reviewed, "sha256:wrong", quarantineRoot, time.Now()); !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("Apply(wrong digest) = %v, want ErrUnconfirmed", err)
	}
	if _, err := os.Stat(quarantineRoot); !os.IsNotExist(err) {
		t.Fatalf("unconfirmed apply created %s", quarantineRoot)
	}

	if err := os.WriteFile(filepath.Join(root, "changed.tmp"), []byte("edited after review"), 0o600); err != nil {
		t.Fatal(err)
	}
	pointer, _ := windows.UTF16PtrFromString(filepath.Join(root, "locked.tmp"))
	handle, err := windows.CreateFile(pointer, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	execution, manifest, err := Apply(context.Background(), reviewed, reviewed.Digest, quarantineRoot, time.Now())
	windows.CloseHandle(handle)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]model.ExecutionState{
		"a.tmp": model.StateQuarantined, "b.tmp": model.StateQuarantined,
		"changed.tmp": model.StateSkipped, "locked.tmp": model.StateSkipped,
	}
	for name, state := range want {
		if states(execution)[name] != state {
			t.Errorf("%s state = %s, want %s (%#v)", name, states(execution)[name], state, execution.Items)
		}
	}
	for _, item := range execution.Items {
		_, original := os.Stat(item.OriginalPath)
		_, quarantined := os.Stat(item.QuarantinePath)
		if moved := item.State == model.StateQuarantined; (original == nil) == moved || (quarantined == nil) != moved {
			t.Errorf("%s: original exists=%v quarantined exists=%v for state %s", item.OriginalPath, original == nil, quarantined == nil, item.State)
		}
	}
	var saved model.Execution
	data, _ := os.ReadFile(manifest)
	if err := json.Unmarshal(data, &saved); err != nil || saved.Count(model.StateQuarantined) != 2 {
		t.Fatalf("manifest = %s, %v", data, err)
	}

	// An occupied original path must never be overwritten.
	occupant := filepath.Join(root, "b.tmp")
	if err := os.WriteFile(occupant, []byte("new file"), 0o600); err != nil {
		t.Fatal(err)
	}
	restored, err := Undo(context.Background(), manifest, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got := states(restored); got["a.tmp"] != model.StateRestored || got["b.tmp"] != model.StateConflict {
		t.Fatalf("undo states = %v", got)
	}
	if data, _ := os.ReadFile(occupant); string(data) != "new file" {
		t.Fatalf("occupant overwritten: %q", data)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "a.tmp")); string(data) != "content of a.tmp" {
		t.Fatalf("restored content = %q", data)
	}

	if err := os.Remove(occupant); err != nil {
		t.Fatal(err)
	}
	restored, err = Undo(context.Background(), manifest, time.Now())
	if err != nil || states(restored)["b.tmp"] != model.StateRestored {
		t.Fatalf("second undo = %v, %v", states(restored), err)
	}
	if data, _ := os.ReadFile(occupant); string(data) != "content of b.tmp" {
		t.Fatalf("b.tmp content = %q", data)
	}
}

func TestApplyRejectsInvalidPlans(t *testing.T) {
	_, reviewed := buildPlan(t, "a.tmp")
	quarantineRoot := filepath.Join(t.TempDir(), "quarantine")

	tampered := reviewed
	tampered.Items = append([]model.PlanItem(nil), reviewed.Items...)
	tampered.Items[0].Path = `C:\Windows\System32\kernel32.dll`
	if _, _, err := Apply(context.Background(), tampered, reviewed.Digest, quarantineRoot, time.Now()); !errors.Is(err, plan.ErrTampered) {
		t.Errorf("Apply(tampered) = %v, want ErrTampered", err)
	}
	if _, _, err := Apply(context.Background(), reviewed, reviewed.Digest, quarantineRoot, time.Now().Add(2*time.Hour)); !errors.Is(err, plan.ErrExpired) {
		t.Errorf("Apply(expired) = %v, want ErrExpired", err)
	}
	if _, err := os.Stat(quarantineRoot); !os.IsNotExist(err) {
		t.Fatalf("rejected plan created %s", quarantineRoot)
	}
}

func TestUndoTreatsUnmovedIntentAsSkipped(t *testing.T) {
	directory := t.TempDir()
	original := filepath.Join(directory, "still-here.tmp")
	if err := os.WriteFile(original, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(directory, ManifestName)
	crashed := model.Execution{Schema: model.ExecutionSchema, Items: []model.ExecutionItem{{
		OriginalPath: original, QuarantinePath: filepath.Join(directory, "files", "000000-still-here.tmp"), State: model.StateIntent,
	}}}
	if err := writeManifest(manifest, &crashed); err != nil {
		t.Fatal(err)
	}
	result, err := Undo(context.Background(), manifest, time.Now())
	if err != nil || result.Items[0].State != model.StateSkipped {
		t.Fatalf("undo of unmoved intent = %#v, %v", result.Items, err)
	}
	if _, err := os.Stat(original); err != nil {
		t.Fatalf("original file disturbed: %v", err)
	}
}
