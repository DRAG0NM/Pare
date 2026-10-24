package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

const ExecutionSchema = "pare.execution/1"

type ExecutionState string

const (
	// StatePending items have not been touched yet.
	StatePending ExecutionState = "pending"
	// StateIntent is recorded before a move starts; after a crash the file may be in
	// either place, so undo checks both.
	StateIntent      ExecutionState = "intent"
	StateQuarantined ExecutionState = "quarantined"
	StateSkipped     ExecutionState = "skipped"
	StateFailed      ExecutionState = "failed"
	StateRestored    ExecutionState = "restored"
	// StateConflict means undo found the original path occupied and left the file in quarantine.
	StateConflict ExecutionState = "conflict"
	// StatePurged items were permanently deleted from quarantine at the user's request.
	StatePurged ExecutionState = "purged"
)

type ExecutionItem struct {
	OriginalPath string `json:"original_path"`
	// Kind is "directory" when a whole folder was moved; empty means a file.
	Kind           string         `json:"kind,omitempty"`
	QuarantinePath string         `json:"quarantine_path"`
	FileID         string         `json:"file_id"`
	LogicalSize    ByteCount      `json:"logical_size"`
	AllocationSize ByteCount      `json:"allocation_size"`
	LastWriteTime  time.Time      `json:"last_write_time"`
	State          ExecutionState `json:"state"`
	Detail         string         `json:"detail"`
}

type Execution struct {
	Schema        string          `json:"schema"`
	PlanDigest    string          `json:"plan_digest"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	QuarantineDir string          `json:"quarantine_dir"`
	Items         []ExecutionItem `json:"items"`
}

func (e *Execution) UnmarshalJSON(data []byte) error {
	type execution Execution
	var raw struct {
		Schema *string `json:"schema"`
		*execution
	}
	raw.execution = (*execution)(e)
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	if raw.Schema == nil || *raw.Schema == "" {
		return fmt.Errorf("execution schema is required")
	}
	if *raw.Schema != ExecutionSchema {
		return fmt.Errorf("unsupported execution schema %q", *raw.Schema)
	}
	e.Schema = *raw.Schema
	return nil
}

// Count returns how many items are in the given state.
func (e Execution) Count(state ExecutionState) int {
	count := 0
	for _, item := range e.Items {
		if item.State == state {
			count++
		}
	}
	return count
}
