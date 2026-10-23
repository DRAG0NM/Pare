// Package plan expands rule matches over a scan into a per-file, reviewable plan.
package plan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/probes"
	"github.com/DRAG0NM/Pare/internal/rules"
)

const (
	KindPathRule     = "path_rule"
	KindAttributes   = "attributes"
	KindFileIdentity = "file_identity"
	KindAge          = "age"
)

const ActionQuarantine = "quarantine"

const fileAttributeSystem = 0x4

// requiredEvidence must all be present and pass before an item is executable.
var requiredEvidence = []string{KindPathRule, KindAttributes, KindFileIdentity, KindAge, probes.KindUnchanged, probes.KindUnlocked}

var (
	ErrExpired  = errors.New("plan has expired")
	ErrTampered = errors.New("plan digest does not match its contents")
)

// Build matches rules against every scanned file. Live probes run only for items whose
// static evidence already passes, so unrelated files are never opened.
func Build(ctx context.Context, scan model.ScanReport, ruleSet []rules.Rule, prober probes.Prober, now time.Time, ttl time.Duration) (model.Plan, error) {
	now = now.UTC().Truncate(time.Second)
	paths := map[string]int{}
	for _, node := range scan.Nodes {
		if node.Kind == "file" && node.FileID != "" {
			paths[node.FileID]++
		}
	}

	items := []model.PlanItem{}
	for _, node := range scan.Nodes {
		if err := ctx.Err(); err != nil {
			return model.Plan{}, err
		}
		if node.Kind != "file" {
			continue
		}
		var matched []rules.Rule
		for _, rule := range ruleSet {
			if rule.Matches(node.Path) {
				matched = append(matched, rule)
			}
		}
		if len(matched) == 0 {
			continue
		}
		items = append(items, buildItem(ctx, node, matched, paths[node.FileID], prober, now))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Path != items[j].Path {
			return items[i].Path < items[j].Path
		}
		return items[i].RuleID < items[j].RuleID
	})

	result := model.Plan{
		Schema:       model.PlanSchema,
		CreatedAt:    now,
		ExpiresAt:    now.Add(ttl),
		Volume:       scan.Volume,
		Scan:         model.PlanSource{StartedAt: scan.StartedAt, CompletedAt: scan.CompletedAt, Complete: scan.Complete},
		RulesVersion: rules.Version,
		Items:        items,
		Totals:       totals(items),
	}
	digest, err := Digest(result)
	if err != nil {
		return model.Plan{}, err
	}
	result.Digest = digest
	return result, nil
}

func buildItem(ctx context.Context, node model.Node, matched []rules.Rule, pathsForID int, prober probes.Prober, now time.Time) model.PlanItem {
	ids := make([]string, len(matched))
	risk := ""
	for i, rule := range matched {
		ids[i] = rule.ID
		if rule.Risk > risk {
			risk = rule.Risk
		}
	}
	item := model.PlanItem{
		Path:           node.Path,
		FileID:         node.FileID,
		LogicalSize:    node.LogicalSize,
		AllocationSize: node.AllocationSize,
		Attributes:     node.Attributes,
		LastWriteTime:  node.LastWriteTime,
		RuleID:         strings.Join(ids, ","),
		RuleVersion:    matched[0].Version,
		Risk:           risk,
		Action:         ActionQuarantine,
	}

	if len(matched) == 1 {
		item.Evidence = append(item.Evidence, model.Evidence{Kind: KindPathRule, Status: model.EvidencePass, Detail: matched[0].Description})
	} else {
		item.Evidence = append(item.Evidence, model.Evidence{Kind: KindPathRule, Status: model.EvidenceFail, Detail: "conflicting rules: " + item.RuleID})
	}

	if node.Attributes&fileAttributeSystem != 0 {
		item.Evidence = append(item.Evidence, model.Evidence{Kind: KindAttributes, Status: model.EvidenceFail, Detail: "system file"})
	} else {
		item.Evidence = append(item.Evidence, model.Evidence{Kind: KindAttributes, Status: model.EvidencePass, Detail: "not a system file"})
	}

	switch {
	case node.FileID == "":
		item.Evidence = append(item.Evidence, model.Evidence{Kind: KindFileIdentity, Status: model.EvidenceUnknown, Detail: "scan has no file ID"})
	case pathsForID > 1:
		item.Evidence = append(item.Evidence, model.Evidence{Kind: KindFileIdentity, Status: model.EvidenceFail, Detail: fmt.Sprintf("file ID appears at %d paths (hard link)", pathsForID)})
	default:
		item.Evidence = append(item.Evidence, model.Evidence{Kind: KindFileIdentity, Status: model.EvidencePass, Detail: "single path for file ID"})
	}

	item.Evidence = append(item.Evidence, ageEvidence(node.LastWriteTime, matched[0].MinAge, now))

	if allPass(item.Evidence) {
		item.Evidence = append(item.Evidence, prober.Probe(ctx, node)...)
	}
	item.Executable = executable(item.Evidence)
	return item
}

