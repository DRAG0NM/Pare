// Package executor moves planned files into a same-volume quarantine and restores them.
//
// Every move follows the same order: record intent in the manifest, open the file with
// exclusive access, re-check its identity on that handle, then rename through the same
// handle without replacing anything. A file that cannot pass every step stays where it is.
package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/plan"
	"github.com/DRAG0NM/Pare/internal/report"
)

const ManifestName = "manifest.json"

const fileAttributeSystem = 0x4

var (
	ErrUnsupportedPlatform = errors.New("quarantine is only available on Windows")
	ErrNothingToApply      = errors.New("plan has no executable items")
	ErrUnconfirmed         = errors.New("confirmation does not match the plan digest")
	ErrNotExecutable       = errors.New("selected path is not an executable item of the plan, or is selected twice")

	errInUse   = errors.New("file is open in another process")
	errMissing = errors.New("file does not exist")
	errExists  = errors.New("destination already exists")
)

type fileInfo struct {
	FileID     string
	Size       uint64
	LastWrite  time.Time
	Attributes uint32
	Reparse    bool
	Directory  bool
}

// lockedFile is a file opened with exclusive access and permission to rename it.
type lockedFile interface {
	Info() (fileInfo, error)
	Rename(destination string) error
	Close() error
}

var openLocked func(path string) (lockedFile, error)
var hasReparseAncestor func(path string) (bool, error)

// DefaultQuarantineRoot is the hidden-by-convention directory at the root of a file's volume.
func DefaultQuarantineRoot(path string) string {
	return filepath.VolumeName(path) + `\.pare-quarantine`
}

// Apply quarantines executable items of a verified plan: the paths in selected, or every
// executable item when selected is nil. confirm must equal the plan digest, which ties
// the action to the plan the user reviewed. A selected path that is not an executable
// item rejects the whole run. The manifest is written before each move, so a failure to
// persist it stops the run without moving anything more.
func Apply(ctx context.Context, p model.Plan, selected []string, confirm, root string, now time.Time) (model.Execution, string, error) {
	if openLocked == nil {
		return model.Execution{}, "", ErrUnsupportedPlatform
	}
	if err := plan.Verify(p, now); err != nil {
		return model.Execution{}, "", err
	}
	if confirm != p.Digest {
		return model.Execution{}, "", ErrUnconfirmed
	}
	items, err := selectItems(p, selected)
	if err != nil {
		return model.Execution{}, "", err
	}
	if len(items) == 0 {
		return model.Execution{}, "", ErrNothingToApply
	}
	for _, item := range items {
		if !strings.EqualFold(filepath.VolumeName(item.Path), filepath.VolumeName(root)) {
			return model.Execution{}, "", fmt.Errorf("quarantine %q is not on the volume of %q", root, item.Path)
		}
	}

	batch := filepath.Join(root, now.UTC().Format("20060102-150405")+"-"+strings.TrimPrefix(p.Digest, "sha256:")[:12])
	if err := os.MkdirAll(root, 0o700); err != nil {
		return model.Execution{}, "", err
	}
	if err := os.Mkdir(batch, 0o700); err != nil {
		return model.Execution{}, "", fmt.Errorf("create batch directory: %w", err)
	}
	if err := os.Mkdir(filepath.Join(batch, "files"), 0o700); err != nil {
		return model.Execution{}, "", err
	}
	if reparse, err := hasReparseAncestor(filepath.Join(batch, "files", "x")); err != nil || reparse {
		return model.Execution{}, "", fmt.Errorf("quarantine path %q passes through a reparse point or cannot be checked: %v", batch, err)
	}

	execution := model.Execution{
		Schema:        model.ExecutionSchema,
		PlanDigest:    p.Digest,
		CreatedAt:     now.UTC(),
		UpdatedAt:     now.UTC(),
		QuarantineDir: batch,
	}
	for index, item := range items {
		execution.Items = append(execution.Items, model.ExecutionItem{
			OriginalPath:   item.Path,
			Kind:           item.Kind,
			QuarantinePath: filepath.Join(batch, "files", fmt.Sprintf("%06d-%s", index, filepath.Base(item.Path))),
			FileID:         item.FileID,
			LogicalSize:    item.LogicalSize,
			AllocationSize: item.AllocationSize,
			LastWriteTime:  item.LastWriteTime,
			State:          model.StatePending,
		})
	}
	manifest := filepath.Join(batch, ManifestName)
	if err := writeManifest(manifest, &execution); err != nil {
		return execution, manifest, err
	}

	for i := range execution.Items {
		if err := ctx.Err(); err != nil {
			return execution, manifest, err
		}
		item := &execution.Items[i]
		item.State = model.StateIntent
		if err := writeManifest(manifest, &execution); err != nil {
			return execution, manifest, err
		}
		item.State, item.Detail = quarantine(*item)
		if err := writeManifest(manifest, &execution); err != nil {
			return execution, manifest, err
		}
	}
	return execution, manifest, nil
}

func selectItems(p model.Plan, selected []string) ([]model.PlanItem, error) {
	var items []model.PlanItem
	if selected == nil {
		for _, item := range p.Items {
			if item.Executable {
				items = append(items, item)
			}
		}
		return items, nil
	}
	executable := map[string]model.PlanItem{}
	for _, item := range p.Items {
		if item.Executable {
			executable[item.Path] = item
		}
	}
	for _, path := range selected {
		item, ok := executable[path]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrNotExecutable, path)
		}
		delete(executable, path)
		items = append(items, item)
	}
	return items, nil
}

func quarantine(item model.ExecutionItem) (model.ExecutionState, string) {
	if protected, reason := plan.Protected(item.OriginalPath); protected {
		return model.StateSkipped, "protected location: " + reason
	}
	if reparse, err := hasReparseAncestor(item.OriginalPath); err != nil {
		return model.StateSkipped, "cannot check parent directories: " + err.Error()
	} else if reparse {
		return model.StateSkipped, "a parent directory is a reparse point"
	}
	file, err := openLocked(item.OriginalPath)
	if err != nil {
		return model.StateSkipped, err.Error()
	}
	defer file.Close()
	info, err := file.Info()
	if err != nil {
		return model.StateSkipped, err.Error()
	}
	if changes := compare(item, info); changes != "" {
		return model.StateSkipped, "changed since plan: " + changes
	}
	if err := file.Rename(item.QuarantinePath); err != nil {
		if item.Kind == "directory" && errors.Is(err, errInUse) {
			return model.StateSkipped, "a file inside the directory is in use"
		}
		return model.StateFailed, "move failed: " + err.Error()
	}
	return model.StateQuarantined, ""
}

// compare checks the live file against the plan. Directories are compared by identity
// only: their size is the sum of contents and their timestamp changes whenever a file
// inside does.
func compare(item model.ExecutionItem, info fileInfo) string {
	var changes []string
	if info.FileID != item.FileID {
		changes = append(changes, "file_id")
	}
	if directory := item.Kind == "directory"; directory != info.Directory {
		changes = append(changes, "kind")
	} else if !directory {
		if info.Size != uint64(item.LogicalSize) {
			changes = append(changes, "size")
		}
		if !info.LastWrite.Equal(item.LastWriteTime) {
			changes = append(changes, "last_write_time")
		}
	}
	if info.Reparse {
		changes = append(changes, "reparse")
	}
	if info.Attributes&fileAttributeSystem != 0 {
		changes = append(changes, "system")
	}
	return strings.Join(changes, ", ")
}

// Undo moves quarantined files back. It never replaces a file at the original path; such
// items become conflicts and stay in quarantine.
func Undo(ctx context.Context, manifest string, now time.Time) (model.Execution, error) {
	if openLocked == nil {
		return model.Execution{}, ErrUnsupportedPlatform
	}
	data, err := os.ReadFile(manifest)
	if err != nil {
		return model.Execution{}, err
	}
	var execution model.Execution
	if err := json.Unmarshal(data, &execution); err != nil {
		return model.Execution{}, fmt.Errorf("parse manifest: %w", err)
	}
	execution.UpdatedAt = now.UTC()

	for i := range execution.Items {
		if err := ctx.Err(); err != nil {
			return execution, err
		}
		item := &execution.Items[i]
		if item.State != model.StateQuarantined && item.State != model.StateIntent && item.State != model.StateConflict {
			continue
		}
		item.State, item.Detail = restore(*item)
		if err := writeManifest(manifest, &execution); err != nil {
			return execution, err
		}
	}
	return execution, nil
}

