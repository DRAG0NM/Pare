// Package serve exposes Pare to agent hosts (the DeepSeek Harness plugin) as a
// long-running process speaking line-delimited JSON over stdin/stdout. It keeps one
// scan index in memory, offers the same read-only investigation tools as the desktop
// deep dive, and prepares and executes cleanups. Every safety check stays in the core
// packages: protected locations, plan evidence, the executor's re-checks and template
// validation. The host is expected to obtain the user's approval before execute, undo
// and purge; serve still refuses a stale, altered or already used plan on its own.
package serve

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DRAG0NM/Pare/internal/actions"
	"github.com/DRAG0NM/Pare/internal/advisor"
	"github.com/DRAG0NM/Pare/internal/executor"
	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/plan"
	"github.com/DRAG0NM/Pare/internal/probes"
	"github.com/DRAG0NM/Pare/internal/scanner"
	"github.com/DRAG0NM/Pare/internal/tree"
)

// Schema names the protocol; a host refuses a core whose schema it does not know.
const Schema = "pare.serve/1"

// planTTL bounds how long a prepared cleanup may wait for approval.
const planTTL = 30 * time.Minute

// Options let tests replace the parts that touch the real machine.
type Options struct {
	Version string
	Scanner scanner.Scanner
	Prober  probes.Prober
	// QuarantineRoot picks the quarantine for a path; default is the volume root's.
	QuarantineRoot func(path string) string
	// QuarantineRoots lists every quarantine root to search for batches.
	QuarantineRoots func() []string
	Now             func() time.Time
	// SkipSystemChecks turns off the baseline diagnostics in scan_overview (tests).
	SkipSystemChecks bool
}

