// Package rules holds Pare's built-in cleanup rules and their path matcher.
package rules

import (
	"path/filepath"
	"strings"
	"time"
)

// Version identifies the built-in rule set recorded in every plan.
const Version = "builtin/1"

const day = 24 * time.Hour

type Rule struct {
	ID          string
	Version     int
	Risk        string
	Description string
	// Under lists path segments below the volume root; "*" matches exactly one segment.
	// A rule matches only files strictly inside this directory.
	Under []string
	// MinAge is the lifecycle evidence: the file must not have been written for this long.
	MinAge time.Duration
}

// Builtin returns the rule set. Every rule is E1: rebuildable data whose loss costs a
// re-download or recomputation, so items are never selected for the user.
func Builtin() []Rule {
	return []Rule{
		{
			ID: "user-temp", Version: 1, Risk: "E1",
			Description: "Per-user temporary files",
			Under:       []string{"Users", "*", "AppData", "Local", "Temp"},
			MinAge:      7 * day,
		},
		{
			ID: "npm-cache", Version: 1, Risk: "E1",
			Description: "npm content cache; npm re-downloads missing entries",
			Under:       []string{"Users", "*", "AppData", "Local", "npm-cache", "_cacache"},
			MinAge:      30 * day,
		},
		{
			ID: "pip-cache", Version: 1, Risk: "E1",
			Description: "pip download and wheel cache",
			Under:       []string{"Users", "*", "AppData", "Local", "pip", "cache"},
			MinAge:      30 * day,
		},
		{
			ID: "go-build-cache", Version: 1, Risk: "E1",
			Description: "Go build cache; go rebuilds missing entries",
			Under:       []string{"Users", "*", "AppData", "Local", "go-build"},
			MinAge:      30 * day,
		},
	}
}

// Matches reports whether path lies strictly inside the rule's directory.
func (r Rule) Matches(path string) bool {
	segments := pathSegments(path)
	if len(segments) <= len(r.Under) {
		return false
	}
	for i, want := range r.Under {
		if want != "*" && !strings.EqualFold(want, segments[i]) {
			return false
		}
	}
	return true
}

func pathSegments(path string) []string {
	path = strings.TrimPrefix(path, `\\?\`)
	rest := strings.TrimPrefix(path, filepath.VolumeName(path))
	return strings.FieldsFunc(rest, func(r rune) bool { return r == '\\' || r == '/' })
}
