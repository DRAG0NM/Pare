//go:build !windows

package advisor

import (
	"context"
	"errors"
	"os/exec"
)

var errWindowsOnly = errors.New("只在 Windows 上可用")

func runCommand(ctx context.Context, argv []string) ([]byte, error) {
	return exec.CommandContext(ctx, argv[0], argv[1:]...).CombinedOutput()
}

func decodeText(data []byte) string { return string(data) }

func lockState(string) string { return "无法检查（只在 Windows 上可用）" }

func installedPrograms() ([]string, error) { return nil, errWindowsOnly }

func runningProcesses() ([]string, error) { return nil, errWindowsOnly }
