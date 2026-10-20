package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/report"
	"github.com/DRAG0NM/Pare/internal/scanner"
)

const version = "0.1.0"

const usage = "Usage: pare <command> [options]\n\nCommands:\n  scan    Scan a directory tree and write a JSON report\n"

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintf(stdout, "pare %s\n", version)
		return 0
	}
	if len(args) > 0 && args[0] == "scan" {
		return runScan(ctx, args[1:], stdout, stderr, scanner.NewWindowsScanner())
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
		err = writeReportFile(*jsonPath, scanReport)
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

func writeReportFile(path string, scan model.ScanReport) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".pare-report-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := report.WriteJSON(temporary, scan); err != nil {
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
