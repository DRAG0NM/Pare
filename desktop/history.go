package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DRAG0NM/Pare/internal/model"
)

// historyLimit keeps the record file small; older entries are dropped.
const historyLimit = 200

// HistoryEntry is one past cleanup or purge, kept so the user can look back later.
type HistoryEntry struct {
	Time             time.Time       `json:"time"`
	Kind             string          `json:"kind"` // clean, purge
	Volume           string          `json:"volume"`
	FreedBytes       int64           `json:"freedBytes"`
	QuarantinedBytes uint64          `json:"quarantinedBytes"`
	Items            int             `json:"items"`
	Actions          []HistoryAction `json:"actions"`
}

type HistoryAction struct {
	FindingTitle string `json:"findingTitle"`
	Label        string `json:"label"`
	Kind         string `json:"kind"`
	Status       string `json:"status"`
	Bytes        uint64 `json:"bytes"`
}

var historyMu sync.Mutex

func (a *App) historyPath() string {
	if a.historyFile != "" {
		return a.historyFile
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Pare", "history.json")
}

func loadHistory(path string) []HistoryEntry {
	var entries []HistoryEntry
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &entries)
	}
	return entries
}

// appendHistory is best effort: a failed write never fails the cleanup it records.
func appendHistory(path string, entry HistoryEntry) {
	if path == "" {
		return
	}
	historyMu.Lock()
	defer historyMu.Unlock()
	entries := append(loadHistory(path), entry)
	if len(entries) > historyLimit {
		entries = entries[len(entries)-historyLimit:]
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(path), 0o700)
	tmp := path + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		os.Rename(tmp, path)
	}
}

func (a *App) recordCleanup(report ExecutionReport) {
	entry := HistoryEntry{Time: report.Time, Kind: "clean", Volume: report.Volume, FreedBytes: report.FreedBytes, QuarantinedBytes: report.QuarantinedBytes, Actions: []HistoryAction{}}
	for _, result := range report.Results {
		entry.Actions = append(entry.Actions, HistoryAction{FindingTitle: result.FindingTitle, Label: result.Label, Kind: result.Kind, Status: result.Status, Bytes: result.Bytes})
		if result.Status == "done" || result.Status == "partial" {
			entry.Items++
		}
	}
	if len(entry.Actions) != 0 {
		appendHistory(a.historyPath(), entry)
	}
}

func (a *App) recordPurge(volume string, execution model.Execution, freed int64) {
	entry := HistoryEntry{Time: time.Now(), Kind: "purge", Volume: volume, FreedBytes: freed, Actions: []HistoryAction{}}
	for _, item := range execution.Items {
		if item.State == model.StatePurged {
			entry.Items++
		}
	}
	if entry.Items != 0 {
		appendHistory(a.historyPath(), entry)
	}
}

// ListHistory returns past cleanups and purges, newest first.
func (a *App) ListHistory() []HistoryEntry {
	historyMu.Lock()
	entries := loadHistory(a.historyPath())
	historyMu.Unlock()
	out := make([]HistoryEntry, 0, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		out = append(out, entries[i])
	}
	return out
}
