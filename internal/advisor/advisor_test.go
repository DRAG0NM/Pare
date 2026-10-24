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
