//go:build windows

package probes

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/scanner"
)

func TestWindowsProberDetectsChangesAndLocks(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "cache.bin")
	if err := os.WriteFile(path, []byte("cached"), 0o600); err != nil {
		t.Fatal(err)
	}
	scan, err := scanner.NewWindowsScanner().Scan(context.Background(), root)
	if err != nil || len(scan.Nodes) != 1 {
		t.Fatalf("scan = %#v, %v", scan, err)
	}
	node := scan.Nodes[0]
	prober := NewWindowsProber()

	assertStatuses(t, "untouched", prober.Probe(context.Background(), node), model.EvidencePass, model.EvidencePass)

	changed := node
	changed.LogicalSize++
	assertStatuses(t, "size changed", prober.Probe(context.Background(), changed), model.EvidenceFail, model.EvidencePass)

	pointer, _ := windows.UTF16PtrFromString(path)
	handle, err := windows.CreateFile(pointer, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	assertStatuses(t, "open elsewhere", prober.Probe(context.Background(), node), model.EvidenceUnknown, model.EvidenceFail)
	windows.CloseHandle(handle)

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	assertStatuses(t, "removed", prober.Probe(context.Background(), node), model.EvidenceFail, model.EvidenceUnknown)
}

func assertStatuses(t *testing.T, name string, evidence []model.Evidence, unchanged, unlocked model.EvidenceStatus) {
	t.Helper()
	if len(evidence) != 2 || evidence[0].Kind != KindUnchanged || evidence[1].Kind != KindUnlocked {
		t.Fatalf("%s: evidence = %#v", name, evidence)
	}
	if evidence[0].Status != unchanged || evidence[1].Status != unlocked {
		t.Errorf("%s: unchanged=%s unlocked=%s, want %s %s (%#v)", name, evidence[0].Status, evidence[1].Status, unchanged, unlocked, evidence)
	}
}
