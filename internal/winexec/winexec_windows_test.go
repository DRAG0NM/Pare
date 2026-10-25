//go:build windows

package winexec

import (
	"context"
	"strings"
	"testing"
)

// Full paths, so a whoami from Git or MSYS on PATH cannot stand in for the system one.
const (
	whoami = `"%SystemRoot%\System32\whoami.exe" /groups`
	// net session only works for administrators.
	adminOnly = `"%SystemRoot%\System32\net.exe" session`
)

// An AI-written command must not keep Pare's administrator rights unless the user
// allowed it for that command.
func TestRunShellDropsElevation(t *testing.T) {
	if !elevated() {
		t.Skip("needs an elevated test process")
	}
	output, err := RunShell(context.Background(), whoami, false)
	if err != nil {
		t.Fatalf("RunShell: %v\n%s", err, output)
	}
	if !strings.Contains(output, "S-1-16-8192") || strings.Contains(output, "S-1-16-12288") {
		t.Errorf("command did not run at medium integrity:\n%s", output)
	}
	if output, err := RunShell(context.Background(), adminOnly, false); err == nil {
		t.Errorf("an admin-only command succeeded without consent:\n%s", output)
	}
	if output, err := RunShell(context.Background(), adminOnly, true); err != nil {
		t.Errorf("an admin-only command failed although admin was allowed: %v\n%s", err, output)
	}
}
