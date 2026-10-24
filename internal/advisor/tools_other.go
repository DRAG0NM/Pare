//go:build !windows

package advisor

import (
	"context"
	"errors"
	"os/exec"
)

var errWindowsOnly = errors.New("只在 Windows 上可用")

func runCommand(ctx context.Context, argv []string) (string, error) {
	output, err := exec.CommandContext(ctx, argv[0], argv[1:]...).CombinedOutput()
	return string(output), err
}

func decodeText(data []byte) string { return string(data) }

func lockState(string) string { return "无法检查（只在 Windows 上可用）" }

func installedPrograms() ([]string, error) { return nil, errWindowsOnly }

func runningProcesses() ([]string, error) { return nil, errWindowsOnly }
