package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

const ScanSchema = "pare.scan/1"

type ByteCount uint64

func (n ByteCount) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatUint(uint64(n), 10))
}

func (n *ByteCount) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("byte count must be a decimal string: %w", err)
	}
	if value == "" {
		return fmt.Errorf("byte count must not be empty")
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return fmt.Errorf("byte count %q is not an unsigned decimal integer", value)
		}
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fmt.Errorf("byte count %q is out of range: %w", value, err)
	}
	*n = ByteCount(parsed)
	return nil
}

type ScanReport struct {
	Schema      string      `json:"schema"`
	Volume      string      `json:"volume"`
	StartedAt   time.Time   `json:"started_at"`
	CompletedAt time.Time   `json:"completed_at"`
	Complete    bool        `json:"complete"`
	Nodes       []Node      `json:"nodes"`
	Errors      []ScanError `json:"errors"`
}

func (r *ScanReport) UnmarshalJSON(data []byte) error {
	type scanReport ScanReport
	var raw struct {
		Schema *string `json:"schema"`
		*scanReport
	}
	raw.scanReport = (*scanReport)(r)
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	if raw.Schema == nil || *raw.Schema == "" {
		return fmt.Errorf("scan report schema is required")
	}
	if *raw.Schema != ScanSchema {
		return fmt.Errorf("unsupported scan report schema %q", *raw.Schema)
	}
	r.Schema = *raw.Schema
	return nil
}

type Node struct {
	Path           string    `json:"path"`
	ParentPath     string    `json:"parent_path"`
	Kind           string    `json:"kind"`
	FileID         string    `json:"file_id"`
	LogicalSize    ByteCount `json:"logical_size"`
	AllocationSize ByteCount `json:"allocation_size"`
	Attributes     uint32    `json:"attributes"`
	LastWriteTime  time.Time `json:"last_write_time"`
}

type ScanError struct {
	Path      string `json:"path"`
	Operation string `json:"operation"`
	Code      string `json:"code"`
}
