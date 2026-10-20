package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/DRAG0NM/Pare/internal/model"
)

func TestWalkScannerEnumeratesNestedAndEmptyDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "nested", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "note.txt"), []byte("pare"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := NewWalkScanner().Scan(context.Background(), root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if !report.Complete {
		t.Fatalf("Complete = false, want true; errors: %#v", report.Errors)
	}
	if got, want := len(report.Nodes), 3; got != want {
		t.Fatalf("node count = %d, want %d", got, want)
	}
	byPath := make(map[string]model.Node, len(report.Nodes))
	for _, node := range report.Nodes {
		byPath[node.Path] = node
	}
	file, ok := byPath[filepath.Join(root, "nested", "note.txt")]
	if !ok || file.Kind != "file" || file.LogicalSize != 4 {
		t.Fatalf("file node = %#v, want a 4-byte file", file)
	}
	if empty, ok := byPath[filepath.Join(root, "nested", "empty")]; !ok || empty.Kind != "directory" {
		t.Fatalf("empty directory node = %#v, want it in the report", empty)
	}
}

func TestWalkScannerReturnsCancellationBeforeReading(t *testing.T) {
	root := t.TempDir()
	calls := 0
	scanner := NewWalkScannerWithReader(func(string) ([]os.DirEntry, error) {
		calls++
		return nil, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	report, err := scanner.Scan(ctx, root)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Scan() error = %v, want context.Canceled", err)
	}
	if report.Complete {
		t.Fatal("Complete = true, want canceled scan to be incomplete")
	}
	if calls != 0 {
		t.Fatalf("ReadDir calls = %d, want 0 after pre-cancellation", calls)
	}
}

func TestWalkScannerReportsChildReadFailure(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "restricted")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	reader := func(path string) ([]os.DirEntry, error) {
		if filepath.Clean(path) == filepath.Clean(child) {
			return nil, os.ErrPermission
		}
		return os.ReadDir(path)
	}

	report, err := NewWalkScannerWithReader(reader).Scan(context.Background(), root)
	if err != nil {
		t.Fatalf("Scan() error = %v, want partial report without fatal error", err)
	}
	if report.Complete {
		t.Fatal("Complete = true, want incomplete for child read failure")
	}
	if len(report.Errors) != 1 || report.Errors[0].Path != child || report.Errors[0].Code != "access_denied" {
		t.Fatalf("Errors = %#v, want one access_denied error for %q", report.Errors, child)
	}
}
