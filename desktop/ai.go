package main

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/DRAG0NM/Pare/internal/advisor"
)

// consentTimeout treats an unanswered consent request as a refusal.
const consentTimeout = 5 * time.Minute

// investigation holds the AI run state. It has its own lock so a running investigation
// never blocks scan and tree calls.
type investigation struct {
	mu       sync.Mutex
	cancel   context.CancelFunc
	consents map[string]chan bool
	nextID   int
	runs     int
}

type AISettingsView struct {
	Provider  string             `json:"provider"`
	BaseURL   string             `json:"baseUrl"`
	Model     string             `json:"model"`
	HasKey    bool               `json:"hasKey"`
	Providers []advisor.Provider `json:"providers"`
}

type AISettingsInput struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"baseUrl"`
	Model    string `json:"model"`
	// APIKey replaces the stored key when not empty; ClearKey removes it.
	APIKey   string `json:"apiKey"`
	ClearKey bool   `json:"clearKey"`
}

func loadAISettings() (advisor.Settings, string, error) {
	path, err := advisor.DefaultPath()
	if err != nil {
		return advisor.Settings{}, "", err
	}
	settings, err := advisor.Load(path)
	return settings, path, err
}

// GetAISettings never returns the key itself, only whether one is stored.
func (a *App) GetAISettings() (AISettingsView, error) {
	settings, _, err := loadAISettings()
	return AISettingsView{
		Provider:  settings.Provider,
		BaseURL:   settings.BaseURL,
		Model:     settings.Model,
		HasKey:    settings.APIKey != "",
		Providers: advisor.Providers,
	}, err
}

func (a *App) SaveAISettings(input AISettingsInput) error {
	settings, path, err := loadAISettings()
	if err != nil && path == "" {
		return err
	}
	settings.Provider = input.Provider
	settings.BaseURL = strings.TrimSpace(input.BaseURL)
	settings.Model = strings.TrimSpace(input.Model)
	if key := strings.TrimSpace(input.APIKey); key != "" {
		settings.APIKey = key
	}
	if input.ClearKey {
		settings.APIKey = ""
	}
	return advisor.Save(path, settings)
}

// TestAI sends one tiny request so the user can check URL, model and key.
func (a *App) TestAI() (string, error) {
	settings, _, err := loadAISettings()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(a.ctx, time.Minute)
	defer cancel()
	reply, _, err := advisor.NewClient(settings).Chat(ctx, []advisor.Message{{Role: "user", Content: "只回复四个字：连接正常"}}, nil)
	if err != nil {
		return "", err
	}
	return reply.Content, nil
}

type InvestigationResult struct {
	Findings  []advisor.Finding `json:"findings"`
	Error     string            `json:"error"`
	Cancelled bool              `json:"cancelled"`
}

// StartInvestigation runs the AI over the latest scan with the step budget of depth
// ("standard", "deep", "thorough"). With continueRun, the model is told what earlier runs
// already reported and its findings are added to them; otherwise they replace them.
// Steps arrive as "ai:event" events, consent requests as "ai:consent", and the end as one
// "ai:finished" event carrying all current findings.
func (a *App) StartInvestigation(scope, note, depth string, continueRun bool) error {
	settings, _, err := loadAISettings()
	if err != nil {
		return err
	}
	if !settings.Configured() {
		return advisor.ErrNotConfigured
	}
	a.mu.Lock()
	report, built, root, volumeUsed := a.report, a.tree, a.root, a.volumeUsed
	var reported []advisor.Finding
	if continueRun {
		reported = append(reported, a.findings...)
	}
	a.mu.Unlock()
	if built == nil {
		return errors.New("请先完成一次扫描")
	}
	if scope == "" {
		scope = root
	}

	a.ai.mu.Lock()
	if a.ai.cancel != nil {
		a.ai.mu.Unlock()
		return errors.New("已有调查在进行")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.ai.cancel = cancel
	a.ai.consents = map[string]chan bool{}
	a.ai.runs++
	prefix := ""
	if continueRun {
		prefix = "r" + strconv.Itoa(a.ai.runs) + "-"
	}
	a.ai.mu.Unlock()

	steps := advisor.StepsFor(depth)
	env := &advisor.Environment{Root: root, Report: report, Tree: built, VolumeUsed: volumeUsed, Consent: a.askConsent}
	agent := advisor.Agent{
		Client:   advisor.NewClient(settings),
		Tools:    env.Tools(),
		MaxSteps: steps,
		IDPrefix: prefix,
		Emit:     func(e advisor.Event) { runtime.EventsEmit(a.ctx, "ai:event", e) },
	}
	go func() {
		findings, runErr := agent.Run(ctx, advisor.SystemPrompt(), advisor.UserPrompt(scope, note, steps, reported))
		result := InvestigationResult{Cancelled: errors.Is(runErr, context.Canceled)}
		if runErr != nil && !result.Cancelled {
			result.Error = runErr.Error()
		}
		// A continued run adds to the earlier findings (its IDs carry a prefix); a fresh
		// run replaces them, matching the UI, which clears its list when one starts.
		a.mu.Lock()
		if continueRun {
			a.findings = append(append([]advisor.Finding{}, reported...), findings...)
		} else {
			a.findings = findings
		}
		result.Findings = append([]advisor.Finding{}, a.findings...)
		a.mu.Unlock()
		a.ai.mu.Lock()
		a.ai.cancel = nil
		for _, answer := range a.ai.consents {
			close(answer)
		}
		a.ai.consents = nil
		a.ai.mu.Unlock()
		cancel()
		runtime.EventsEmit(a.ctx, "ai:finished", result)
	}()
	return nil
}

func (a *App) StopInvestigation() {
	a.ai.mu.Lock()
	defer a.ai.mu.Unlock()
	if a.ai.cancel != nil {
		a.ai.cancel()
	}
}

type ConsentRequest struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// askConsent blocks the tool call until the user answers in the UI. No answer, a closed
// dialog or a cancelled investigation all mean "no".
func (a *App) askConsent(ctx context.Context, path, reason string) bool {
	a.ai.mu.Lock()
	if a.ai.consents == nil {
		a.ai.mu.Unlock()
		return false
	}
	a.ai.nextID++
	id := "c" + strconv.Itoa(a.ai.nextID)
	answer := make(chan bool, 1)
	a.ai.consents[id] = answer
	a.ai.mu.Unlock()

	runtime.EventsEmit(a.ctx, "ai:consent", ConsentRequest{ID: id, Path: path, Reason: reason})
	timer := time.NewTimer(consentTimeout)
	defer timer.Stop()
	select {
	case allowed, ok := <-answer:
		return ok && allowed
	case <-ctx.Done():
	case <-timer.C:
	}
	a.ai.mu.Lock()
	delete(a.ai.consents, id)
	a.ai.mu.Unlock()
	return false
}

func (a *App) AnswerConsent(id string, allow bool) {
	a.ai.mu.Lock()
	answer := a.ai.consents[id]
	delete(a.ai.consents, id)
	a.ai.mu.Unlock()
	if answer != nil {
		answer <- allow
	}
}

// Findings returns the latest investigation's findings, for G3 to act on.
func (a *App) Findings() []advisor.Finding {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]advisor.Finding{}, a.findings...)
}
