//go:build windows

package serve

import (
	"golang.org/x/sys/windows"

	"github.com/DRAG0NM/Pare/internal/executor"
)

// volumeSpace returns the size and free space of the volume whose root is root.
func volumeSpace(root string) (total, free uint64, ok bool) {
	pointer, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return 0, 0, false
	}
	var totalFree uint64
	if windows.GetDiskFreeSpaceEx(pointer, &free, &total, &totalFree) != nil {
		return 0, 0, false
	}
	return total, free, true
}

// defaultQuarantineRoots lists the quarantine folder of every local fixed or removable drive.
func defaultQuarantineRoots() []string {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}
	var roots []string
	for i := 0; i < 26; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		pointer, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		switch windows.GetDriveType(pointer) {
		case windows.DRIVE_FIXED, windows.DRIVE_REMOVABLE:
			roots = append(roots, executor.DefaultQuarantineRoot(root))
		}
	}
	return roots
}
