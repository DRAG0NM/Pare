# Pare

Pare is a small Windows disk-usage scanner built around one rule: show what was observed, including what could not be read.

The current slice is a CLI. `pare scan` walks a chosen directory tree, records file metadata and coverage gaps, and writes a versioned JSON report. `pare plan` matches a small set of built-in cleanup rules against that report and writes a per-file plan with the evidence behind each candidate. `pare apply` moves a reviewed plan's executable files into a same-volume quarantine, and `pare undo` moves them back. Pare never deletes files.

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

## Apply and undo

```powershell
.\pare.exe apply --plan '.\plan.json'
.\pare.exe apply --plan '.\plan.json' --confirm 'sha256:...'
.\pare.exe undo --manifest 'C:\.pare-quarantine\20261023-141444-d242abdf9fd5\manifest.json'
```

Repeat `--item <path>` to quarantine only those items; every selected path must be an executable item of the plan, or the run is rejected before anything moves. Without `--item`, all executable items are used.

Without `--confirm`, `apply` only prints how many files the plan would move and the digest to confirm with; nothing is moved. With the matching digest it verifies the plan again (digest and expiry) and handles each executable item in turn:

1. Record the item as `intent` in the manifest. If the manifest cannot be written, the run stops before moving anything else.
2. Check that no parent directory is a reparse point.
3. Open the file with no sharing, so no other process can use, replace or delete it meanwhile, and compare file ID, size, last write time and attributes with the plan on that handle.
4. Rename the file through the same handle into the batch directory. The rename never replaces an existing file and cannot cross volumes.

Files that fail a check are `skipped` and stay where they are. The quarantine defaults to `\.pare-quarantine` at the root of the files' volume; `--quarantine` chooses another directory on the same volume. Each run creates a batch directory holding `manifest.json` (schema `pare.execution/1`) and the moved files.

`undo` restores `quarantined` items, and `intent` items left by an interrupted run. It recreates missing parent directories, never replaces a file at the original path, and records such items as `conflict`; running `undo` again after clearing the path restores them.

Exit codes: 0 every item moved (or restored), 2 invalid arguments or missing confirmation, 3 some items skipped, failed or in conflict, 4 plan or manifest rejected or unreadable.

## Current limits

- The Windows backend uses batched directory enumeration with 8 directories read in parallel; nodes are sorted by path, so output does not depend on scheduling. It does not read the NTFS MFT. On the development machine a warm-cache scan of D: (about 326,000 entries) took 1.7 s including the JSON write; cold-cache time is not measured.
- Reparse points are listed and their targets are not traversed.
- Paths remain separate entries even when they share a file ID, as with hard links. The internal `tree` package (used by the coming desktop UI) totals subtrees per path and counts each file ID once only in the whole-tree total; the CLI does not print either total yet.
- The scan report's volume serial comes from the root directory handle. On this machine an EFS-encrypted `AppData\Local` reported a different serial from the files inside it, so plan probes do not compare volume serials.
- Quarantine does not free space on the volume. There is no purge command yet; emptying a batch is a manual, separate decision.
- Cross-volume quarantine, system maintenance, an Agent adapter and a desktop interface are not included.
- The reparse-point integration test uses a symlink or junction; it skips only when Windows denies creation of both.

## Development

```powershell
go test ./internal/scanner -run '^$' -bench '^BenchmarkDirectoryScanner$' -benchtime=1x -benchmem
```

The benchmark creates its own temporary directory tree. Its log records the Go version, OS, volume type and filesystem, cache condition, scope, and entry count. Measurements are local observations; Pare has no performance target yet.

The design and staged roadmap are in [docs/design.md](docs/design.md) and [docs/development-plan.md](docs/development-plan.md). Third-party dependency terms are listed in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## Project notes

Pare is maintained by DRAG0NM as a personal project. The core is Go, and its Windows API wrapper dependency is `golang.org/x/sys` under BSD-3-Clause. The repository is private and the project license has not been selected yet; see [third-party notices](THIRD_PARTY_NOTICES.md) for the dependency notice.
