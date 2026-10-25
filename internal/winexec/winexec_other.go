//go:build !windows

package winexec

import (
	"context"
	"os/exec"
)

func Run(ctx context.Context, argv []string) (string, error) {
	output, err := exec.CommandContext(ctx, argv[0], argv[1:]...).CombinedOutput()
	return string(output), err
}

func Decode(data []byte) string { return string(data) }

func RunShell(ctx context.Context, line string, admin bool) (string, error) {
	output, err := exec.CommandContext(ctx, "sh", "-c", line).CombinedOutput()
	return string(output), err
}
