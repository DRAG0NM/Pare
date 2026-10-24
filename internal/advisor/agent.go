package advisor

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Tool is something the model may call. Every tool is read-only.
type Tool struct {
	Name        string
	Description string
	Parameters  string // JSON schema
	Run         func(ctx context.Context, args json.RawMessage) (string, error)
}

// Event kinds sent to the UI while an investigation runs.
const (
	EventThinking = "thinking"
	EventTool     = "tool"
	EventResult   = "result"
	EventFinding  = "finding"
	EventMessage  = "message"
	EventError    = "error"
	EventDone     = "done"
)

type Event struct {
	Kind    string   `json:"kind"`
	Step    int      `json:"step"`
	Tool    string   `json:"tool,omitempty"`
	Args    string   `json:"args,omitempty"`
	Text    string   `json:"text,omitempty"`
	Finding *Finding `json:"finding,omitempty"`
	Tokens  int      `json:"tokens,omitempty"`
}

type chatter interface {
	Chat(ctx context.Context, messages []Message, tools []ToolSpec) (Message, Usage, error)
}

type Agent struct {
	Client   chatter
	Tools    []Tool
	MaxSteps int
	// ResultLimit bounds how much of one tool result goes back to the model.
	ResultLimit int
	Emit        func(Event)
}

const reportFindingSchema = `{
  "type": "object",
  "properties": {
    "title": {"type": "string", "description": "一句话标题，例如“罗技 G HUB 更新包残留 35.8 GB”"},
    "summary": {"type": "string", "description": "这是什么、在哪里"},
    "cause": {"type": "string", "description": "为什么会产生，根因是什么"},
    "paths": {"type": "array", "items": {"type": "string"}, "description": "涉及的完整路径"},
    "bytes": {"type": "integer", "description": "处理后预计可回收的字节数"},
    "risk": {"type": "string", "enum": ["safe", "rebuildable", "careful", "system"], "description": "safe=纯垃圾；rebuildable=缓存可重建；careful=可能含用户数据或仍在使用；system=系统或驱动组件"},
    "confidence": {"type": "string", "enum": ["high", "medium", "low"]},
    "regrowth": {"type": "string", "description": "清理后会不会再长出来，怎么防止"},
    "evidence": {"type": "array", "items": {"type": "string"}, "description": "支撑结论的具体事实，每条一句，来自工具结果"},
    "actions": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "kind": {"type": "string", "enum": ["quarantine", "template", "command", "config", "manual"]},
          "label": {"type": "string"},
          "explain": {"type": "string", "description": "做了会怎样、有什么风险"},
          "bytes": {"type": "integer"},
          "paths": {"type": "array", "items": {"type": "string"}, "description": "kind=quarantine 时要移入隔离区的路径"},
          "template": {"type": "string", "description": "kind=template 时的模板 ID"},
          "params": {"type": "object", "additionalProperties": {"type": "string"}},
          "command": {"type": "string", "description": "kind=command 时的完整命令，原样展示给用户"}
        },
        "required": ["kind", "label", "explain"]
      }
    }
  },
  "required": ["title", "summary", "cause", "bytes", "risk", "confidence", "evidence", "actions"]
}`

// Run drives the conversation until the model stops calling tools or MaxSteps is reached.
// It returns the findings reported along the way.
func (a Agent) Run(ctx context.Context, system, user string) ([]Finding, error) {
	emit := a.Emit
	if emit == nil {
		emit = func(Event) {}
	}
	maxSteps := a.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 40
	}
	limit := a.ResultLimit
	if limit <= 0 {
		limit = 12000
	}

	tools := map[string]Tool{}
	specs := make([]ToolSpec, 0, len(a.Tools)+1)
	for _, tool := range append(a.Tools, Tool{Name: "report_finding", Description: "报告一条发现。每个独立问题调用一次；没有证据的猜测不要报告。", Parameters: reportFindingSchema}) {
		tools[tool.Name] = tool
		var spec ToolSpec
		spec.Type = "function"
		spec.Function.Name = tool.Name
		spec.Function.Description = tool.Description
		spec.Function.Parameters = json.RawMessage(tool.Parameters)
		specs = append(specs, spec)
	}

	messages := []Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
	var findings []Finding
	tokens := 0
	for step := 1; step <= maxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return findings, err
		}
		emit(Event{Kind: EventThinking, Step: step, Tokens: tokens})
		reply, usage, err := a.Client.Chat(ctx, messages, specs)
		if err != nil {
			emit(Event{Kind: EventError, Step: step, Text: err.Error()})
			return findings, err
		}
		tokens += usage.PromptTokens + usage.CompletionTokens
		messages = append(messages, reply)

		if len(reply.ToolCalls) == 0 {
			emit(Event{Kind: EventMessage, Step: step, Text: reply.Content, Tokens: tokens})
			emit(Event{Kind: EventDone, Step: step, Tokens: tokens})
			return findings, nil
		}
		if reply.Content != "" {
			emit(Event{Kind: EventMessage, Step: step, Text: reply.Content})
		}
		for _, call := range reply.ToolCalls {
			name, args := call.Function.Name, call.Function.Arguments
			var result string
			if name == "report_finding" {
				var finding Finding
				if err := json.Unmarshal([]byte(args), &finding); err != nil {
					result = "参数不是合法的 JSON：" + err.Error()
				} else {
					finding.normalize()
					finding.ID = "f" + strconv.Itoa(len(findings)+1)
					findings = append(findings, finding)
					emit(Event{Kind: EventFinding, Step: step, Finding: &finding})
					result = "已记录 " + finding.ID
				}
			} else {
				emit(Event{Kind: EventTool, Step: step, Tool: name, Args: args})
				result = a.call(ctx, tools, name, args)
				emit(Event{Kind: EventResult, Step: step, Tool: name, Text: clip(result, 2000)})
			}
			messages = append(messages, Message{Role: "tool", ToolCallID: call.ID, Content: clip(result, limit)})
		}
	}
	emit(Event{Kind: EventMessage, Text: fmt.Sprintf("已达到最多 %d 步，调查提前结束。", maxSteps), Tokens: tokens})
	emit(Event{Kind: EventDone, Tokens: tokens})
	return findings, nil
}

func (a Agent) call(ctx context.Context, tools map[string]Tool, name, args string) string {
	tool, ok := tools[name]
	if !ok || tool.Run == nil {
		return "没有这个工具：" + name
	}
	if args == "" {
		args = "{}"
	}
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	result, err := tool.Run(callCtx, json.RawMessage(args))
	if err != nil {
		return "工具出错：" + err.Error()
	}
	return result
}
