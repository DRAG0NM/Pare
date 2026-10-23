package scanner

import (
	"context"
	"errors"
	"time"

	"github.com/DRAG0NM/Pare/internal/model"
)

var ErrUnsupportedPlatform = errors.New("Windows directory scanner is only available on Windows")

type Scanner interface {
	Scan(context.Context, string) (model.ScanReport, error)
}

// DefaultWorkers is the number of directories read at the same time.
const DefaultWorkers = 8

const DefaultProgressInterval = 100 * time.Millisecond

// Progress is a snapshot of a running scan.
type Progress struct {
	Entries     int    `json:"entries"`
	Directories int    `json:"directories"`
	Current     string `json:"current"`
	Done        bool   `json:"done"`
}

type Options struct {
	// Workers is how many directories are read concurrently; 0 means DefaultWorkers.
	Workers int
	// Progress, when set, is called from a separate goroutine about every
	// ProgressInterval and once more with Done set when the scan ends.
	Progress         func(Progress)
	ProgressInterval time.Duration
}
