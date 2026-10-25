package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/DRAG0NM/Pare/internal/executor"
	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/plan"
	"github.com/DRAG0NM/Pare/internal/probes"
	"github.com/DRAG0NM/Pare/internal/report"
	"github.com/DRAG0NM/Pare/internal/rules"
	"github.com/DRAG0NM/Pare/internal/scanner"
	"github.com/DRAG0NM/Pare/internal/serve"
)

const version = "0.3.0"

const usage = "Usage: pare <command> [options]\n\nCommands:\n  scan    Scan a directory tree and write a JSON report\n  plan    Match cleanup rules against a scan report and write a JSON plan\n  apply   Move a plan's executable files into quarantine\n  undo    Restore files from a quarantine manifest\n  serve   Serve agent hosts over line-delimited JSON on stdin/stdout\n"

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintf(stdout, "pare %s\n", version)
		return 0
	}
	if len(args) > 0 && args[0] == "scan" {
		return runScan(ctx, args[1:], stdout, stderr, scanner.NewWindowsScanner())
	}
	if len(args) > 0 && args[0] == "plan" {
		return runPlan(ctx, args[1:], stdout, stderr, probes.NewWindowsProber(), time.Now())
	}
	if len(args) > 0 && args[0] == "apply" {
		return runApply(ctx, args[1:], stdout, stderr, time.Now())
	}
	if len(args) > 0 && args[0] == "undo" {
		return runUndo(ctx, args[1:], stdout, stderr, time.Now())
	}
	if len(args) == 1 && args[0] == "serve" {
		if err := serve.New(serve.Options{Version: version}).Run(ctx, os.Stdin, stdout); err != nil {
			fmt.Fprintln(stderr, err)
			return 4
		}
		return 0
	}

	fmt.Fprint(stderr, usage)
	return 2
}

func runScan(ctx context.Context, args []string, stdout, stderr io.Writer, backend scanner.Scanner) int {
	flags := flag.NewFlagSet("scan", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, scanUsage) }
	root := flags.String("root", "", "directory tree to scan")
	jsonPath := flags.String("json", "", "report path, or - for stdout")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || *root == "" || *jsonPath == "" {
		fmt.Fprint(stderr, scanUsage)
		return 2
	}

	scanReport, scanErr := backend.Scan(ctx, *root)
	if errors.Is(scanErr, scanner.ErrUnsupportedPlatform) {
		fmt.Fprintln(stderr, "scan is supported only on Windows")
		return 2
	}
	if scanReport.Schema == "" {
		fmt.Fprintln(stderr, "scan failed before a report was produced")
		return 4
	}

	var err error
	if *jsonPath == "-" {
		err = report.WriteJSON(stdout, scanReport)
	} else {
		err = writeJSONFile(*jsonPath, scanReport)
	}
	if err != nil {
		fmt.Fprintf(stderr, "write report: %v\n", err)
		return 4
	}
	if !scanReport.Complete {
		return 3
	}
	if scanErr != nil {
		fmt.Fprintln(stderr, "scan failed")
		return 4
	}
	return 0
}

func runPlan(ctx context.Context, args []string, stdout, stderr io.Writer, prober probes.Prober, now time.Time) int {
	flags := flag.NewFlagSet("plan", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, planUsage) }
	scanPath := flags.String("scan", "", "scan report path")
	jsonPath := flags.String("json", "", "plan path, or - for stdout")
	ttl := flags.Duration("ttl", 24*time.Hour, "how long the plan stays valid")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || *scanPath == "" || *jsonPath == "" || *ttl <= 0 {
		fmt.Fprint(stderr, planUsage)
		return 2
	}

	data, err := os.ReadFile(*scanPath)
	if err != nil {
		fmt.Fprintf(stderr, "read scan report: %v\n", err)
		return 4
	}
	var scanReport model.ScanReport
	if err := json.Unmarshal(data, &scanReport); err != nil {
		fmt.Fprintf(stderr, "parse scan report: %v\n", err)
		return 4
	}
	result, err := plan.Build(ctx, scanReport, rules.Builtin(), prober, now, *ttl)
	if err != nil {
		fmt.Fprintf(stderr, "build plan: %v\n", err)
		return 4
	}

	if *jsonPath == "-" {
		err = report.WriteJSON(stdout, result)
	} else {
		err = writeJSONFile(*jsonPath, result)
	}
	if err != nil {
		fmt.Fprintf(stderr, "write plan: %v\n", err)
		return 4
	}
	return 0
}

