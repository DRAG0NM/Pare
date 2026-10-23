package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

const PlanSchema = "pare.plan/1"

type EvidenceStatus string

const (
	EvidencePass    EvidenceStatus = "pass"
	EvidenceFail    EvidenceStatus = "fail"
	EvidenceUnknown EvidenceStatus = "unknown"
	EvidenceError   EvidenceStatus = "error"
)

type Evidence struct {
	Kind   string         `json:"kind"`
	Status EvidenceStatus `json:"status"`
	Detail string         `json:"detail"`
}

type PlanSource struct {
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	Complete    bool      `json:"complete"`
}

type PlanItem struct {
	Path           string     `json:"path"`
	FileID         string     `json:"file_id"`
	LogicalSize    ByteCount  `json:"logical_size"`
	AllocationSize ByteCount  `json:"allocation_size"`
	Attributes     uint32     `json:"attributes"`
	LastWriteTime  time.Time  `json:"last_write_time"`
	RuleID         string     `json:"rule_id"`
	RuleVersion    int        `json:"rule_version"`
	Risk           string     `json:"risk"`
	Action         string     `json:"action"`
	Executable     bool       `json:"executable"`
	Evidence       []Evidence `json:"evidence"`
}

type PlanTotals struct {
	Candidates           int       `json:"candidates"`
	Executable           int       `json:"executable"`
	ExecutableLogical    ByteCount `json:"executable_logical_size"`
	ExecutableAllocation ByteCount `json:"executable_allocation_size"`
}

type Plan struct {
	Schema       string     `json:"schema"`
	Digest       string     `json:"digest"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    time.Time  `json:"expires_at"`
	Volume       string     `json:"volume"`
	Scan         PlanSource `json:"scan"`
	RulesVersion string     `json:"rules_version"`
	Items        []PlanItem `json:"items"`
	Totals       PlanTotals `json:"totals"`
}

func (p *Plan) UnmarshalJSON(data []byte) error {
	type plan Plan
	var raw struct {
		Schema *string `json:"schema"`
		*plan
	}
	raw.plan = (*plan)(p)
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	if raw.Schema == nil || *raw.Schema == "" {
		return fmt.Errorf("plan schema is required")
	}
	if *raw.Schema != PlanSchema {
		return fmt.Errorf("unsupported plan schema %q", *raw.Schema)
	}
	p.Schema = *raw.Schema
	return nil
}
