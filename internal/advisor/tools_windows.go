//go:build windows

package advisor

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// lockState opens the file for reading with no sharing; nothing is read or written.
func lockState(path string) string {
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "无法检查"
	}
	handle, err := windows.CreateFile(pointer, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	switch {
	case errors.Is(err, windows.ERROR_SHARING_VIOLATION):
		return "正被其他程序使用"
	case err != nil:
		return "无法检查：" + err.Error()
	}
	windows.CloseHandle(handle)
	return "没有程序在使用"
}

func installedPrograms() ([]string, error) {
	roots := []struct {
		key  registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
	}
	seen := map[string]bool{}
	var result []string
	for _, root := range roots {
		parent, err := registry.OpenKey(root.key, root.path, registry.READ)
		if err != nil {
			continue
		}
		names, _ := parent.ReadSubKeyNames(-1)
		for _, name := range names {
			key, err := registry.OpenKey(parent, name, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			display, _, _ := key.GetStringValue("DisplayName")
			if display == "" {
				key.Close()
				continue
			}
			version, _, _ := key.GetStringValue("DisplayVersion")
			publisher, _, _ := key.GetStringValue("Publisher")
			location, _, _ := key.GetStringValue("InstallLocation")
			sizeKB, _, _ := key.GetIntegerValue("EstimatedSize")
			date, _, _ := key.GetStringValue("InstallDate")
			key.Close()
			line := fmt.Sprintf("%s %s｜%s｜安装于 %s｜位置 %s｜约 %s", display, version, publisher, date, location, size(sizeKB*1024))
			if !seen[line] {
				seen[line] = true
				result = append(result, line)
			}
		}
		parent.Close()
	}
	sort.Strings(result)
	return result, nil
}

func runningProcesses() ([]string, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	counts := map[string]int{}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		name := windows.UTF16ToString(entry.ExeFile[:])
		path := processPath(entry.ProcessID)
		counts[strings.TrimSpace(name+"  "+path)]++
	}
	var result []string
	for line, count := range counts {
		if count > 1 {
			line += fmt.Sprintf("  ×%d", count)
		}
		result = append(result, line)
	}
	sort.Strings(result)
	return result, nil
}

func processPath(pid uint32) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)
	buffer := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buffer))
	if windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size) != nil {
		return ""
	}
	return windows.UTF16ToString(buffer[:size])
}
