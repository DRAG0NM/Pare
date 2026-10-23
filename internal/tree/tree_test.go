package tree

import (
	"testing"

	"github.com/DRAG0NM/Pare/internal/model"
)

func TestBuildAggregatesSubtreesAndGaps(t *testing.T) {
	file := func(path, parent, id string, alloc model.ByteCount) model.Node {
		return model.Node{Path: path, ParentPath: parent, Kind: "file", FileID: id, LogicalSize: alloc - 1, AllocationSize: alloc}
	}
	dir := func(path, parent string) model.Node {
		return model.Node{Path: path, ParentPath: parent, Kind: "directory", FileID: path}
	}
	scan := model.ScanReport{
		Complete: false,
		Nodes: []model.Node{
			file(`R:\b\big.bin`, `R:\b`, "1", 100),
			dir(`R:\a`, `R:`),
			dir(`R:\b`, `R:`),
			dir(`R:\a\deep`, `R:\a`),
			file(`R:\a\deep\x`, `R:\a\deep`, "2", 10),
			file(`R:\a\link-to-x`, `R:\a`, "2", 10),
			dir(`R:\locked`, `R:`),
			{Path: `R:\junction`, ParentPath: `R:`, Kind: "reparse"},
		},
		Errors: []model.ScanError{{Path: `R:\locked`, Operation: "enumerate", Code: "access_denied"}},
	}

	result := Build(`R:`, scan)

	root := result.Root
	if root.AllocationSize != 120 || root.LogicalSize != 117 || root.Files != 3 || root.Directories != 4 {
		t.Fatalf("root = %+v", root)
	}
	if result.UniqueAllocation != 110 {
		t.Errorf("UniqueAllocation = %d, want 110 (hard link counted once)", result.UniqueAllocation)
	}
	if got := []string{root.Children[0].Name, root.Children[1].Name}; got[0] != "b" || got[1] != "a" {
		t.Errorf("children order = %v, want largest first", got)
	}
	a := result.Find(`R:\a`)
	if a.AllocationSize != 20 || a.Files != 2 || a.Directories != 1 || a.Incomplete {
		t.Errorf("a = %+v", a)
	}
	if !result.Find(`R:\locked`).Incomplete || !root.Incomplete || result.Find(`R:\b`).Incomplete {
		t.Error("coverage gap must mark the failed directory and its ancestors only")
	}
}
