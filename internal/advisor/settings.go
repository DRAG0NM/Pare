// Package advisor asks an LLM for explanations and suggestions about scanned folders.
// It never decides what gets executed: its output is advice that the UI shows next to
// the rule evidence, and every executed item still passes the executor's checks.
package advisor

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Provider struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	BaseURL      string `json:"baseUrl"`
	DefaultModel string `json:"defaultModel"`
}

// Providers are presets for OpenAI-compatible endpoints. Model names change often, so
// the UI lets the user edit both the URL and the model.
var Providers = []Provider{
	{ID: "deepseek", Name: "DeepSeek", BaseURL: "https://api.deepseek.com", DefaultModel: "deepseek-chat"},
	{ID: "qwen", Name: "通义千问", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", DefaultModel: "qwen-plus"},
	{ID: "moonshot", Name: "Kimi", BaseURL: "https://api.moonshot.cn/v1", DefaultModel: "moonshot-v1-8k"},
	{ID: "zhipu", Name: "智谱", BaseURL: "https://open.bigmodel.cn/api/paas/v4", DefaultModel: "glm-4-flash"},
	{ID: "openai", Name: "OpenAI", BaseURL: "https://api.openai.com/v1", DefaultModel: "gpt-4o-mini"},
	{ID: "custom", Name: "自定义", BaseURL: "", DefaultModel: ""},
}

type Settings struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"baseUrl"`
	Model    string `json:"model"`
	// APIKey is kept in memory only; on disk it is stored encrypted for the current
	// Windows user (DPAPI).
	APIKey string `json:"-"`
}

type storedSettings struct {
	Settings
	EncryptedKey string `json:"encryptedKey"`
}

// Configured reports whether enough is set to make a request.
func (s Settings) Configured() bool {
	return s.BaseURL != "" && s.Model != "" && s.APIKey != ""
}

var ErrNoProtection = errors.New("secure key storage is only available on Windows")

// DefaultPath is %APPDATA%\Pare\advisor.json.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Pare", "advisor.json"), nil
}

// Load returns zero settings with DeepSeek preselected when the file does not exist.
func Load(path string) (Settings, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Settings{Provider: Providers[0].ID, BaseURL: Providers[0].BaseURL, Model: Providers[0].DefaultModel}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	var stored storedSettings
	if err := json.Unmarshal(data, &stored); err != nil {
		return Settings{}, fmt.Errorf("parse %s: %w", path, err)
	}
	settings := stored.Settings
	if stored.EncryptedKey != "" {
		sealed, err := base64.StdEncoding.DecodeString(stored.EncryptedKey)
		if err != nil {
			return settings, fmt.Errorf("decode API key: %w", err)
		}
		key, err := unprotect(sealed)
		if err != nil {
			return settings, fmt.Errorf("decrypt API key (saved by another Windows user?): %w", err)
		}
		settings.APIKey = string(key)
	}
	return settings, nil
}

// Save writes settings with the API key encrypted for the current Windows user.
func Save(path string, settings Settings) error {
	stored := storedSettings{Settings: settings}
	if settings.APIKey != "" {
		sealed, err := protect([]byte(settings.APIKey))
		if err != nil {
			return err
		}
		stored.EncryptedKey = base64.StdEncoding.EncodeToString(sealed)
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
