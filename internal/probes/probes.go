// Package probes collects live, read-only evidence about candidate files.
package probes

import (
	"context"

	"github.com/DRAG0NM/Pare/internal/model"
)

const (
	KindUnchanged = "unchanged"
	KindUnlocked  = "unlocked"
)

// Prober re-checks a scanned file. It returns one KindUnchanged and one KindUnlocked
// evidence entry and must never modify the file.
type Prober interface {
	Probe(ctx context.Context, node model.Node) []model.Evidence
}

func unknown(detail string) []model.Evidence {
	return []model.Evidence{
		{Kind: KindUnchanged, Status: model.EvidenceUnknown, Detail: detail},
		{Kind: KindUnlocked, Status: model.EvidenceUnknown, Detail: detail},
	}
}
