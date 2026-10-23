package main

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"time"

	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/plan"
	"github.com/DRAG0NM/Pare/internal/probes"
	"github.com/DRAG0NM/Pare/internal/rules"
)

// planTTL matches the CLI default; G3 executes only plans that are still valid.
const planTTL = 24 * time.Hour

type CandidateItem struct {
	Path           string           `json:"path"`
	Name           string           `json:"name"`
	Directory      string           `json:"directory"`
	LogicalSize    uint64           `json:"logicalSize"`
	AllocationSize uint64           `json:"allocationSize"`
	LastWriteTime  time.Time        `json:"lastWriteTime"`
	Risk           string           `json:"risk"`
	Executable     bool             `json:"executable"`
	Evidence       []model.Evidence `json:"evidence"`
}

type CandidateGroup struct {
	RuleID               string          `json:"ruleId"`
	Description          string          `json:"description"`
	Risk                 string          `json:"risk"`
	MinAgeDays           int             `json:"minAgeDays"`
	Candidates           int             `json:"candidates"`
	Executable           int             `json:"executable"`
	ExecutableAllocation uint64          `json:"executableAllocation"`
	Items                []CandidateItem `json:"items"`
}

type CandidateView struct {
	Digest       string           `json:"digest"`
	CreatedAt    time.Time        `json:"createdAt"`
	ExpiresAt    time.Time        `json:"expiresAt"`
	RulesVersion string           `json:"rulesVersion"`
	Groups       []CandidateGroup `json:"groups"`
	// Rules lists every built-in rule, including those without matches, so the UI can
	// say what was looked for.
	Rules []CandidateGroup `json:"rules"`
}

// BuildCandidates matches the built-in rules against the latest scan and runs the live
// probes on items whose static evidence passes. The plan is kept for G3 to execute and
// reused on later calls unless refresh asks for fresh probes.
func (a *App) BuildCandidates(refresh bool) (*CandidateView, error) {
	a.mu.Lock()
	report, scanned, cached := a.report, a.tree != nil, a.plan
	a.mu.Unlock()
	if !scanned {
		return nil, errors.New("no scan result yet")
	}
	if cached != nil && !refresh && time.Now().Before(cached.ExpiresAt) {
		return candidateView(*cached, rules.Builtin()), nil
	}

	built, err := plan.Build(context.Background(), report, rules.Builtin(), probes.NewWindowsProber(), time.Now(), planTTL)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.plan = &built
	a.mu.Unlock()
	return candidateView(built, rules.Builtin()), nil
}

func candidateView(p model.Plan, ruleSet []rules.Rule) *CandidateView {
	view := &CandidateView{Digest: p.Digest, CreatedAt: p.CreatedAt, ExpiresAt: p.ExpiresAt, RulesVersion: p.RulesVersion, Groups: []CandidateGroup{}}
	groups := map[string]*CandidateGroup{}
	for _, rule := range ruleSet {
		group := CandidateGroup{RuleID: rule.ID, Description: rule.Description, Risk: rule.Risk, MinAgeDays: int(rule.MinAge / (24 * time.Hour)), Items: []CandidateItem{}}
		view.Rules = append(view.Rules, group)
	}
	for _, item := range p.Items {
		group := groups[item.RuleID]
		if group == nil {
			group = &CandidateGroup{RuleID: item.RuleID, Risk: item.Risk, Items: []CandidateItem{}}
			for _, rule := range view.Rules {
				if rule.RuleID == item.RuleID {
					group.Description, group.MinAgeDays = rule.Description, rule.MinAgeDays
				}
			}
			groups[item.RuleID] = group
		}
		group.Candidates++
		if item.Executable {
			group.Executable++
			group.ExecutableAllocation += uint64(item.AllocationSize)
		}
		group.Items = append(group.Items, CandidateItem{
			Path:           item.Path,
			Name:           filepath.Base(item.Path),
			Directory:      filepath.Dir(item.Path),
			LogicalSize:    uint64(item.LogicalSize),
			AllocationSize: uint64(item.AllocationSize),
			LastWriteTime:  item.LastWriteTime,
			Risk:           item.Risk,
			Executable:     item.Executable,
			Evidence:       item.Evidence,
		})
	}
	for _, group := range groups {
		sort.SliceStable(group.Items, func(i, j int) bool {
			a, b := group.Items[i], group.Items[j]
			if a.Executable != b.Executable {
				return a.Executable
			}
			return a.AllocationSize > b.AllocationSize
		})
		view.Groups = append(view.Groups, *group)
	}
	sort.Slice(view.Groups, func(i, j int) bool { return view.Groups[i].ExecutableAllocation > view.Groups[j].ExecutableAllocation })
	return view
}
