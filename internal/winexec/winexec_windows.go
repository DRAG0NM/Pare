//go:build windows

// Package winexec runs external programs without a shell or console window and turns
// their output into text, whatever encoding the program used.
package winexec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/sys/windows"
)

const createNoWindow = 0x08000000

// Run runs argv without a shell and without flashing a console window. Stdout and
// stderr are decoded separately: wsl.exe writes its own messages as UTF-16 while the
// Linux command inside writes UTF-8, and mixing them garbles both.
func Run(ctx context.Context, argv []string) (string, error) {
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	command.Env = append(os.Environ(), "WSL_UTF8=1")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	text := strings.TrimSpace(Decode(stdout.Bytes()))
	if extra := strings.TrimSpace(Decode(stderr.Bytes())); extra != "" {
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

// Decode turns command output or file bytes into text: UTF-16 (wsl.exe and files with
// a BOM), UTF-8, or the console's OEM code page (GBK on Chinese Windows, used by DISM).
func Decode(data []byte) string {
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

// RunShell runs one command line through cmd.exe exactly as written. It is only used for
// commands the user explicitly chose to run; the line is passed raw so quotes, pipes and
// redirections behave as they would in a console.
func RunShell(ctx context.Context, line string) (string, error) {
	command := exec.CommandContext(ctx, "cmd.exe")
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow, CmdLine: `cmd.exe /d /s /c "` + line + `"`}
	command.Env = append(os.Environ(), "WSL_UTF8=1")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	text := strings.TrimSpace(Decode(stdout.Bytes()) + "\n" + Decode(stderr.Bytes()))
	if ctx.Err() != nil {
		return text, fmt.Errorf("超时或被取消")
	}
	return text, err
}