func writeJSONFile(path string, document any) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".pare-report-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := report.WriteJSON(temporary, document); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

const scanUsage = "Usage: pare scan --root <path> --json <path|->\n"

const planUsage = "Usage: pare plan --scan <path> --json <path|-> [--ttl <duration>]\n"

const applyUsage = "Usage: pare apply --plan <path> [--confirm <digest>] [--item <path>]... [--quarantine <dir>]\n"

const undoUsage = "Usage: pare undo --manifest <path>\n"

func runApply(ctx context.Context, args []string, stdout, stderr io.Writer, now time.Time) int {
	flags := flag.NewFlagSet("apply", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, applyUsage) }
	planPath := flags.String("plan", "", "plan path")
	confirm := flags.String("confirm", "", "plan digest, copied from the reviewed plan")
	root := flags.String("quarantine", "", "quarantine directory on the files' volume")
	var selected []string
	flags.Func("item", "path of one executable item to quarantine; repeat to select several (default: all)", func(path string) error {
		selected = append(selected, path)
		return nil
	})
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || *planPath == "" {
		fmt.Fprint(stderr, applyUsage)
		return 2
	}

	data, err := os.ReadFile(*planPath)
	if err != nil {
		fmt.Fprintf(stderr, "read plan: %v\n", err)
		return 4
	}
	var reviewed model.Plan
	if err := json.Unmarshal(data, &reviewed); err != nil {
		fmt.Fprintf(stderr, "parse plan: %v\n", err)
		return 4
	}
	if *confirm == "" {
		fmt.Fprintf(stdout, "Plan %s has %d executable files (%s bytes allocated).\nNothing was moved. To quarantine them, run again with --confirm %s\n",
			*planPath, reviewed.Totals.Executable, strconv.FormatUint(uint64(reviewed.Totals.ExecutableAllocation), 10), reviewed.Digest)
		return 2
	}
	if *root == "" {
		for _, item := range reviewed.Items {
			if item.Executable {
				*root = executor.DefaultQuarantineRoot(item.Path)
				break
			}
		}
	}

	execution, manifest, err := executor.Apply(ctx, reviewed, selected, *confirm, *root, now)
	if manifest != "" {
		fmt.Fprintf(stdout, "Manifest: %s\n", manifest)
	}
	fmt.Fprintf(stdout, "Quarantined %d, skipped %d, failed %d\n",
		execution.Count(model.StateQuarantined), execution.Count(model.StateSkipped), execution.Count(model.StateFailed))
	if errors.Is(err, executor.ErrUnconfirmed) || errors.Is(err, executor.ErrNotExecutable) {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "apply: %v\n", err)
		return 4
	}
	if execution.Count(model.StateQuarantined) != len(execution.Items) {
		return 3
	}
	return 0
}

func runUndo(ctx context.Context, args []string, stdout, stderr io.Writer, now time.Time) int {
	flags := flag.NewFlagSet("undo", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, undoUsage) }
	manifest := flags.String("manifest", "", "manifest path written by pare apply")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || *manifest == "" {
		fmt.Fprint(stderr, undoUsage)
		return 2
	}

	execution, err := executor.Undo(ctx, *manifest, now)
	fmt.Fprintf(stdout, "Restored %d, conflicts %d, still quarantined %d\n",
		execution.Count(model.StateRestored), execution.Count(model.StateConflict),
		execution.Count(model.StateQuarantined)+execution.Count(model.StateIntent))
	if err != nil {
		fmt.Fprintf(stderr, "undo: %v\n", err)
		return 4
	}
	if execution.Count(model.StateConflict)+execution.Count(model.StateQuarantined)+execution.Count(model.StateIntent) != 0 {
		return 3
	}
	return 0
}
