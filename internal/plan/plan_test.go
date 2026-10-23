package plan

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/probes"
	"github.com/DRAG0NM/Pare/internal/rules"
)

var now = time.Date(2026, time.October, 23, 12, 0, 0, 0, time.UTC)

const tempDir = `C:\Users\me\AppData\Local\Temp\`

func file(path, id string, age time.Duration) model.Node {
	return model.Node{Path: path, Kind: "file", FileID: id, LogicalSize: 100, AllocationSize: 4096, LastWriteTime: now.Add(-age)}
}

type fakeProber struct {
	status model.EvidenceStatus
	calls  int
}

func (p *fakeProber) Probe(context.Context, model.Node) []model.Evidence {
	p.calls++
	return []model.Evidence{
		{Kind: probes.KindUnchanged, Status: model.EvidencePass},
		{Kind: probes.KindUnlocked, Status: p.status},
	}
}

func build(t *testing.T, nodes []model.Node, prober probes.Prober) model.Plan {
	t.Helper()
	scan := model.ScanReport{Schema: model.ScanSchema, Volume: "C::1a2b3c4d", Complete: true, Nodes: nodes}
	result, err := Build(context.Background(), scan, rules.Builtin(), prober, now, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestBuildMarksOnlyFullyEvidencedItemsExecutable(t *testing.T) {
	system := file(tempDir+"system.tmp", "05", 30*24*time.Hour)
	system.Attributes = fileAttributeSystem
	nodes := []model.Node{
		file(tempDir+"old.tmp", "01", 30*24*time.Hour),
		file(tempDir+"new.tmp", "02", time.Hour),
		file(tempDir+"noid.tmp", "", 30*24*time.Hour),
		file(tempDir+"link-a.tmp", "04", 30*24*time.Hour),
		file(tempDir+"link-b.tmp", "04", 30*24*time.Hour),
		system,
		file(`C:\Users\me\Documents\keep.docx`, "06", 365*24*time.Hour),
		{Path: tempDir + "dir", Kind: "directory", FileID: "07"},
	}
	prober := &fakeProber{status: model.EvidencePass}

	result := build(t, nodes, prober)

	executable := map[string]bool{}
	for _, item := range result.Items {
		executable[item.Path] = item.Executable
	}
	if len(executable) != 6 {
		t.Fatalf("items = %d, want 6 rule-matched files: %#v", len(executable), result.Items)
	}
	for path, want := range map[string]bool{
		tempDir + "old.tmp": true, tempDir + "new.tmp": false, tempDir + "noid.tmp": false,
		tempDir + "link-a.tmp": false, tempDir + "link-b.tmp": false, tempDir + "system.tmp": false,
	} {
		if executable[path] != want {
			t.Errorf("%s executable = %v, want %v", path, executable[path], want)
		}
	}
	if prober.calls != 1 {
		t.Errorf("probe calls = %d, want 1 (only items whose static evidence passes)", prober.calls)
	}
	if result.Totals.Executable != 1 || result.Totals.ExecutableAllocation != 4096 {
		t.Errorf("totals = %#v", result.Totals)
	}
}

func TestBuildBlocksUnknownAndErrorProbes(t *testing.T) {
	for _, status := range []model.EvidenceStatus{model.EvidenceFail, model.EvidenceUnknown, model.EvidenceError} {
		result := build(t, []model.Node{file(tempDir+"old.tmp", "01", 30*24*time.Hour)}, &fakeProber{status: status})
		if result.Items[0].Executable {
			t.Errorf("unlocked=%s produced an executable item", status)
		}
	}
}

func TestBuildBlocksConflictingRules(t *testing.T) {
	scan := model.ScanReport{Schema: model.ScanSchema, Nodes: []model.Node{file(tempDir+"old.tmp", "01", 30*24*time.Hour)}}
	twice := append(rules.Builtin()[:1], rules.Builtin()[0])
	twice[1].ID = "user-temp-copy"

	result, err := Build(context.Background(), scan, twice, &fakeProber{status: model.EvidencePass}, now, time.Hour)

	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].Executable || result.Items[0].Evidence[0].Status != model.EvidenceFail {
		t.Fatalf("conflicting rules item = %#v, want one blocked item", result.Items)
	}
}

func TestDigestIgnoresScanOrder(t *testing.T) {
	a := file(tempDir+"a.tmp", "01", 30*24*time.Hour)
	b := file(tempDir+"b.tmp", "02", 30*24*time.Hour)

	first := build(t, []model.Node{a, b}, &fakeProber{status: model.EvidencePass})
	second := build(t, []model.Node{b, a}, &fakeProber{status: model.EvidencePass})

	if first.Digest == "" || first.Digest != second.Digest {
		t.Fatalf("digests differ: %s vs %s", first.Digest, second.Digest)
	}
}

func TestVerifyRejectsTamperedAndExpiredPlans(t *testing.T) {
	original := build(t, []model.Node{file(tempDir+"old.tmp", "01", 30*24*time.Hour)}, &fakeProber{status: model.EvidencePass})
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded model.Plan
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := Verify(decoded, now.Add(time.Minute)); err != nil {
		t.Fatalf("Verify(round-tripped plan) = %v, want nil", err)
	}

	tampered := decoded
	tampered.Items = append([]model.PlanItem(nil), decoded.Items...)
	tampered.Items[0].Path = `C:\Users\me\Documents\keep.docx`
	if err := Verify(tampered, now.Add(time.Minute)); !errors.Is(err, ErrTampered) {
		t.Errorf("Verify(tampered) = %v, want ErrTampered", err)
	}
	if err := Verify(decoded, now.Add(time.Hour)); !errors.Is(err, ErrExpired) {
		t.Errorf("Verify(expired) = %v, want ErrExpired", err)
	}
	if err := Verify(decoded, now.Add(-time.Minute)); err == nil {
		t.Error("Verify(before creation) = nil, want error")
	}
}
