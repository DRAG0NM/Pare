//go:build windows

package scanner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/DRAG0NM/Pare/internal/model"
)

func TestWindowsScannerReadsNestedAndUnicodeEntries(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "子目录"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "子目录", "数据.bin"), []byte("pare"), 0o600); err != nil {
		t.Fatal(err)
	}

	report, err := NewWindowsScanner().Scan(context.Background(), root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if !report.Complete {
		t.Fatalf("Complete = false, errors = %+v", report.Errors)
	}
	if !hasNodePath(report.Nodes, filepath.Join(root, "子目录"), "directory") {
		t.Fatalf("directory node missing from %+v", report.Nodes)
	}
	file := findNode(report.Nodes, filepath.Join(root, "子目录", "数据.bin"))
	if file == nil {
		t.Fatalf("Unicode file node missing from %+v", report.Nodes)
	}
	if file.LogicalSize != 4 || file.FileID == "" {
		t.Fatalf("file metadata = size %d, ID %q; want size 4 and an ID", file.LogicalSize, file.FileID)
	}
}

func TestWindowsScannerReturnsIncompleteReportWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	report, err := NewWindowsScanner().Scan(ctx, t.TempDir())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Scan() error = %v, want context.Canceled", err)
	}
	if report.Complete {
		t.Fatal("Complete = true after cancellation")
	}
}

func TestWindowsScannerSupportsLongPaths(t *testing.T) {
	root := t.TempDir()
	longDir := root
	for len(longDir) < 275 {
		longDir = filepath.Join(longDir, strings.Repeat("d", 40))
		if err := os.Mkdir(longDir, 0o755); err != nil {
			t.Fatalf("create long test directory %q: %v", longDir, err)
		}
	}
	longFile := filepath.Join(longDir, "long-name.txt")
	if err := os.WriteFile(longFile, []byte("long"), 0o600); err != nil {
		t.Fatalf("create long test file: %v", err)
	}

	report, err := NewWindowsScanner().Scan(context.Background(), root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if !report.Complete {
		t.Fatalf("Complete = false, errors = %+v", report.Errors)
	}
	if findNode(report.Nodes, longFile) == nil {
		t.Fatalf("long-path file %q missing", longFile)
	}
}

func TestWindowsScannerEmitsButDoesNotFollowReparsePoint(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "inside.txt"), []byte("target"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		command := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", link, target)
		output, junctionErr := command.CombinedOutput()
		if junctionErr != nil {
			if strings.Contains(strings.ToLower(err.Error()), "privilege is not held") || strings.Contains(strings.ToLower(string(output)), "access is denied") {
				t.Skipf("OS denied test reparse-point creation: symlink: %v; junction: %s", err, strings.TrimSpace(string(output)))
			}
			t.Fatalf("create test reparse point: symlink: %v; junction: %v (%s)", err, junctionErr, strings.TrimSpace(string(output)))
		}
	}

	report, err := NewWindowsScanner().Scan(context.Background(), root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if !hasNodePath(report.Nodes, link, "reparse") {
		t.Fatalf("reparse node missing from %+v", report.Nodes)
	}
	if findNode(report.Nodes, filepath.Join(link, "inside.txt")) != nil {
		t.Fatalf("scanner followed reparse point %q", link)
	}
}

func TestWindowsErrorCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "access denied", err: syscall.ERROR_ACCESS_DENIED, want: "access_denied"},
		{name: "changed entry", err: syscall.ERROR_FILE_NOT_FOUND, want: "changed_entry"},
		{name: "unsupported information class", err: windows.ERROR_INVALID_PARAMETER, want: "unsupported_information_class"},
		{name: "generic I/O", err: errors.New("device failure"), want: "io_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := windowsErrorCode(test.err); got != test.want {
				t.Fatalf("windowsErrorCode(%v) = %q, want %q", test.err, got, test.want)
			}
		})
	}
}

func hasNodePath(nodes []model.Node, path, kind string) bool {
	for _, node := range nodes {
		if node.Path == path && node.Kind == kind {
			return true
		}
	}
	return false
}

func findNode(nodes []model.Node, path string) *model.Node {
	for i := range nodes {
		if nodes[i].Path == path {
			return &nodes[i]
		}
	}
	return nil
}
