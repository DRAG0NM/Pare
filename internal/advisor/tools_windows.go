//go:build windows

package advisor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const createNoWindow = 0x08000000

// runCommand runs argv without a shell and without flashing a console window. Stdout and
// stderr are decoded separately: wsl.exe writes its own messages as UTF-16 while the
// Linux command inside writes UTF-8, and mixing them garbles both.
func runCommand(ctx context.Context, argv []string) (string, error) {
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	command.Env = append(os.Environ(), "WSL_UTF8=1")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	text := strings.TrimSpace(decodeText(stdout.Bytes()))
	if extra := strings.TrimSpace(decodeText(stderr.Bytes())); extra != "" {
		text = strings.TrimSpace(text + "\n" + extra)
	}
	if ctx.Err() != nil {
		return text, fmt.Errorf("超时或被取消")
	}
	return text, err
}

var (
	kernel32            = windows.NewLazySystemDLL("kernel32.dll")
	multiByteToWideChar = kernel32.NewProc("MultiByteToWideChar")
)

// decodeText turns command output or file bytes into text: UTF-16 (wsl.exe and files with
// a BOM), UTF-8, or the console's OEM code page (GBK on Chinese Windows, used by DISM).
func decodeText(data []byte) string {
	if len(data) >= 2 && data[0] == 0xff && data[1] == 0xfe {
		return utf16Text(data[2:])
	}
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		data = data[3:]
	}
	if looksUTF16(data) {
		return utf16Text(data)
	}
	if utf8.Valid(data) {
		return string(data)
	}
	return oemText(data)
}

func looksUTF16(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	zeros := 0
	for i := 1; i < len(data); i += 2 {
		if data[i] == 0 {
			zeros++
		}
	}
	return zeros*3 > len(data)/2
}

func utf16Text(data []byte) string {
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = uint16(data[2*i]) | uint16(data[2*i+1])<<8
	}
	return string(utf16.Decode(units))
}

func oemText(data []byte) string {
	const cpOEM = 1
	n, _, _ := multiByteToWideChar.Call(cpOEM, 0, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), 0, 0)
	if n == 0 {
		return string(data)
	}
	wide := make([]uint16, n)
	multiByteToWideChar.Call(cpOEM, 0, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), uintptr(unsafe.Pointer(&wide[0])), n)
	return string(utf16.Decode(wide))
}

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