type request struct {
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type response struct {
	ID     int64  `json:"id"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Server is one session: the latest scan and the cleanups prepared from it.
type Server struct {
	opts Options

	mu       sync.Mutex
	report   model.ScanReport
	built    *tree.Tree
	env      *advisor.Environment
	root     string
	prepared map[string]*prepared
}

type prepared struct {
	plan    *model.Plan
	actions []actions.Prepared
	digest  string
	expires time.Time
	used    bool
}

// New returns a server with defaults filled in for the real machine.
func New(opts Options) *Server {
	if opts.Scanner == nil {
		opts.Scanner = scanner.NewWindowsScanner()
	}
	if opts.Prober == nil {
		opts.Prober = probes.NewWindowsProber()
	}
	if opts.QuarantineRoot == nil {
		opts.QuarantineRoot = executor.DefaultQuarantineRoot
	}
	if opts.QuarantineRoots == nil {
		opts.QuarantineRoots = defaultQuarantineRoots
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Server{opts: opts, prepared: map[string]*prepared{}}
}

// Run serves requests from in until it closes. Requests are handled one at a time, in
// order, so a host can rely on scan finishing before the next call sees its index.
func (s *Server) Run(ctx context.Context, in io.Reader, out io.Writer) error {
	reader := bufio.NewReaderSize(in, 1<<20)
	encoder := json.NewEncoder(out)
	for {
		line, err := reader.ReadBytes('\n')
		if len(strings.TrimSpace(string(line))) != 0 {
			var req request
			resp := response{}
			if jsonErr := json.Unmarshal(line, &req); jsonErr != nil {
				resp.Error = "invalid request: " + jsonErr.Error()
			} else {
				resp.ID = req.ID
				result, callErr := s.handle(ctx, req.Method, req.Params)
				if callErr != nil {
					resp.Error = callErr.Error()
				} else {
					resp.Result = result
				}
			}
			if writeErr := encoder.Encode(resp); writeErr != nil {
				return writeErr
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func (s *Server) handle(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	switch method {
	case "hello":
		return s.hello(), nil
	case "scan":
		var p struct{ Root string }
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		return s.scan(ctx, p.Root)
	case "tool":
		var p struct {
			Name string
			Args json.RawMessage
		}
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		return s.tool(ctx, p.Name, p.Args, false)
	case "read_sensitive":
		var p struct{ Path string }
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		args, _ := json.Marshal(map[string]string{"path": p.Path})
		return s.tool(ctx, "read_text", args, true)
	case "prepare":
		var p PrepareParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		return s.prepare(ctx, p)
	case "execute":
		var p struct{ ID, Digest string }
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		return s.execute(ctx, p.ID, p.Digest)
	case "batches":
		batches := executor.ListBatches(s.opts.QuarantineRoots())
		if batches == nil {
			batches = []executor.BatchSummary{}
		}
		return batches, nil
	case "undo":
		var p struct{ Manifest string }
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if err := s.checkManifest(p.Manifest); err != nil {
			return nil, err
		}
		return executor.Undo(ctx, p.Manifest, s.opts.Now())
	case "purge":
		var p struct{ Manifest string }
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		return s.purge(ctx, p.Manifest)
	}
	return nil, fmt.Errorf("unknown method %q", method)
}

func decode(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}
	return nil
}

// ToolSpec describes one investigation tool for the host to register.
type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

func (s *Server) hello() map[string]any {
	// The specs do not depend on a scan; an empty environment lists them.
	var specs []ToolSpec
	for _, t := range (&advisor.Environment{}).Tools() {
		specs = append(specs, ToolSpec{Name: t.Name, Description: t.Description, Parameters: json.RawMessage(t.Parameters)})
	}
	return map[string]any{"schema": Schema, "version": s.opts.Version, "tools": specs, "templates": advisor.Templates}
}

// ScanSummary is what the host shows after a scan; the full index stays in the core.
type ScanSummary struct {
	Root            string  `json:"root"`
	Complete        bool    `json:"complete"`
	Entries         int     `json:"entries"`
	Files           int     `json:"files"`
	Directories     int     `json:"directories"`
	AllocationBytes uint64  `json:"allocationBytes"`
	UniqueBytes     uint64  `json:"uniqueBytes"`
	VolumeUsedBytes uint64  `json:"volumeUsedBytes"`
	CoverageGaps    int     `json:"coverageGaps"`
	Seconds         float64 `json:"seconds"`
}

func (s *Server) scan(ctx context.Context, root string) (ScanSummary, error) {
	if !filepath.IsAbs(root) || filepath.VolumeName(root) == "" {
		return ScanSummary{}, fmt.Errorf("root must be an absolute Windows path, got %q", root)
	}
	root = filepath.Clean(root)
	if root == filepath.VolumeName(root) {
		root += `\`
	}
	started := s.opts.Now()
	report, err := s.opts.Scanner.Scan(ctx, root)
	if err != nil && len(report.Nodes) == 0 {
		return ScanSummary{}, err
	}
	built := tree.Build(root, report)
	var used uint64
	if root == filepath.VolumeName(root)+`\` {
		if total, free, ok := volumeSpace(root); ok {
			used = total - free
		}
	}
	env := &advisor.Environment{Root: root, Report: report, Tree: built, VolumeUsed: used, Now: s.opts.Now, SkipSystemChecks: s.opts.SkipSystemChecks}
	s.mu.Lock()
	s.report, s.built, s.env, s.root = report, built, env, root
	s.prepared = map[string]*prepared{}
	s.mu.Unlock()
	return ScanSummary{
		Root: root, Complete: report.Complete, Entries: len(report.Nodes),
		Files: built.Root.Files, Directories: built.Root.Directories,
		AllocationBytes: built.Root.AllocationSize, UniqueBytes: built.UniqueAllocation,
		VolumeUsedBytes: used, CoverageGaps: len(report.Errors), Seconds: s.opts.Now().Sub(started).Seconds(),
	}, nil
}

// tool runs one investigation tool. sensitive grants consent to read credential-like
// content for this one call; the host only sets it after the user approved.
func (s *Server) tool(ctx context.Context, name string, args json.RawMessage, sensitive bool) (string, error) {
	s.mu.Lock()
	env := s.env
	s.mu.Unlock()
	if env == nil {
		return "", errors.New("no scan yet: call pare_scan first")
	}
	call := *env
	call.Consent = nil
	if sensitive {
		call.Consent = func(context.Context, string, string) bool { return true }
	}
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	for _, t := range call.Tools() {
		if t.Name == name {
			return t.Run(ctx, args)
		}
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

// PrepareParams lists what to clean: paths to quarantine and supported templates.
type PrepareParams struct {
	Quarantine []string        `json:"quarantine"`
	Actions    []ActionRequest `json:"actions"`
}

type ActionRequest struct {
	Template string            `json:"template"`
	Params   map[string]string `json:"params"`
}

// Preview is a prepared cleanup. ID and Digest must be passed back to execute.
type Preview struct {
	ID        string          `json:"id"`
	Digest    string          `json:"digest"`
	ExpiresAt time.Time       `json:"expiresAt"`
	Items     []PreviewItem   `json:"items"`
	Actions   []PreviewAction `json:"actions"`
	// Bytes counts the items that would actually move.
	Bytes uint64 `json:"bytes"`
	// Runnable is false when nothing in the preview can be executed.
	Runnable bool `json:"runnable"`
}

type PreviewItem struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Bytes      uint64 `json:"bytes"`
	Executable bool   `json:"executable"`
	Reason     string `json:"reason,omitempty"`
}

type PreviewAction struct {
	Template   string   `json:"template"`
	Steps      []string `json:"steps"`
	Warnings   []string `json:"warnings"`
	NeedsAdmin bool     `json:"needsAdmin"`
	Error      string   `json:"error,omitempty"`
}

func (s *Server) prepare(ctx context.Context, p PrepareParams) (Preview, error) {
	if len(p.Quarantine) == 0 && len(p.Actions) == 0 {
		return Preview{}, errors.New("nothing to prepare: pass quarantine paths or actions")
	}
	s.mu.Lock()
	report, hasScan := s.report, s.env != nil
	s.mu.Unlock()
	now := s.opts.Now()
	entry := &prepared{}
	preview := Preview{Items: []PreviewItem{}, Actions: []PreviewAction{}}
	var digestParts []string

	if len(p.Quarantine) != 0 {
		if !hasScan {
			return Preview{}, errors.New("no scan yet: call pare_scan first")
		}
		var selections []plan.Selection
		for _, path := range p.Quarantine {
			selections = append(selections, plan.Selection{Path: path, Source: "pare-dsh"})
		}
		built, err := plan.BuildSelected(ctx, report, selections, s.opts.Prober, now, planTTL)
		if err != nil {
			return Preview{}, err
		}
		entry.plan = &built
		digestParts = append(digestParts, built.Digest)
		for _, item := range built.Items {
			view := PreviewItem{Path: item.Path, Kind: item.Kind, Bytes: uint64(item.AllocationSize), Executable: item.Executable}
			if view.Kind == "" {
				view.Kind = "file"
			}
			for _, e := range item.Evidence {
				if e.Status != model.EvidencePass {
					view.Reason = e.Detail
					break
				}
			}
			if item.Executable {
				preview.Bytes += view.Bytes
				preview.Runnable = true
			}
			preview.Items = append(preview.Items, view)
		}
	}
	for _, a := range p.Actions {
		view := PreviewAction{Template: a.Template, Steps: []string{}, Warnings: []string{}}
		ready, err := actions.Prepare(a.Template, a.Params)
		if err != nil {
			view.Error = err.Error()
		} else {
			view.Steps, view.Warnings, view.NeedsAdmin = ready.Display(), ready.Warnings, ready.NeedsAdmin
			entry.actions = append(entry.actions, ready)
			preview.Runnable = true
			digestParts = append(digestParts, strings.Join(view.Steps, "\n"))
		}
		preview.Actions = append(preview.Actions, view)
	}

	preview.ID = randomID()
	preview.Digest = digestOf(preview.ID, digestParts)
	preview.ExpiresAt = now.Add(planTTL)
	if preview.Runnable {
		s.mu.Lock()
		s.prepared[preview.ID] = entry
		s.mu.Unlock()
	}
	entry.digest, entry.expires = preview.Digest, preview.ExpiresAt
	return preview, nil
}

// ExecuteResult reports one executed cleanup.
type ExecuteResult struct {
	Manifest         string         `json:"manifest,omitempty"`
	Quarantined      int            `json:"quarantined"`
	QuarantinedBytes uint64         `json:"quarantinedBytes"`
	Skipped          []PreviewItem  `json:"skipped"`
	Actions          []ActionResult `json:"actions"`
	FreeBefore       uint64         `json:"freeBefore"`
	FreeAfter        uint64         `json:"freeAfter"`
	Error            string         `json:"error,omitempty"`
}

type ActionResult struct {
	Template string `json:"template"`
	Status   string `json:"status"` // done, failed
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
}

func (s *Server) execute(ctx context.Context, id, digest string) (ExecuteResult, error) {
	s.mu.Lock()
	entry := s.prepared[id]
	switch {
	case entry == nil:
		s.mu.Unlock()
		return ExecuteResult{}, errors.New("unknown or discarded cleanup id; prepare it again")
	case entry.used:
		s.mu.Unlock()
		return ExecuteResult{}, errors.New("this cleanup was already executed; prepare a new one")
	case entry.digest != digest:
		s.mu.Unlock()
		return ExecuteResult{}, errors.New("digest does not match the prepared cleanup")
	case s.opts.Now().After(entry.expires):
		s.mu.Unlock()
		return ExecuteResult{}, errors.New("the prepared cleanup expired; prepare it again")
	}
	// Mark it used before running: a cleanup executes at most once, even if it fails.
	entry.used = true
	root := s.root
	s.mu.Unlock()

	result := ExecuteResult{Skipped: []PreviewItem{}, Actions: []ActionResult{}}
	volume := filepath.VolumeName(root) + `\`
	_, result.FreeBefore, _ = volumeSpace(volume)
	if entry.plan != nil {
		var selected []string
		for _, item := range entry.plan.Items {
			if item.Executable {
				selected = append(selected, item.Path)
			}
		}
		if len(selected) != 0 {
			execution, manifest, err := executor.Apply(ctx, *entry.plan, selected, entry.plan.Digest, s.opts.QuarantineRoot(selected[0]), s.opts.Now())
			result.Manifest = manifest
			if err != nil {
				result.Error = err.Error()
			}
			for _, item := range execution.Items {
				if item.State == model.StateQuarantined {
					result.Quarantined++
					result.QuarantinedBytes += uint64(item.AllocationSize)
				} else {
					result.Skipped = append(result.Skipped, PreviewItem{Path: item.OriginalPath, Kind: item.Kind, Bytes: uint64(item.AllocationSize), Reason: item.Detail})
				}
			}
		}
	}
	for _, ready := range entry.actions {
		output, err := actions.Run(ctx, ready)
		r := ActionResult{Template: ready.Template, Status: "done", Output: clip(output, 4000)}
		if err != nil {
			r.Status, r.Error = "failed", err.Error()
		}
		result.Actions = append(result.Actions, r)
	}
	_, result.FreeAfter, _ = volumeSpace(volume)
	return result, nil
}

// PurgeResult reports a permanently emptied batch.
type PurgeResult struct {
	Purged     int   `json:"purged"`
	FreedBytes int64 `json:"freedBytes"`
}

func (s *Server) purge(ctx context.Context, manifest string) (PurgeResult, error) {
	if err := s.checkManifest(manifest); err != nil {
		return PurgeResult{}, err
	}
	volume := filepath.VolumeName(manifest) + `\`
	_, before, _ := volumeSpace(volume)
	execution, err := executor.Purge(ctx, manifest, s.opts.Now())
	_, after, _ := volumeSpace(volume)
	result := PurgeResult{FreedBytes: int64(after) - int64(before)}
	for _, item := range execution.Items {
		if item.State == model.StatePurged {
			result.Purged++
		}
	}
	return result, err
}

// checkManifest only accepts manifests inside a Pare quarantine root.
func (s *Server) checkManifest(manifest string) error {
	clean := strings.ToLower(filepath.Clean(manifest))
	for _, root := range s.opts.QuarantineRoots() {
		if strings.HasPrefix(clean, strings.ToLower(filepath.Clean(root))+`\`) && filepath.Base(clean) == executor.ManifestName {
			return nil
		}
	}
	return errors.New("not a manifest inside a Pare quarantine")
}

// digestOf binds a preview's id to exactly what it would do: the plan digest (which
// covers every item and its evidence) and each template's steps.
func digestOf(id string, parts []string) string {
	sum := sha256.Sum256([]byte(id + "\x00" + strings.Join(parts, "\x00")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func randomID() string {
	var b [12]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func clip(s string, limit int) string {
	if r := []rune(s); len(r) > limit {
		return string(r[:limit]) + "…"
	}
	return s
}
