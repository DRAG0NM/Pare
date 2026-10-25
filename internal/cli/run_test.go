package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/scanner"
)

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run(context.Background(), []string{"--version"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if got, want := stdout.String(), "pare 0.4.0\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run(context.Background(), []string{"not-a-command"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("Run() exit code = 0, want non-zero for an unknown command")
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("stderr = %q, want usage text", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunScanWritesCompleteReportToStdout(t *testing.T) {
	report := testScanReport(true, nil)
	var stdout, stderr bytes.Buffer

	code := runScan(context.Background(), []string{"--root", `C:\data`, "--json", "-"}, &stdout, &stderr, fixedScanner{report: report})

	if code != 0 {
		t.Fatalf("runScan() exit code = %d, stderr: %s", code, stderr.String())
	}
	var got model.ScanReport
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not a scan report: %v; output: %s", err, stdout.String())
	}
	if got.Schema != model.ScanSchema || !got.Complete {
		t.Fatalf("report = %#v, want complete %s report", got, model.ScanSchema)
	}
}

func TestRunScanWritesIncompleteReportAndReturnsThree(t *testing.T) {
	errorsInReport := []model.ScanError{{Path: `C:\restricted`, Operation: "enumerate", Code: "access_denied"}}
	var stdout, stderr bytes.Buffer

	code := runScan(context.Background(), []string{"--root", `C:\data`, "--json", "-"}, &stdout, &stderr, fixedScanner{report: testScanReport(false, errorsInReport)})

	if code != 3 {
		t.Fatalf("runScan() exit code = %d, want 3; stderr: %s", code, stderr.String())
	}
	var got model.ScanReport
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("partial stdout is not a scan report: %v; output: %s", err, stdout.String())
	}
	if got.Complete || len(got.Errors) != 1 || got.Errors[0].Code != "access_denied" {
		t.Fatalf("partial report = %#v, want incomplete with access_denied coverage gap", got)
	}
}

func TestRunScanRejectsUnknownFlagsAndMissingRoot(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown flag", args: []string{"--root", `C:\data`, "--json", "-", "--typo"}},
		{name: "missing root", args: []string{"--json", "-"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runScan(context.Background(), test.args, &stdout, &stderr, fixedScanner{})
			if code != 2 || !strings.Contains(stderr.String(), "Usage:") {
				t.Fatalf("runScan() = %d, stderr %q; want code 2 and usage", code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestRunScanReturnsFourWhenOutputFails(t *testing.T) {
	var stderr bytes.Buffer
	want := errors.New("disk full")
	code := runScan(context.Background(), []string{"--root", `C:\data`, "--json", "-"}, errorWriter{err: want}, &stderr, fixedScanner{report: testScanReport(true, nil)})
	if code != 4 {
		t.Fatalf("runScan() exit code = %d, want 4; stderr: %s", code, stderr.String())
	}
}

func TestRunScanReturnsTwoWhenPlatformIsUnsupported(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runScan(context.Background(), []string{"--root", `C:\data`, "--json", "-"}, &stdout, &stderr, fixedScanner{err: scanner.ErrUnsupportedPlatform})
	if code != 2 {
		t.Fatalf("runScan() exit code = %d, want 2; stderr: %s", code, stderr.String())
	}
}

func TestRunScanWritesReportFile(t *testing.T) {
	output := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(output, []byte("previous report"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runScan(context.Background(), []string{"--root", `C:\data`, "--json", output}, &stdout, &stderr, fixedScanner{report: testScanReport(true, nil)})
	if code != 0 {
		t.Fatalf("runScan() exit code = %d, stderr: %s", code, stderr.String())
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("report file not written: %v", err)
	}
	var got model.ScanReport
	if err := json.Unmarshal(data, &got); err != nil || got.Schema != model.ScanSchema {
		t.Fatalf("report file is invalid: error = %v, contents = %s", err, data)
	}
}

type fixedScanner struct {
	report model.ScanReport
	err    error
}

func (s fixedScanner) Scan(context.Context, string) (model.ScanReport, error) {
	return s.report, s.err
}

func testScanReport(complete bool, reportErrors []model.ScanError) model.ScanReport {
	return model.ScanReport{
		Schema:      model.ScanSchema,
		StartedAt:   time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC),
		CompletedAt: time.Date(2026, time.October, 20, 0, 0, 1, 0, time.UTC),
		Complete:    complete,
		Nodes:       []model.Node{},
		Errors:      reportErrors,
	}
}

type errorWriter struct{ err error }

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }
