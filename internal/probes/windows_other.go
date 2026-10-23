//go:build !windows

package probes

import (
	"context"

	"github.com/DRAG0NM/Pare/internal/model"
)

type unsupportedProber struct{}

func NewWindowsProber() Prober {
	return unsupportedProber{}
}

func (unsupportedProber) Probe(context.Context, model.Node) []model.Evidence {
	return unknown("file probes are only available on Windows")
}
