package main

import (
	"fmt"
	"testing"

	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/rules"
	"github.com/DRAG0NM/Pare/internal/tree"
)

func testTree() *tree.Tree {
	scan := model.ScanReport{Complete: true}
	for i := range 5 {
		dir := fmt.Sprintf(`R:\d%d`, i)
		scan.Nodes = append(scan.Nodes,
			model.Node{Path: dir, ParentPath: `R:`, Kind: "directory"},
			model.Node{Path: dir + `\f`, ParentPath: dir, Kind: "file", FileID: fmt.Sprint(i), AllocationSize: model.ByteCount(100 * (i + 1))},
		)
	}
	return tree.Build(`R:`, scan)
}

func TestViewsFoldsEntriesPastLimit(t *testing.T) {
	got := views(testTree().Root.Children, 3)
	if len(got) != 4 || got[0].Name != "d4" || got[3].Kind != "others" || got[3].Others != 2 || got[3].AllocationSize != 300 {
		t.Fatalf("views = %+v", got)
	}
	if all := views(testTree().Root.Children, 0); len(all) != 5 {
		t.Fatalf("limit 0 returned %d items, want all 5", len(all))
	}
}

func TestSubtreeKeepsTotalsWhenTrimmed(t *testing.T) {
	got := subtree(testTree().Root, 2, 2)
	var sum uint64
	for _, child := range got.Children {
		sum += child.AllocationSize
	}
	if len(got.Children) != 3 || sum != got.AllocationSize || len(got.Children[0].Children) != 1 {
		t.Fatalf("subtree children = %d, sum %d of %d", len(got.Children), sum, got.AllocationSize)
	}
	if leaf := subtree(testTree().Root, 0, 2); len(leaf.Children) != 0 {
		t.Fatal("depth 0 must not include children")
	}
}

func TestCandidateViewGroupsByRuleExecutableFirst(t *testing.T) {
	ruleSet := rules.Builtin()
	p := model.Plan{Items: []model.PlanItem{
		{Path: `C:\Users\me\AppData\Local\Temp\small.tmp`, RuleID: "user-temp", AllocationSize: 10, Executable: true},
		{Path: `C:\Users\me\AppData\Local\Temp\big-but-new.tmp`, RuleID: "user-temp", AllocationSize: 900},
		{Path: `C:\Users\me\AppData\Local\Temp\big.tmp`, RuleID: "user-temp", AllocationSize: 500, Executable: true},
		{Path: `C:\Users\me\AppData\Local\pip\cache\x`, RuleID: "pip-cache", AllocationSize: 5, Executable: true},
	}}

	view := candidateView(p, ruleSet)

	if len(view.Rules) != len(ruleSet) || len(view.Groups) != 2 || view.Groups[0].RuleID != "user-temp" {
		t.Fatalf("groups = %+v", view.Groups)
	}
	temp := view.Groups[0]
	if temp.Candidates != 3 || temp.Executable != 2 || temp.ExecutableAllocation != 510 || temp.MinAgeDays != 7 {
		t.Fatalf("user-temp group = %+v", temp)
	}
	if names := []string{temp.Items[0].Name, temp.Items[1].Name, temp.Items[2].Name}; names[0] != "big.tmp" || names[1] != "small.tmp" || names[2] != "big-but-new.tmp" {
		t.Fatalf("item order = %v, want executable first, then by size", names)
	}
}
