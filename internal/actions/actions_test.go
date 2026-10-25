package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DRAG0NM/Pare/internal/advisor"
)

func TestEveryAdvisorTemplateHasABuilder(t *testing.T) {
	for _, template := range advisor.Templates {
		if _, ok := builders[template.ID]; !ok {
			t.Errorf("advisor offers template %q that actions cannot run", template.ID)
		}
	}
}

func TestPrepareValidatesParameters(t *testing.T) {
	parent := t.TempDir()
	existing := filepath.Join(parent, "exists")
	os.Mkdir(existing, 0o755)

	good, err := Prepare("wsl_move", map[string]string{"distro": "Ubuntu-24.04", "target": filepath.Join(parent, "WSL")})
	if err != nil || len(good.Steps) != 2 || good.Steps[1][3] != "--move" || len(good.Warnings) == 0 {
		t.Fatalf("wsl_move = %+v, %v", good, err)
	}
	if prepared, err := Prepare("driver_delete", map[string]string{"inf": "OEM27.INF"}); err != nil || strings.Join(prepared.Display(), "") != "pnputil.exe /delete-driver oem27.inf" || !prepared.NeedsAdmin {
		t.Fatalf("driver_delete = %+v, %v", prepared, err)
	}

	for name, params := range map[string]map[string]string{
		"wsl_move":           {"distro": "Ubuntu & del", "target": filepath.Join(parent, "x")},
		"wsl_set_sparse":     {"distro": ""},
		"wsl_journal_vacuum": {"distro": "Ubuntu", "size": "100M; rm -rf /"},
		"wsl_journal_limit":  {"distro": "Ubuntu", "size": "200M' && reboot '"},
		"driver_delete":      {"inf": "nv_dispi.inf"},
	} {
		if _, err := Prepare(name, params); err == nil {
			t.Errorf("Prepare(%s, %v) accepted bad parameters", name, params)
		}
	}
	for _, to := range []string{existing, `relative\dir`, `C:\Windows\WSL`, filepath.Join(parent, "missing", "deeper")} {
		if _, err := Prepare("wsl_move", map[string]string{"distro": "Ubuntu", "target": to}); err == nil {
			t.Errorf("wsl_move accepted target %q", to)
		}
	}
	if _, err := Prepare("format_disk", nil); err == nil {
		t.Error("unknown template accepted")
	}
}

func TestPrepareCommandKeepsTheLineAndWarns(t *testing.T) {
	prepared, err := PrepareCommand(`  npm rebuild @lancedb/lancedb  `)
	if err != nil || prepared.Shell != "npm rebuild @lancedb/lancedb" || len(prepared.Warnings) < 2 {
		t.Fatalf("PrepareCommand = %+v, %v", prepared, err)
	}
	if _, err := PrepareCommand("echo a\r\ndel x"); err == nil {
		t.Error("multi-line command accepted")
	}
}
