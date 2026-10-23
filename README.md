# Pare

Pare is a small Windows disk-usage scanner built around one rule: show what was observed, including what could not be read.

The current slice is a read-only CLI. `pare scan` walks a chosen directory tree, records file metadata and coverage gaps, and writes a versioned JSON report. `pare plan` matches a small set of built-in cleanup rules against that report and writes a per-file plan with the evidence behind each candidate. Pare has no cleanup or write commands yet.

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

## Plan

```powershell
.\pare.exe plan --scan '.\downloads-scan.json' --json '.\plan.json'
```

The plan uses schema `pare.plan/1`. Each item names one file, the rule it matched, its risk level and a list of evidence entries with status `pass`, `fail`, `unknown` or `error`. An item is `executable` only when every required piece of evidence passes:

| Evidence | Checks |
|---|---|
| `path_rule` | Exactly one rule matches the path |
| `attributes` | Not a system file |
| `file_identity` | The scan has a file ID and it appears at only one path (no hard links) |
| `age` | Last write is at least the rule's minimum age and not in the future |
| `unchanged` | File ID, size and last write time still match the scan |
| `unlocked` | An exclusive read open succeeds, so no other process holds the file |

The last two are live probes; they run only for items whose other evidence passes and never read or write file contents. Totals count executable bytes once per file ID. `--ttl` sets how long the plan is valid (default `24h`). The `digest` field is a SHA-256 of the plan with an empty digest; it detects edits but is not a signature.

Built-in rules (`builtin/1`, all risk E1): per-user `AppData\Local\Temp` older than 7 days, and the npm, pip and Go build caches older than 30 days.

Exit codes: 0 plan written, 2 invalid arguments, 4 scan report unreadable or plan output failed.

## Current limits

- The Windows backend uses batched directory enumeration. It does not read the NTFS MFT, so a full-volume scan can take longer than specialized tools.
- Reparse points are listed and their targets are not traversed.
- Paths remain separate entries even when they share a file ID, as with hard links. The current report does not calculate deduplicated reclaimable space.
- The scan report's volume serial comes from the root directory handle. On this machine an EFS-encrypted `AppData\Local` reported a different serial from the files inside it, so plan probes do not compare volume serials.
- This release does not include quarantine, undo, an Agent adapter, or a desktop interface. Plans are for review only.
- The reparse-point integration test uses a symlink or junction; it skips only when Windows denies creation of both.

## Development

```powershell
go test ./internal/scanner -run '^$' -bench '^BenchmarkDirectoryScanner$' -benchtime=1x -benchmem
```

The benchmark creates its own temporary directory tree. Its log records the Go version, OS, volume type and filesystem, cache condition, scope, and entry count. Measurements are local observations; Pare has no performance target yet.

The design and staged roadmap are in [docs/design.md](docs/design.md) and [docs/development-plan.md](docs/development-plan.md). Third-party dependency terms are listed in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## Project notes

Pare is maintained by DRAG0NM as a personal project. The core is Go, and its Windows API wrapper dependency is `golang.org/x/sys` under BSD-3-Clause. The repository is private and the project license has not been selected yet; see [third-party notices](THIRD_PARTY_NOTICES.md) for the dependency notice.
