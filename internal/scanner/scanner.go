package scanner

import (
	"context"
	"errors"

	"github.com/DRAG0NM/Pare/internal/model"
)

var ErrUnsupportedPlatform = errors.New("Windows directory scanner is only available on Windows")

type Scanner interface {
	Scan(context.Context, string) (model.ScanReport, error)
}
