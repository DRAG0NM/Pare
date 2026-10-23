//go:build !windows

package scanner

import (
	"context"

	"github.com/DRAG0NM/Pare/internal/model"
)

type unsupportedWindowsScanner struct{}

func NewWindowsScanner() Scanner {
	return unsupportedWindowsScanner{}
}

func NewWindowsScannerWithOptions(Options) Scanner {
	return unsupportedWindowsScanner{}
}

func (unsupportedWindowsScanner) Scan(context.Context, string) (model.ScanReport, error) {
	return model.ScanReport{}, ErrUnsupportedPlatform
}
