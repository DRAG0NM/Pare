//go:build windows

package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DRAG0NM/Pare/internal/executor"
)

func call(t *testing.T, s *Server, method string, params any) any {
	t.Helper()
	raw, _ := json.Marshal(params)
	result, err := s.handle(context.Background(), method, raw)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return result
}

func TestServeHelloOverTheWire(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(`{"id":7,"method":"hello"}` + "\n" + `{"id":8,"method":"nope"}` + "\n")
	if err := New(Options{Version: "test"}).Run(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"id":7`) || !strings.Contains(lines[0], Schema) || !strings.Contains(lines[0], `"list_dir"`) {
		t.Fatalf("hello = %q", out.String())
	}
	if !strings.Contains(lines[1], `"id":8`) || !strings.Contains(lines[1], "unknown method") {
		t.Fatalf("unknown method reply = %q", lines[1])
	}
}

func TestServeCleanupCycle(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "cache")
	os.MkdirAll(cache, 0o755)
	os.WriteFile(filepath.Join(cache, "a.bin"), bytes.Repeat([]byte("x"), 4096), 0o600)
	quarantine := filepath.Join(t.TempDir(), "q")
	s := New(Options{
		QuarantineRoot:   func(string) string { return quarantine },
		QuarantineRoots:  func() []string { return []string{quarantine} },
		SkipSystemChecks: true,
	})

	if _, err := s.handle(context.Background(), "tool", json.RawMessage(`{"name":"list_dir"}`)); err == nil {
		t.Fatal("tool before scan should fail")
	}
	summary := call(t, s, "scan", map[string]string{"root": root}).(ScanSummary)
	if summary.Files != 1 || !summary.Complete {
		t.Fatalf("scan = %+v", summary)
	}
	if listing := call(t, s, "tool", map[string]any{"name": "list_dir", "args": map[string]string{"path": root}}).(string); !strings.Contains(listing, "cache") {
		t.Fatalf("list_dir = %q", listing)
	}

	preview := call(t, s, "prepare", PrepareParams{Quarantine: []string{cache, `C:\Windows\System32`}}).(Preview)
	if !preview.Runnable || len(preview.Items) != 2 {
		t.Fatalf("preview = %+v", preview)
	}
	for _, item := range preview.Items {
		if strings.EqualFold(item.Path, `C:\Windows\System32`) && (item.Executable || item.Reason == "") {
			t.Fatalf("protected path not refused: %+v", item)
		}
	}
	if _, err := s.handle(context.Background(), "execute", mustJSON(map[string]string{"id": preview.ID, "digest": "sha256:forged"})); err == nil {
		t.Fatal("execute with a wrong digest should fail")
	}
	result := call(t, s, "execute", map[string]string{"id": preview.ID, "digest": preview.Digest}).(ExecuteResult)
	if result.Quarantined != 1 || result.Manifest == "" || result.Error != "" {
		t.Fatalf("execute = %+v", result)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("cache still in place: %v", err)
	}
	if _, err := s.handle(context.Background(), "execute", mustJSON(map[string]string{"id": preview.ID, "digest": preview.Digest})); err == nil {
		t.Fatal("a cleanup must not execute twice")
	}

	if _, err := s.handle(context.Background(), "undo", mustJSON(map[string]string{"manifest": `C:\Windows\manifest.json`})); err == nil {
		t.Fatal("undo outside the quarantine should be refused")
	}
	call(t, s, "undo", map[string]string{"manifest": result.Manifest})
	if _, err := os.Stat(filepath.Join(cache, "a.bin")); err != nil {
		t.Fatalf("undo did not restore: %v", err)
	}

	call(t, s, "scan", map[string]string{"root": root})
	again := call(t, s, "prepare", PrepareParams{Quarantine: []string{cache}}).(Preview)
	second := call(t, s, "execute", map[string]string{"id": again.ID, "digest": again.Digest}).(ExecuteResult)
	purged := call(t, s, "purge", map[string]string{"manifest": second.Manifest}).(PurgeResult)
	if purged.Purged != 1 {
		t.Fatalf("purge = %+v", purged)
	}
	if batches := call(t, s, "batches", nil).([]executor.BatchSummary); len(batches) != 2 {
		t.Fatalf("batches = %+v", batches)
	}
}

func TestPrepareValidatesTemplates(t *testing.T) {
	s := New(Options{SkipSystemChecks: true})
	preview := call(t, s, "prepare", PrepareParams{Actions: []ActionRequest{{Template: "wsl_journal_limit", Params: map[string]string{"distro": "Ubuntu", "size": "200M; reboot"}}}}).(Preview)
	if preview.Runnable || preview.Actions[0].Error == "" {
		t.Fatalf("invalid template accepted: %+v", preview)
	}
	good := call(t, s, "prepare", PrepareParams{Actions: []ActionRequest{{Template: "npm_cache_clean"}}}).(Preview)
	if !good.Runnable || len(good.Actions[0].Steps) == 0 {
		t.Fatalf("valid template rejected: %+v", good)
	}
}

func mustJSON(v any) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}
