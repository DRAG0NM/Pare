package advisor

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type scriptedModel struct {
	replies []Message
	seen    [][]Message
}

func (m *scriptedModel) Chat(_ context.Context, messages []Message, _ []ToolSpec) (Message, Usage, error) {
	m.seen = append(m.seen, append([]Message(nil), messages...))
	reply := m.replies[0]
	m.replies = m.replies[1:]
	return reply, Usage{PromptTokens: 10, CompletionTokens: 5}, nil
}

func toolCall(id, name, args string) ToolCall {
	var call ToolCall
	call.ID, call.Type = id, "function"
	call.Function.Name, call.Function.Arguments = name, args
	return call
}

func TestAgentRunsToolsAndCollectsFindings(t *testing.T) {
	model := &scriptedModel{replies: []Message{
		{Role: "assistant", ToolCalls: []ToolCall{toolCall("c1", "echo", `{"text":"hi"}`), toolCall("c2", "missing", `{}`)}},
		{Role: "assistant", ToolCalls: []ToolCall{toolCall("c3", "report_finding", `{"title":"T","summary":"S","cause":"C","bytes":42,"risk":"weird","confidence":"high","evidence":["e"],"actions":[{"kind":"quarantine","label":"L","explain":"X","paths":["C:\\a"]},{"kind":"format-disk","label":"bad","explain":"no"}]}`)}},
		{Role: "assistant", Content: "完成"},
	}}
	var events []Event
	agent := Agent{
		Client: model,
		Tools: []Tool{{Name: "echo", Parameters: `{"type":"object"}`, Run: func(_ context.Context, args json.RawMessage) (string, error) {
			return "echo:" + string(args), nil
		}}},
		Emit: func(e Event) { events = append(events, e) },
	}

	findings, err := agent.Run(context.Background(), "sys", "user")

	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].ID != "f1" || findings[0].Bytes != 42 {
		t.Fatalf("findings = %+v", findings)
	}
	if findings[0].Risk != "careful" || len(findings[0].Actions) != 1 {
		t.Errorf("normalize: risk %q, actions %+v; want unknown risk -> careful and unknown action kinds dropped", findings[0].Risk, findings[0].Actions)
	}
	second := model.seen[1]
	if got := second[len(second)-2].Content; got != `echo:{"text":"hi"}` {
		t.Errorf("tool result sent back = %q", got)
	}
	if got := second[len(second)-1].Content; !strings.Contains(got, "没有这个工具") {
		t.Errorf("unknown tool result = %q", got)
	}
	if last := events[len(events)-1]; last.Kind != EventDone || last.Tokens != 45 {
		t.Errorf("last event = %+v", last)
	}
}

// specModel records which tools each request offered.
type specModel struct {
	scriptedModel
	offered [][]string
}

func (m *specModel) Chat(ctx context.Context, messages []Message, tools []ToolSpec) (Message, Usage, error) {
	var names []string
	for _, tool := range tools {
		names = append(names, tool.Function.Name)
	}
	m.offered = append(m.offered, names)
	return m.scriptedModel.Chat(ctx, messages, tools)
}

func TestAgentNudgesSamplingAndWrapsUpWhenStepsRunOut(t *testing.T) {
	head := func(id, file string) ToolCall {
		return toolCall(id, "file_head", `{"path":"C:/data/`+file+`"}`)
	}
	model := &specModel{scriptedModel: scriptedModel{replies: []Message{
		{Role: "assistant", ToolCalls: []ToolCall{head("1", "a"), head("2", "b"), head("3", "c")}},
		{Role: "assistant", ToolCalls: []ToolCall{head("4", "d"), head("5", "e")}},
		{Role: "assistant", ToolCalls: []ToolCall{head("6", "f")}},
		// Wrap-up phase: a stray investigation call is refused, a report is accepted.
		{Role: "assistant", ToolCalls: []ToolCall{head("7", "g"), toolCall("8", "report_finding", `{"title":"T","summary":"S","cause":"C","bytes":1,"risk":"safe","confidence":"low","evidence":[],"actions":[]}`)}},
		{Role: "assistant", Content: "总结"},
	}}}
	agent := Agent{
		Client:   model,
		MaxSteps: 3,
		Tools: []Tool{{Name: "file_head", Parameters: `{"type":"object"}`, Run: func(context.Context, json.RawMessage) (string, error) {
			return "bytes", nil
		}}},
	}

	findings, err := agent.Run(context.Background(), "sys", "user")

	if err != nil || len(findings) != 1 {
		t.Fatalf("findings = %+v, %v; want the wrap-up report kept", findings, err)
	}
	second := model.seen[2]
	var fifth, nudge string
	for _, m := range second {
		if m.ToolCallID == "5" {
			fifth = m.Content
		}
		if m.Role == "user" && strings.Contains(m.Content, "还没有报告任何发现") {
			nudge = m.Content
		}
	}
	if !strings.Contains(fifth, "样本已经足够") {
		t.Errorf("fifth read of one directory = %q, want a sampling note", fifth)
	}
	if nudge == "" {
		t.Error("no nudge after 60% of the steps without findings")
	}
	if got := model.offered[3]; len(got) != 1 || got[0] != "report_finding" {
		t.Errorf("tools offered in wrap-up = %v, want only report_finding", got)
	}
	wrap := model.seen[4]
	if refused := wrap[len(wrap)-2].Content; !strings.Contains(refused, "只能调用 report_finding") {
		t.Errorf("investigation call during wrap-up got %q", refused)
	}
}

