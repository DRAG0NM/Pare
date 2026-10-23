package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestScanReportPreservesLargeSizesAndEmptyErrors(t *testing.T) {
	const largeSize = uint64(9007199254740993)
	created := time.Date(2026, time.October, 20, 12, 30, 0, 0, time.UTC)
	report := ScanReport{
		Schema:      ScanSchema,
		Volume:      `C:\`,
		StartedAt:   created,
		CompletedAt: created.Add(time.Second),
		Complete:    true,
		Nodes: []Node{{
			Path:           `C:\large.bin`,
			ParentPath:     `C:\`,
			Kind:           "file",
			FileID:         "18446744073709551615",
			LogicalSize:    ByteCount(largeSize),
			AllocationSize: ByteCount(largeSize),
			Attributes:     32,
			LastWriteTime:  created,
		}},
		Errors: []ScanError{},
	}

	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.Contains(string(encoded), `"logical_size":"9007199254740993"`) {
		t.Fatalf("JSON does not preserve the large size as a decimal string: %s", encoded)
	}
	if !strings.Contains(string(encoded), `"file_id":"18446744073709551615"`) {
		t.Fatalf("JSON does not preserve the file ID as a string: %s", encoded)
	}
	if !strings.Contains(string(encoded), `"errors":[]`) {
		t.Fatalf("empty errors were not encoded as an array: %s", encoded)
	}

	var decoded ScanReport
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if decoded.Schema != ScanSchema || decoded.Nodes[0].LogicalSize != ByteCount(largeSize) {
		t.Fatalf("round trip lost schema or size: %#v", decoded)
	}
	if len(decoded.Errors) != 0 || decoded.Errors == nil {
		t.Fatalf("errors = %#v, want a non-nil empty slice", decoded.Errors)
	}
}

func TestScanReportRoundTripsCoverageErrors(t *testing.T) {
	report := ScanReport{
		Schema: ScanSchema,
		Errors: []ScanError{{Path: `C:\Restricted`, Operation: "enumerate", Code: "access_denied"}},
	}

	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded ScanReport
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(decoded.Errors) != 1 || decoded.Errors[0].Code != "access_denied" {
		t.Fatalf("errors = %#v, want one bounded access error", decoded.Errors)
	}
}

func TestScanReportRejectsMissingSchema(t *testing.T) {
	var report ScanReport
	err := json.Unmarshal([]byte(`{"volume":"C:\\"}`), &report)
	if err == nil {
		t.Fatal("Unmarshal() error = nil, want missing schema to be rejected")
	}
}

func TestByteCountRejectsMalformedInteger(t *testing.T) {
	var size ByteCount
	if err := json.Unmarshal([]byte(`"12.5"`), &size); err == nil {
		t.Fatal("Unmarshal() error = nil, want a non-integer decimal string to be rejected")
	}
}
