package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wails "github.com/wailsapp/wails/v2/pkg/runtime"
)

// appVersion matches the version resource in winres/winres.json.
const appVersion = "0.2.0"

// logLimit is where the log rotates to pare.log.1, so the two files stay small.
const logLimit = 2 << 20

var logMu sync.Mutex

func (a *App) logDir() string {
	if a.logFolder != "" {
		return a.logFolder
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Pare", "logs")
}

// logf appends one line to the log. It is best effort and never fails the caller. The
// API key is never passed in; paths and file names are, which the export dialog says.
func (a *App) logf(format string, args ...any) {
	dir := a.logDir()
	if dir == "" {
		return
	}
	logMu.Lock()
	defer logMu.Unlock()
	os.MkdirAll(dir, 0o700)
	path := filepath.Join(dir, "pare.log")
	if info, err := os.Stat(path); err == nil && info.Size() > logLimit {
		os.Rename(path, path+".1")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	line := strings.ReplaceAll(fmt.Sprintf(format, args...), "\n", " ⏎ ")
	fmt.Fprintf(file, "%s  %s\n", time.Now().Format("2006-01-02 15:04:05"), line)
}

// buildLogExport collects what a bug report needs into one text.
func (a *App) buildLogExport() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Pare %s 日志导出 · %s\n\n", appVersion, time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "系统：%s（%s/%s）\n", osVersion(), runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "管理员身份：%v\n", a.Elevated())
	if settings, _, err := loadAISettings(); err == nil {
		fmt.Fprintf(&b, "AI 服务：%s · %s · %s · 已保存 Key：%v\n", settings.Provider, settings.BaseURL, settings.Model, settings.APIKey != "")
	}
	for _, drive := range a.Drives() {
		fmt.Fprintf(&b, "磁盘 %s：可用 %.1f GB / 共 %.1f GB\n", drive.Root, float64(drive.Free)/(1<<30), float64(drive.Total)/(1<<30))
	}

	b.WriteString("\n===== 最近的清理记录 =====\n")
	history := a.ListHistory()
	if len(history) == 0 {
		b.WriteString("（无）\n")
	}
	for i, entry := range history {
		if i == 20 {
			break
		}
		fmt.Fprintf(&b, "%s %s %s 释放 %d 字节，隔离 %d 字节，%d 项\n", entry.Time.Format("2006-01-02 15:04"), entry.Kind, entry.Volume, entry.FreedBytes, entry.QuarantinedBytes, entry.Items)
		for _, action := range entry.Actions {
			fmt.Fprintf(&b, "    [%s] %s / %s（%s）%d 字节\n", action.Status, action.FindingTitle, action.Label, action.Kind, action.Bytes)
		}
	}

	b.WriteString("\n===== 运行日志 =====\n")
	logMu.Lock()
	for _, name := range []string{"pare.log.1", "pare.log"} {
		if data, err := os.ReadFile(filepath.Join(a.logDir(), name)); err == nil {
			b.Write(data)
		}
	}
	logMu.Unlock()
	return b.String()
}

// ExportLog asks where to save the report, writes it and shows it in Explorer. It
// returns the saved path, or "" when the user cancelled.
func (a *App) ExportLog() (string, error) {
	path, err := wails.SaveFileDialog(a.ctx, wails.SaveDialogOptions{
		Title:           "导出日志",
		DefaultFilename: "pare-log-" + time.Now().Format("20060102-1504") + ".txt",
		Filters:         []wails.FileFilter{{DisplayName: "文本文件 (*.txt)", Pattern: "*.txt"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	if err := os.WriteFile(path, []byte(a.buildLogExport()), 0o600); err != nil {
		return "", errors.New("写入失败：" + err.Error())
	}
	exec.Command("explorer.exe", "/select,", path).Start()
	return path, nil
}

// OpenIssuePage opens the project's new-issue page in the default browser.
func (a *App) OpenIssuePage() {
	wails.BrowserOpenURL(a.ctx, "https://github.com/DRAG0NM/Pare/issues/new")
}

// clipLog keeps command output in the log short.
func clipLog(s string) string {
	if r := []rune(s); len(r) > 600 {
		return string(r[:600]) + "…"
	}
	return s
}
