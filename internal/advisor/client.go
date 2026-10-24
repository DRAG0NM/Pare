package advisor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolSpec struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// Client talks to an OpenAI-compatible /chat/completions endpoint.
type Client struct {
	BaseURL string
	Model   string
	APIKey  string
	HTTP    *http.Client
}

func NewClient(settings Settings) Client {
	return Client{
		BaseURL: strings.TrimRight(settings.BaseURL, "/"),
		Model:   settings.Model,
		APIKey:  settings.APIKey,
		// The default transport honours HTTP(S)_PROXY. One step of an investigation can
		// carry a long context, so allow slow responses.
		HTTP: &http.Client{Timeout: 3 * time.Minute},
	}
}

var ErrNotConfigured = errors.New("AI 尚未配置：请在设置里填写接口地址、模型和 API Key")

// Chat sends one completion request and returns the assistant message.
func (c Client) Chat(ctx context.Context, messages []Message, tools []ToolSpec) (Message, Usage, error) {
	if c.BaseURL == "" || c.Model == "" || c.APIKey == "" {
		return Message{}, Usage{}, ErrNotConfigured
	}
	body := map[string]any{
		"model":       c.Model,
		"messages":    messages,
		"temperature": 0.2,
	}
	if len(tools) != 0 {
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return Message{}, Usage{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return Message{}, Usage{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.APIKey)

	response, err := c.HTTP.Do(request)
	if err != nil {
		return Message{}, Usage{}, fmt.Errorf("请求 %s 失败：%w", c.BaseURL, err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return Message{}, Usage{}, err
	}
	if response.StatusCode != http.StatusOK {
		return Message{}, Usage{}, fmt.Errorf("接口返回 %d：%s", response.StatusCode, clip(string(data), 400))
	}
	var parsed struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return Message{}, Usage{}, fmt.Errorf("无法解析接口返回：%w；内容：%s", err, clip(string(data), 200))
	}
	if len(parsed.Choices) == 0 {
		return Message{}, Usage{}, fmt.Errorf("接口没有返回结果：%s", clip(string(data), 200))
	}
	message := parsed.Choices[0].Message
	message.Role = "assistant"
	return message, parsed.Usage, nil
}

func clip(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}
