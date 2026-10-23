package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/DRAG0NM/Pare/internal/model"
)

type readDirectory func(string) ([]os.DirEntry, error)

type WalkScanner struct {
	readDir readDirectory
}

func NewWalkScanner() WalkScanner {
	return NewWalkScannerWithReader(os.ReadDir)
}

func NewWalkScannerWithReader(readDir func(string) ([]os.DirEntry, error)) WalkScanner {
	return WalkScanner{readDir: readDir}
}

func (s WalkScanner) Scan(ctx context.Context, root string) (model.ScanReport, error) {
	started := time.Now().UTC()
	report := model.ScanReport{
		Schema:    model.ScanSchema,
		StartedAt: started,
		Complete:  true,
		Nodes:     []model.Node{},
		Errors:    []model.ScanError{},
	}

	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return report, err
	}
	report.Volume = absoluteRoot
	if s.readDir == nil {
		return report, errors.New("directory reader is not configured")
	}

	pending := []string{absoluteRoot}
	for len(pending) != 0 {
		if err := ctx.Err(); err != nil {
			report.Complete = false
			report.CompletedAt = time.Now().UTC()
			return report, err
		}
		directory := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		entries, err := s.readDir(directory)
		if err != nil {
			report.Complete = false
			report.Errors = append(report.Errors, model.ScanError{
				Path: directory, Operation: "enumerate", Code: errorCode(err),
			})
			continue
		}

		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				report.Complete = false
				report.CompletedAt = time.Now().UTC()
				return report, err
			}
			path := filepath.Join(directory, entry.Name())
			info, err := entry.Info()
			if err != nil {
				report.Complete = false
				report.Errors = append(report.Errors, model.ScanError{
					Path: path, Operation: "stat", Code: errorCode(err),
				})
				continue
			}

			kind := "file"
			if entry.IsDir() {
				kind = "directory"
			} else if info.Mode()&os.ModeSymlink != 0 {
				kind = "symlink"
			}
			report.Nodes = append(report.Nodes, model.Node{
				Path:          path,
				ParentPath:    directory,
				Kind:          kind,
				LogicalSize:   model.ByteCount(max(info.Size(), 0)),
				LastWriteTime: info.ModTime().UTC(),
			})
			if kind == "directory" {
				pending = append(pending, path)
			}
		}
	}

	report.CompletedAt = time.Now().UTC()
	return report, nil
}

func errorCode(err error) string {
	if errors.Is(err, os.ErrPermission) {
		return "access_denied"
	}
	return "io_error"
}
