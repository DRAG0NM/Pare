package scanner

import (
	"context"

	"github.com/DRAG0NM/Pare/internal/model"
)

type Scanner interface {
	Scan(context.Context, string) (model.ScanReport, error)
}
