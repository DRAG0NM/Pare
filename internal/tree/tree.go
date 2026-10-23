// Package tree aggregates a flat scan report into a directory tree with subtree totals.
package tree

import (
	"path/filepath"
	"sort"

	"github.com/DRAG0NM/Pare/internal/model"
)

type Node struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	// LogicalSize and AllocationSize are subtree totals for directories. Hard-linked
	// files count once per path, as Windows Explorer does.
	LogicalSize    uint64 `json:"logical_size"`
	AllocationSize uint64 `json:"allocation_size"`
	Files          int    `json:"files"`
	Directories    int    `json:"directories"`
	// Incomplete marks a directory whose subtree has coverage gaps, so its totals
	// are lower bounds.
	Incomplete bool `json:"incomplete"`
	// Children are sorted by allocation size, largest first.
	Children []*Node `json:"-"`
	Parent   *Node   `json:"-"`
}

type Tree struct {
	Root *Node
	// UniqueAllocation counts each file ID once across the whole tree, the closest
	// estimate of space the scanned files really occupy.
	UniqueAllocation uint64
	nodes            map[string]*Node
}

// Build links scan nodes under root. Nodes whose parent is not in the scan are attached
// to root, so nothing observed is dropped from the totals.
func Build(root string, scan model.ScanReport) *Tree {
	t := &Tree{
		Root:  &Node{Path: root, Name: root, Kind: "directory"},
		nodes: make(map[string]*Node, len(scan.Nodes)+1),
	}
	t.nodes[root] = t.Root

	counted := map[string]bool{}
	for _, scanned := range scan.Nodes {
		node := &Node{Path: scanned.Path, Name: filepath.Base(scanned.Path), Kind: scanned.Kind}
		if scanned.Kind != "directory" {
			node.LogicalSize = uint64(scanned.LogicalSize)
			node.AllocationSize = uint64(scanned.AllocationSize)
		}
		if scanned.Kind == "file" {
			node.Files = 1
			if scanned.FileID == "" || !counted[scanned.FileID] {
				counted[scanned.FileID] = scanned.FileID != ""
				t.UniqueAllocation += uint64(scanned.AllocationSize)
			}
		}
		t.nodes[scanned.Path] = node
	}
	for _, scanned := range scan.Nodes {
		node := t.nodes[scanned.Path]
		parent := t.nodes[scanned.ParentPath]
		if parent == nil || parent == node {
			parent = t.Root
		}
		node.Parent = parent
		parent.Children = append(parent.Children, node)
	}

	aggregate(t.Root)
	for _, gap := range scan.Errors {
		node := t.nodes[gap.Path]
		if node == nil {
			node = t.Root
		}
		for ; node != nil; node = node.Parent {
			node.Incomplete = true
		}
	}
	if !scan.Complete {
		t.Root.Incomplete = true
	}
	return t
}

func aggregate(node *Node) {
	for _, child := range node.Children {
		aggregate(child)
		node.LogicalSize += child.LogicalSize
		node.AllocationSize += child.AllocationSize
		node.Files += child.Files
		node.Directories += child.Directories
		if child.Kind == "directory" {
			node.Directories++
		}
	}
	sort.Slice(node.Children, func(i, j int) bool {
		a, b := node.Children[i], node.Children[j]
		if a.AllocationSize != b.AllocationSize {
			return a.AllocationSize > b.AllocationSize
		}
		return a.Name < b.Name
	})
}

// Find returns the node at path, or nil.
func (t *Tree) Find(path string) *Node {
	return t.nodes[path]
}