func restore(item model.ExecutionItem) (model.ExecutionState, string) {
	file, err := openLocked(item.QuarantinePath)
	if errors.Is(err, errMissing) && item.State == model.StateIntent {
		// The move never happened; the file is still at its original path or gone.
		return model.StateSkipped, "not moved before the run stopped"
	}
	if err != nil {
		return item.State, "cannot open quarantined file: " + err.Error()
	}
	defer file.Close()
	info, err := file.Info()
	if err != nil {
		return item.State, err.Error()
	}
	if info.FileID != item.FileID {
		return item.State, "quarantined file has a different file ID"
	}
	if err := os.MkdirAll(filepath.Dir(item.OriginalPath), 0o700); err != nil {
		return item.State, "recreate parent directory: " + err.Error()
	}
	if reparse, err := hasReparseAncestor(item.OriginalPath); err != nil || reparse {
		return item.State, "original parent directory is a reparse point or cannot be checked"
	}
	if err := file.Rename(item.OriginalPath); errors.Is(err, errExists) {
		return model.StateConflict, "original path is occupied; file kept in quarantine"
	} else if err != nil {
		return item.State, "restore failed: " + err.Error()
	}
	return model.StateRestored, ""
}

func writeManifest(path string, execution *model.Execution) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".manifest-*")
	if err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	defer os.Remove(temporary.Name())
	if err := report.WriteJSON(temporary, execution); err != nil {
		temporary.Close()
		return fmt.Errorf("write manifest: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("write manifest: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	if err := os.Rename(temporary.Name(), path); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

// Purge permanently deletes the quarantined items of one batch, freeing their space. Only
// items Pare moved into the batch (quarantined or kept as conflicts) are deleted, and only
// paths inside the batch's files folder; the manifest stays as a record.
func Purge(ctx context.Context, manifest string, now time.Time) (model.Execution, error) {
	data, err := os.ReadFile(manifest)
	if err != nil {
		return model.Execution{}, err
	}
	var execution model.Execution
	if err := json.Unmarshal(data, &execution); err != nil {
		return model.Execution{}, fmt.Errorf("parse manifest: %w", err)
	}
	execution.UpdatedAt = now.UTC()
	files := strings.ToLower(filepath.Join(filepath.Dir(manifest), "files")) + `\`
	for i := range execution.Items {
		if err := ctx.Err(); err != nil {
			return execution, err
		}
		item := &execution.Items[i]
		if item.State != model.StateQuarantined && item.State != model.StateConflict {
			continue
		}
		target := filepath.Clean(item.QuarantinePath)
		if !strings.HasPrefix(strings.ToLower(target), files) {
			item.Detail = "not inside this batch; left untouched"
			continue
		}
		if err := removeAll(target); err != nil {
			item.Detail = "delete failed: " + err.Error()
		} else {
			item.State, item.Detail = model.StatePurged, ""
		}
		if err := writeManifest(manifest, &execution); err != nil {
			return execution, err
		}
	}
	return execution, nil
}

// removeAll deletes path and, if read-only files block it, clears their read-only flag
// and tries once more.
func removeAll(path string) error {
	if err := os.RemoveAll(path); err == nil {
		return nil
	}
	_ = filepath.WalkDir(path, func(p string, entry os.DirEntry, err error) error {
		if err == nil && entry.Type().IsRegular() {
			_ = os.Chmod(p, 0o666)
		}
		return nil
	})
	return os.RemoveAll(path)
}

type BatchSummary struct {
	Manifest         string    `json:"manifest"`
	CreatedAt        time.Time `json:"createdAt"`
	Items            int       `json:"items"`
	Quarantined      int       `json:"quarantined"`
	Restored         int       `json:"restored"`
	Purged           int       `json:"purged"`
	Other            int       `json:"other"`
	QuarantinedBytes uint64    `json:"quarantinedBytes"`
}

// ListBatches reads the manifests under the given quarantine roots, newest first.
func ListBatches(roots []string) []BatchSummary {
	var batches []BatchSummary
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			manifest := filepath.Join(root, entry.Name(), ManifestName)
			data, err := os.ReadFile(manifest)
			if !entry.IsDir() || err != nil {
				continue
			}
			var execution model.Execution
			if json.Unmarshal(data, &execution) != nil {
				continue
			}
			summary := BatchSummary{Manifest: manifest, CreatedAt: execution.CreatedAt, Items: len(execution.Items)}
			for _, item := range execution.Items {
				switch item.State {
				case model.StateQuarantined, model.StateConflict:
					summary.Quarantined++
					summary.QuarantinedBytes += uint64(item.AllocationSize)
				case model.StateRestored:
					summary.Restored++
				case model.StatePurged:
					summary.Purged++
				default:
					summary.Other++
				}
			}
			batches = append(batches, summary)
		}
	}
	sort.Slice(batches, func(i, j int) bool { return batches[i].CreatedAt.After(batches[j].CreatedAt) })
	return batches
}