func ageEvidence(lastWrite time.Time, minAge time.Duration, now time.Time) model.Evidence {
	if lastWrite.IsZero() {
		return model.Evidence{Kind: KindAge, Status: model.EvidenceUnknown, Detail: "no last write time"}
	}
	age := now.Sub(lastWrite)
	if age < 0 {
		return model.Evidence{Kind: KindAge, Status: model.EvidenceFail, Detail: "last write time is in the future"}
	}
	days := int(age / (24 * time.Hour))
	minDays := int(minAge / (24 * time.Hour))
	if age < minAge {
		return model.Evidence{Kind: KindAge, Status: model.EvidenceFail, Detail: fmt.Sprintf("written %d days ago, rule requires %d", days, minDays)}
	}
	return model.Evidence{Kind: KindAge, Status: model.EvidencePass, Detail: fmt.Sprintf("written %d days ago, rule requires %d", days, minDays)}
}

func allPass(evidence []model.Evidence) bool {
	for _, entry := range evidence {
		if entry.Status != model.EvidencePass {
			return false
		}
	}
	return true
}

func executable(evidence []model.Evidence) bool {
	if !allPass(evidence) {
		return false
	}
	seen := map[string]bool{}
	for _, entry := range evidence {
		seen[entry.Kind] = true
	}
	for _, kind := range requiredEvidence {
		if !seen[kind] {
			return false
		}
	}
	return true
}

// totals counts executable bytes once per file ID.
func totals(items []model.PlanItem) model.PlanTotals {
	result := model.PlanTotals{Candidates: len(items)}
	counted := map[string]bool{}
	for _, item := range items {
		if !item.Executable {
			continue
		}
		result.Executable++
		if counted[item.FileID] {
			continue
		}
		counted[item.FileID] = true
		result.ExecutableLogical += item.LogicalSize
		result.ExecutableAllocation += item.AllocationSize
	}
	return result
}

// Digest is the SHA-256 of the plan's JSON encoding with an empty digest field. It
// detects edits and corruption; it is not a signature.
func Digest(p model.Plan) (string, error) {
	p.Digest = ""
	data, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// Verify rejects plans that were edited after creation or are outside their validity window.
func Verify(p model.Plan, now time.Time) error {
	if p.Schema != model.PlanSchema {
		return fmt.Errorf("unsupported plan schema %q", p.Schema)
	}
	digest, err := Digest(p)
	if err != nil {
		return err
	}
	if digest != p.Digest {
		return ErrTampered
	}
	if now.Before(p.CreatedAt) {
		return fmt.Errorf("plan was created in the future (%s)", p.CreatedAt.Format(time.RFC3339))
	}
	if !now.Before(p.ExpiresAt) {
		return ErrExpired
	}
	return nil
}
