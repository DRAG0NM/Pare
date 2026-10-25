//go:build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

type Drive struct {
	Root  string `json:"root"`
	Label string `json:"label"`
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
}

// Elevated reports whether the app runs with an administrator token. Only then can the
// scanner enable the backup privilege that reads system-only folders.
func (a *App) Elevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// osVersion names the Windows build for log exports, for example "Windows 10.0.26100".
func osVersion() string {
	v := windows.RtlGetVersion()
	return fmt.Sprintf("Windows %d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber)
}

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

// Drives lists local fixed and removable drives. Network and optical drives are left out.
func (a *App) Drives() []Drive {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}
	var drives []Drive
	for i := 0; i < 26; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		pointer, _ := windows.UTF16PtrFromString(root)
		kind := windows.GetDriveType(pointer)
		if kind != windows.DRIVE_FIXED && kind != windows.DRIVE_REMOVABLE {
			continue
		}
		drive := Drive{Root: root}
		var free, total, totalFree uint64
		if windows.GetDiskFreeSpaceEx(pointer, &free, &total, &totalFree) == nil {
			drive.Total, drive.Free = total, free
		}
		label := make([]uint16, windows.MAX_PATH+1)
		if windows.GetVolumeInformation(pointer, &label[0], uint32(len(label)), nil, nil, nil, nil, 0) == nil {
			drive.Label = windows.UTF16ToString(label)
		}
		drives = append(drives, drive)
	}
	return drives
}
