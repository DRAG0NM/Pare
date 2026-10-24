//go:build !windows

package advisor

import (
	"errors"
)

var errWindowsOnly = errors.New("只在 Windows 上可用")

func lockState(string) string { return "无法检查（只在 Windows 上可用）" }

func installedPrograms() ([]string, error) { return nil, errWindowsOnly }

func runningProcesses() ([]string, error) { return nil, errWindowsOnly }
