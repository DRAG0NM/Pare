//go:build windows

package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/sys/windows"
)

func BenchmarkDirectoryScanner(b *testing.B) {
	root := b.TempDir()
	const directoryCount = 8
	const filesPerDirectory = 128
	for directory := 0; directory < directoryCount; directory++ {
		path := filepath.Join(root, fmt.Sprintf("dir-%02d", directory))
		if err := os.Mkdir(path, 0o755); err != nil {
			b.Fatal(err)
		}
		for file := 0; file < filesPerDirectory; file++ {
			name := filepath.Join(path, fmt.Sprintf("file-%03d.bin", file))
			if err := os.WriteFile(name, []byte("pare benchmark sample"), 0o600); err != nil {
				b.Fatal(err)
			}
		}
	}

	entryCount := directoryCount * (filesPerDirectory + 1)
	volumeType, fileSystem := benchmarkVolume(root)
	scanner := NewWindowsScanner()
	if report, err := scanner.Scan(context.Background(), root); err != nil || len(report.Nodes) != entryCount {
		b.Fatalf("fixture scan returned %d entries, error %v; want %d", len(report.Nodes), err, entryCount)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		report, err := scanner.Scan(context.Background(), root)
		if err != nil {
			b.Fatal(err)
		}
		if !report.Complete || len(report.Nodes) != entryCount {
			b.Fatalf("scan complete=%v entries=%d; want complete with %d entries", report.Complete, len(report.Nodes), entryCount)
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(entryCount)/b.Elapsed().Seconds(), "entries/s")
	b.Logf("go=%s os=%s volume_type=%s filesystem=%s cache=warm-after-fixture-scan scope=synthetic-temp-tree entries=%d", runtime.Version(), runtime.GOOS, volumeType, fileSystem, entryCount)
}

func benchmarkVolume(path string) (string, string) {
	volumeRoot := filepath.VolumeName(path) + `\`
	rootPointer, err := windows.UTF16PtrFromString(volumeRoot)
	if err != nil {
		return "unknown", "unknown"
	}
	volumeType := "unknown"
	switch windows.GetDriveType(rootPointer) {
	case windows.DRIVE_FIXED:
		volumeType = "fixed"
	case windows.DRIVE_REMOVABLE:
		volumeType = "removable"
	case windows.DRIVE_REMOTE:
		volumeType = "remote"
	case windows.DRIVE_CDROM:
		volumeType = "cdrom"
	case windows.DRIVE_RAMDISK:
		volumeType = "ramdisk"
	}

	fileSystemName := make([]uint16, 64)
	if err := windows.GetVolumeInformation(rootPointer, nil, 0, nil, nil, nil, &fileSystemName[0], uint32(len(fileSystemName))); err != nil {
		return volumeType, "unknown"
	}
	return volumeType, windows.UTF16ToString(fileSystemName)
}
