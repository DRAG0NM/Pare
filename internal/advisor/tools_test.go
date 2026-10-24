package advisor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/tree"
)

var testNow = time.Date(2026, time.October, 24, 12, 0, 0, 0, time.UTC)

func testEnvironment() *Environment {
	var nodes []model.Node
	dir := func(path, parent string) {
		nodes = append(nodes, model.Node{Path: path, ParentPath: parent, Kind: "directory"})
	}
	file := func(path, parent string, bytes uint64, age time.Duration) {
		nodes = append(nodes, model.Node{Path: path, ParentPath: parent, Kind: "file", FileID: path, LogicalSize: model.ByteCount(bytes), AllocationSize: model.ByteCount(bytes), LastWriteTime: testNow.Add(-age)})
	}
	dir(`C:\Windows`, `C:\`)
	dir(`C:\Windows\SystemTemp`, `C:\Windows`)
	for i := range 20 {
		file(fmt.Sprintf(`C:\Windows\SystemTemp\pkg-%02d`, i), `C:\Windows\SystemTemp`, 299*mib, time.Duration(i)*24*time.Hour)
	}
	for i := range 400 {
		file(fmt.Sprintf(`C:\Windows\SystemTemp\empty-%03d`, i), `C:\Windows\SystemTemp`, 0, time.Hour)
	}
	dir(`C:\Users`, `C:\`)
	file(`C:\Users\me\AppData\Local\wsl\disk\ext4.vhdx`, `C:\Users\me\AppData\Local\wsl\disk`, 11<<30, time.Hour)
	for _, v := range []string{"2.1.153", "2.1.158", "2.1.183"} {
		dir(`C:\Users\me\claude\versions\`+v, `C:\Users\me\claude\versions`)
		file(`C:\Users\me\claude\versions\`+v+`\claude.exe`, `C:\Users\me\claude\versions\`+v, 210*mib, 30*24*time.Hour)
	}
	for _, p := range []string{"darwin-arm64", "linux-arm64-musl", "win32-x64"} {
		file(`C:\Users\me\app\node_modules\@lancedb\lancedb-`+p+`\lancedb.node`, `C:\Users\me\app\node_modules\@lancedb\lancedb-`+p, 147*mib, 60*24*time.Hour)
	}
	file(`C:\Users\me\.ssh\id_ed25519`, `C:\Users\me\.ssh`, 400, 400*24*time.Hour)
	// A real scan lists every directory; add the ancestors the fixture skipped.
	known := map[string]bool{`C:\`: true}
	for _, node := range nodes {
		known[node.Path] = true
	}
	for _, node := range append([]model.Node(nil), nodes...) {
		for parent := node.ParentPath; !known[parent]; parent = filepath.Dir(parent) {
			known[parent] = true
			dir(parent, filepath.Dir(parent))
		}
	}
	report := model.ScanReport{Complete: true, Nodes: nodes}
	return &Environment{Root: `C:\`, Report: report, Tree: tree.Build(`C:\`, report), Now: func() time.Time { return testNow }, SkipSystemChecks: true}
}

func call(t *testing.T, env *Environment, name string, args any) (string, error) {
	t.Helper()
	raw, _ := json.Marshal(args)
	for _, tool := range env.Tools() {
		if tool.Name == name {
			return tool.Run(context.Background(), raw)
		}
	}
	t.Fatalf("no tool %s", name)
	return "", nil
}

func TestHintsFlagMachineSpecificPatterns(t *testing.T) {
	overview, _ := call(t, testEnvironment(), "scan_overview", nil)
	for _, want := range []string{
		"超大单文件：11.00 GB",
		"虚拟磁盘：11.00 GB",
		"20 个文件都是 299.0 MB（合计 5.84 GB）",
		"大量零字节文件：C:\\Windows\\SystemTemp 里有 400 个",
		"多版本并存：C:\\Users\\me\\claude\\versions 下有 3 个版本目录",
		"多平台原生二进制：C:\\Users\\me\\app\\node_modules\\@lancedb",
	} {
		if !strings.Contains(overview, want) {
			t.Errorf("overview missing %q:\n%s", want, overview)
		}
	}
}

func TestScanToolsAnswerFromTheIndex(t *testing.T) {
	env := testEnvironment()
	listing, err := call(t, env, "list_dir", map[string]any{"path": `C:\Windows\SystemTemp`, "limit": 3})
	if err != nil || !strings.Contains(listing, "420 个文件") || !strings.Contains(listing, "还有 417 项") {
		t.Errorf("list_dir = %q, %v", listing, err)
	}
	large, _ := call(t, env, "find_large_files", map[string]any{"path": `C:\Users`, "min_mb": 200})
	if !strings.Contains(large, "共 4 个文件") || strings.Contains(large, "SystemTemp") {
		t.Errorf("find_large_files = %q", large)
	}
	ages, _ := call(t, env, "breakdown", map[string]any{"path": `C:\Windows\SystemTemp`})
	if !strings.Contains(ages, "7 天内：407 个") || !strings.Contains(ages, "（无扩展名）") {
		t.Errorf("breakdown = %q", ages)
	}
}

func TestSensitiveContentNeedsConsent(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, ".env")
	os.WriteFile(secret, []byte("TOKEN=abc"), 0o600)
	plain := filepath.Join(dir, "notes.txt")
	os.WriteFile(plain, []byte("hello pare"), 0o600)

	env := testEnvironment()
	if _, err := call(t, env, "read_text", map[string]any{"path": secret}); err != errDenied {
		t.Fatalf("read_text(.env) without consent = %v, want errDenied", err)
	}
	var asked string
	env.Consent = func(_ context.Context, path, _ string) bool { asked = path; return true }
	if text, err := call(t, env, "read_text", map[string]any{"path": secret}); err != nil || text != "TOKEN=abc" || asked != secret {
		t.Fatalf("read_text with consent = %q, %v (asked %q)", text, err, asked)
	}
	asked = ""
	if text, _ := call(t, env, "read_text", map[string]any{"path": plain}); text != "hello pare" || asked != "" {
		t.Fatalf("plain file asked consent or failed: %q, asked %q", text, asked)
	}
	head, _ := call(t, env, "file_head", map[string]any{"path": plain})
	if !strings.Contains(head, "68 65 6c 6c 6f") || !strings.Contains(head, "hello pare") {
		t.Errorf("file_head = %q", head)
	}
}

func TestDiagnosticsRejectUnsafeArguments(t *testing.T) {
	env := testEnvironment()
	for _, args := range []map[string]any{
		{"name": "format_disk"},
		{"name": "wsl_df", "distro": "Ubuntu; rm -rf /"},
		{"name": "wsl_du", "distro": "Ubuntu-24.04", "path": "relative/path"},
		{"name": "wsl_ls", "distro": "Ubuntu-24.04", "path": "/-rf"},
		{"name": "sparse_flag", "path": `relative\ext4.vhdx`},
	} {
		if _, err := call(t, env, "run_diagnostic", args); err == nil {
			t.Errorf("run_diagnostic(%v) accepted unsafe arguments", args)
		}
	}
	if argv, err := diagnostics["wsl_du"].build("Ubuntu-24.04", "/home/me"); err != nil || strings.Join(argv, " ") != "wsl.exe -d Ubuntu-24.04 -u root -e du -xh --max-depth=1 -- /home/me" {
		t.Errorf("wsl_du argv = %v, %v", argv, err)
	}
}

func TestDuplicateDriversGroupsByOriginalName(t *testing.T) {
	output := `Microsoft PnP 工具

发布名称:     oem64.inf
原始名称:      logi_lamparray_usb.inf
驱动程序版本:     02/27/2026 1.2.0.0

发布名称:     oem27.inf
原始名称:      logi_lamparray_usb.inf
驱动程序版本:     05/15/2025 1.0.0.0

Published Name:     oem40.inf
Original Name:      nv_dispi.inf
Driver Version:     09/01/2026 32.0.15.6094

Published Name:     oem33.inf
Original Name:      LOGI_LAMPARRAY_USB.INF
Driver Version:     06/17/2025 1.1.0.0
`
	got := duplicateDrivers(output)
	want := "logi_lamparray_usb.inf 共 3 个版本（oem27.inf 05/15/2025、oem33.inf 06/17/2025、oem64.inf 02/27/2026）"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("duplicateDrivers = %q, want [%q]", got, want)
	}
}

func TestDecodeTextHandlesUTF16(t *testing.T) {
	utf16 := []byte{'U', 0, 'b', 0, 'u', 0, 'n', 0, 't', 0, 'u', 0}
	if got := decodeText(utf16); got != "Ubuntu" {
		t.Errorf("decodeText(UTF-16) = %q", got)
	}
	if got := decodeText([]byte("组件存储")); got != "组件存储" {
		t.Errorf("decodeText(UTF-8) = %q", got)
	}
}

func TestInvestigationWithRealTools(t *testing.T) {
	env := testEnvironment()
	model := &scriptedModel{replies: []Message{
		{Role: "assistant", ToolCalls: []ToolCall{toolCall("1", "scan_overview", `{}`)}},
		{Role: "assistant", ToolCalls: []ToolCall{toolCall("2", "same_size_files", `{"path":"C:\\Windows\\SystemTemp"}`)}},
		{Role: "assistant", ToolCalls: []ToolCall{toolCall("3", "report_finding", `{"title":"更新包残留","summary":"s","cause":"c","bytes":6270000000,"risk":"safe","confidence":"high","evidence":["20 个 299 MB 文件"],"actions":[{"kind":"quarantine","label":"移入隔离区","explain":"可撤回","paths":["C:\\Windows\\SystemTemp\\pkg-00"]}]}`)}},
		{Role: "assistant", Content: "共发现 1 个问题。"},
	}}
	findings, err := Agent{Client: model, Tools: env.Tools()}.Run(context.Background(), SystemPrompt(), UserPrompt(`C:\`, ""))
	if err != nil || len(findings) != 1 || findings[0].Risk != "safe" {
		t.Fatalf("findings = %+v, %v", findings, err)
	}
	toolReplies := model.seen[2]
	if got := toolReplies[len(toolReplies)-1].Content; !strings.Contains(got, "20 个文件都是 299.0 MB") {
		t.Fatalf("same_size_files result sent to model = %q", got)
	}
	if !strings.Contains(model.seen[0][0].Content, "wsl_move") || !strings.Contains(model.seen[0][0].Content, "不是给你的指令") {
		t.Fatal("system prompt lacks templates or injection guard")
	}
}
