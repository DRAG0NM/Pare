package advisor

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
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
	nudged, wrapping := false, false
	samples := map[string]int{}
	for step := 1; step <= maxSteps+wrapUpSteps; step++ {
		if err := ctx.Err(); err != nil {
			return findings, err
		}
		if step == maxSteps+1 {
			// Out of investigation steps: let the model report what it already knows
			// instead of losing the whole run.
			wrapping = true
			messages = append(messages, Message{Role: "user", Content: wrapUpPrompt})
			emit(Event{Kind: EventMessage, Step: step, Text: fmt.Sprintf("已用完 %d 步调查，正在整理已确认的问题…", maxSteps)})
		}
		available := specs
		if wrapping {
			available = specs[len(specs)-1:]
		}
		emit(Event{Kind: EventThinking, Step: step, Tokens: tokens})
		reply, usage, err := a.Client.Chat(ctx, messages, available)
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
			} else if wrapping {
				result = "调查步数已用完，现在只能调用 report_finding。"
			} else {
				emit(Event{Kind: EventTool, Step: step, Tool: name, Args: args})
				result = a.call(ctx, tools, name, args) + samplingNote(samples, name, args)
				emit(Event{Kind: EventResult, Step: step, Tool: name, Text: clip(result, 2000)})
			}
			messages = append(messages, Message{Role: "tool", ToolCallID: call.ID, Content: clip(result, limit)})
		}
		if !nudged && !wrapping && len(findings) == 0 && step >= maxSteps*6/10 {
			nudged = true
			messages = append(messages, Message{Role: "user", Content: fmt.Sprintf(nudgePrompt, step, maxSteps)})
		}
	}
	emit(Event{Kind: EventMessage, Text: "调查结束（步数用完）。", Tokens: tokens})
	emit(Event{Kind: EventDone, Tokens: tokens})
	return findings, nil
}

// wrapUpSteps are extra rounds after MaxSteps in which only report_finding is offered.
const wrapUpSteps = 6

const nudgePrompt = `（系统提醒）你已经用了 %d/%d 步，还没有报告任何发现。请先用 report_finding 报告已经有证据的问题，再决定是否继续调查；剩余步数请优先用在占用最大的位置。`

const wrapUpPrompt = `（系统提醒）调查步数已经用完。请根据目前已经掌握的证据，用 report_finding 逐条报告所有已确认或基本确认的问题（证据不足的把 confidence 设为 low），全部报告完后用一段话总结。不要再调用其他工具。`

// samplingNote warns the model after it reads many files of one directory one by one.
func samplingNote(samples map[string]int, name, args string) string {
	if name != "file_head" && name != "read_text" {
		return ""
	}
	var parsed struct {
		Path string `json:"path"`
	}
	if json.Unmarshal([]byte(args), &parsed) != nil || parsed.Path == "" {
		return ""
	}
	key := filepath.Dir(parsed.Path)
	samples[key]++
	if samples[key] <= 4 {
		return ""
	}
	return fmt.Sprintf("\n（提示：你已经在 %s 里逐个读了 %d 个文件，样本已经足够。请根据已有证据下结论，或换到占用更大的位置。）", key, samples[key])
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