func TestCompactShortensOnlyOldToolResults(t *testing.T) {
	long := strings.Repeat("x", 2000)
	messages := []Message{{Role: "system", Content: long}, {Role: "tool", Content: long}, {Role: "assistant", Content: long}, {Role: "tool", Content: long}}

	mark := compact(messages, 0, 3)

	if mark != 3 || !strings.Contains(messages[1].Content, "已压缩") || len([]rune(messages[1].Content)) > 700 {
		t.Errorf("old tool result = %d runes, mark %d", len([]rune(messages[1].Content)), mark)
	}
	if messages[0].Content != long || messages[2].Content != long || messages[3].Content != long {
		t.Error("compact touched system, assistant or recent tool messages")
	}
}

func TestClientSendsToolsAndParsesToolCalls(t *testing.T) {
	var request map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer sk-test" {
			http.Error(w, "bad request "+r.URL.Path, http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &request)
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"x","type":"function","function":{"name":"scan_overview","arguments":"{}"}}]}}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`)
	}))
	defer server.Close()

	client := NewClient(Settings{BaseURL: server.URL + "/v1/", Model: "m", APIKey: "sk-test"})
	var spec ToolSpec
	spec.Type, spec.Function.Name, spec.Function.Parameters = "function", "scan_overview", json.RawMessage(`{"type":"object"}`)
	reply, usage, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, []ToolSpec{spec})

	if err != nil {
		t.Fatal(err)
	}
	if len(reply.ToolCalls) != 1 || reply.ToolCalls[0].Function.Name != "scan_overview" || usage.PromptTokens != 3 {
		t.Fatalf("reply = %+v usage = %+v", reply, usage)
	}
	if request["model"] != "m" || request["tool_choice"] != "auto" || len(request["tools"].([]any)) != 1 {
		t.Fatalf("request = %v", request)
	}

	_, _, err = NewClient(Settings{BaseURL: server.URL, Model: "m", APIKey: "sk-wrong"}).Chat(context.Background(), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "sk-wrong") {
		t.Fatalf("auth error = %v; want status shown and key not leaked", err)
	}
}

func TestIsSensitive(t *testing.T) {
	for path, want := range map[string]bool{
		`C:\Users\me\.ssh\config`:        true,
		`C:\Users\me\project\.env`:       true,
		`C:\Users\me\project\.env.local`: true,
		`D:\certs\server.PEM`:            true,
		`C:\Users\me\AppData\Local\Google\Chrome\User Data\Default\Login Data`: true,
		`C:\Users\me\AppData\Roaming\Microsoft\Credentials\ABC`:                true,
		`C:\Users\me\.wslconfig`:                         false,
		`C:\Windows\SystemTemp\fff2-e083`:                false,
		`C:\Users\me\AppData\Local\Temp\environment.log`: false,
	} {
		if got := IsSensitive(path); got != want {
			t.Errorf("IsSensitive(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestSettingsEncryptKeyOnDisk(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("DPAPI is Windows only")
	}
	path := filepath.Join(t.TempDir(), "advisor.json")
	if err := Save(path, Settings{Provider: "deepseek", BaseURL: "https://api.deepseek.com", Model: "deepseek-chat", APIKey: "sk-secret-123"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "sk-secret-123") {
		t.Fatalf("API key stored in plain text: %s", raw)
	}
	loaded, err := Load(path)
	if err != nil || loaded.APIKey != "sk-secret-123" || !loaded.Configured() {
		t.Fatalf("Load = %+v, %v", loaded, err)
	}
	if fresh, err := Load(filepath.Join(t.TempDir(), "none.json")); err != nil || fresh.Provider != "deepseek" || fresh.Configured() {
		t.Fatalf("defaults = %+v, %v", fresh, err)
	}
}
