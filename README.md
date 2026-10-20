# Pare

Pare is a small Windows disk-usage scanner built around one rule: show what was observed, including what could not be read.

The first working slice is a read-only CLI. It walks a chosen directory tree, records file metadata and coverage gaps, and writes a versioned JSON report. It does not decide what is safe to remove and has no cleanup or write commands.

## Build

Pare currently targets Windows. Go 1.27 or newer is required.

```powershell
go test ./...
go vet ./...
go build -o pare.exe ./cmd/pare
```

## Scan

```powershell
.\pare.exe scan --root 'C:\Users\me\Downloads' --json '.\downloads-scan.json'
```

Use `--json -` to write the report to standard output. File output is written through a temporary file in the destination directory and then renamed into place. The report uses schema `pare.scan/1`; file IDs and byte counts that could exceed JavaScript's exact integer range are decimal strings.

Exit codes:

| Code | Meaning |
|---:|---|
| 0 | Scan completed with no reported coverage gaps |
| 2 | Invalid arguments or unsupported platform |
| 3 | Report written, but the scan has coverage gaps |
| 4 | Scan or report output failed |

A nonzero exit code does not mean a report is missing: code 3 deliberately accompanies a partial report. Check its `complete` and `errors` fields before using it.

## Current limits

- The Windows backend uses batched directory enumeration. It does not read the NTFS MFT, so a full-volume scan can take longer than specialized tools.
- Reparse points are listed and their targets are not traversed.
- Paths remain separate entries even when they share a file ID, as with hard links. The current report does not calculate deduplicated reclaimable space.
- This release does not include cleanup rules, deletion plans, quarantine, undo, an Agent adapter, or a desktop interface.
- The reparse-point integration test uses a symlink or junction; it skips only when Windows denies creation of both.

## Development

```powershell
go test ./internal/scanner -run '^$' -bench '^BenchmarkDirectoryScanner$' -benchtime=1x -benchmem
```

The benchmark creates its own temporary directory tree. Its log records the Go version, OS, volume type and filesystem, cache condition, scope, and entry count. Measurements are local observations; Pare has no performance target yet.

The design and staged roadmap are in [docs/design.md](docs/design.md) and [docs/development-plan.md](docs/development-plan.md). Third-party dependency terms are listed in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## Project notes

Pare is maintained by DRAG0NM as a personal project. The core is Go, and its Windows API wrapper dependency is `golang.org/x/sys` under BSD-3-Clause. The repository is private and the project license has not been selected yet; see [third-party notices](THIRD_PARTY_NOTICES.md) for the dependency notice.
