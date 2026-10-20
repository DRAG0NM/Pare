# Pare Read-Only Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a read-only Windows CLI that enumerates a selected directory tree into a versioned report and exposes measurable coverage gaps.

**Architecture:** A small Go CLI calls a scanner interface. The first implementation is a Windows directory-enumeration backend; model and JSON schema are platform-neutral. Rules, plans, UI, MFT parsing and write operations remain outside this first implementation.

**Tech Stack:** Go, Windows directory APIs, Go standard library, `go test` and `go vet`.

**Spec:** `docs/design.md`, sections 3, 4 and 8.

## Global Constraints

- The first scanner is read-only and never follows reparse points.
- Access and I/O failures remain visible as incomplete coverage; they are never reported as empty directories.
- File identity is scoped by volume and full available file ID; missing IDs are not merged.
- JSON uses schema `pare.scan/1`; 64-bit values must preserve integer precision.
- No raw scan output, personal path inventory, logs or credentials enter Git.
- MFT parsing, cleanup rules, plans and write operations are excluded from this plan.

## Review Focus

- Reparse points and junctions: scanner must record the entry and skip its target.
- Access denied or a directory changing during enumeration: report incomplete coverage and continue only where safe.
- Cancellation during a large tree: stop promptly and return an explicitly incomplete report.
- Long paths and Unicode names: preserve names without truncation or normalization collisions.
- Duplicate IDs or IDs unavailable from the current API: preserve path entries and avoid false identity merges.

---

### Task 1: Bootstrap the Go CLI

**Files:**
- Create: `go.mod`
- Create: `cmd/pare/main.go`
- Create: `internal/cli/run.go`
- Test: `internal/cli/run_test.go`

**Interfaces:**
- Produce `func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int`.
- `main` passes `os.Args[1:]`, `os.Stdout`, and `os.Stderr` to `cli.Run` and exits with its result.

- [ ] **Step 1: Write the failing CLI tests**

Add tests for `--version` returning exit code 0 with a stable version line, and for an unknown command returning a non-zero code with usage on stderr.

- [ ] **Step 2: Run the focused tests**

Run: `go test ./internal/cli -run 'TestRun(Version|UnknownCommand)' -v`
Expected: FAIL because `Run` is not implemented.

- [ ] **Step 3: Implement the minimal CLI dispatcher**

Create the module `github.com/DRAG0NM/Pare`. Implement only `--version`, usage, and unknown-command handling; do not add scan behavior in this task.

- [ ] **Step 4: Run the focused tests and full static checks**

Run: `go test ./internal/cli -run 'TestRun(Version|UnknownCommand)' -v`
Expected: PASS.

Run: `go test ./...`
Expected: PASS.

Run: `go vet ./...`
Expected: no diagnostics.

- [ ] **Step 5: Commit**

Commit as `feat: bootstrap pare cli`.

### Task 2: Define the scan report model

**Files:**
- Create: `internal/model/scan.go`
- Test: `internal/model/scan_test.go`

**Interfaces:**
- `ScanReport` contains `Schema`, `Volume`, `StartedAt`, `CompletedAt`, `Complete`, `Nodes`, and `Errors`.
- `Node` contains `Path`, `ParentPath`, `Kind`, `FileID`, `LogicalSize`, `AllocationSize`, `Attributes`, and `LastWriteTime`.
- `ScanError` contains `Path`, `Operation`, and `Code`; it must not store an unbounded raw OS message.
- IDs and sizes serialize as decimal strings in JSON.

- [ ] **Step 1: Write failing serialization tests**

Test schema value `pare.scan/1`, round-trip of a size greater than `2^53`, empty and non-empty error lists, and rejection of a missing schema value.

- [ ] **Step 2: Run the model tests**

Run: `go test ./internal/model -v`
Expected: FAIL because the types and validation are not implemented.

- [ ] **Step 3: Implement the model and JSON validation**

Use explicit JSON marshal/unmarshal types for decimal-string IDs and sizes. Reject unsupported schema versions and malformed integer strings.

- [ ] **Step 4: Run tests and static checks**

Run: `go test ./internal/model -v`
Expected: PASS.

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

Commit as `feat: define scan report schema`.

### Task 3: Add the scanner contract and portable test backend

**Files:**
- Create: `internal/scanner/scanner.go`
- Create: `internal/scanner/walk.go`
- Test: `internal/scanner/walk_test.go`

**Interfaces:**
- `type Scanner interface { Scan(context.Context, string) (model.ScanReport, error) }`.
- `WalkScanner` is a test backend built on `os.ReadDir`; it is not selected as the Windows production backend.
- The scanner returns incomplete reports for per-directory errors and returns `context.Canceled` when canceled.

- [ ] **Step 1: Write failing tests**

Test nested directories, empty directory, file metadata, cancellation before traversal, and a simulated child-directory read failure through an injected directory reader.

- [ ] **Step 2: Run the scanner tests**

Run: `go test ./internal/scanner -run 'TestWalkScanner' -v`
Expected: FAIL because the scanner contract is not implemented.

- [ ] **Step 3: Implement the test backend**

Use a small injected `ReadDir` function so error cases are deterministic. Do not follow symlinks; mark unsupported or unreadable entries in `Errors` and set `Complete` to false.

- [ ] **Step 4: Run scanner tests**

Run: `go test ./internal/scanner -run 'TestWalkScanner' -v`
Expected: PASS.

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

Commit as `feat: define scanner contract and test backend`.

### Task 4: Implement Windows batched directory enumeration

**Files:**
- Create: `internal/scanner/windows.go`
- Test: `internal/scanner/windows_test.go`
- Modify: `internal/scanner/scanner.go`

**Interfaces:**
- `NewWindowsScanner() Scanner` returns the Windows production scanner.
- Each directory is opened with directory-list access and backup semantics; enumeration loops through every returned buffer until completion.
- A reparse-point entry is emitted but its target is not opened.
- API errors map to stable `ScanError.Code` values, including access denied, changed entry, unsupported information class, and generic I/O failure.

- [ ] **Step 1: Write failing Windows tests**

Create a temporary tree and test regular files, nested directories, Unicode names, a path longer than `MAX_PATH`, and reparse-point non-traversal. Add pure record-parser tests for invalid offsets and duplicate IDs; duplicate IDs must retain each path node. Add error-mapping tests for access denied, changed entry and generic I/O failure. Skip only the reparse test when the OS denies creating a test link; never skip the other cases.

- [ ] **Step 2: Cross-compile the test package**

Run: `$env:GOOS='windows'; $env:GOARCH='amd64'; go test -c ./internal/scanner -o $env:TEMP\pare-scanner.test.exe`
Expected: FAIL until the Windows implementation and build tags exist.

- [ ] **Step 3: Implement the Win32 backend**

Use `CreateFileW` with directory-list access and `FILE_FLAG_BACKUP_SEMANTICS`, then loop over `GetFileInformationByHandleEx(FileIdBothDirectoryInfo)` buffers. Validate each record offset and length before reading names. Use a platform-specific file and keep non-Windows builds compiling with an explicit unsupported-backend error.

- [ ] **Step 4: Run Windows tests and static checks**

Run on Windows: `go test ./internal/scanner -v`
Expected: PASS, with only the reparse creation test conditionally skipped when the OS denies link creation.

Run: `go test ./...`
Expected: PASS.

Run: `go vet ./...`
Expected: no diagnostics.

- [ ] **Step 5: Commit**

Commit as `feat: add windows directory scanner`.

### Task 5: Expose `scan` JSON output

**Files:**
- Create: `internal/report/json.go`
- Test: `internal/report/json_test.go`
- Modify: `internal/cli/run.go`
- Test: `internal/cli/run_test.go`

**Interfaces:**
- Command: `pare scan --root <path> --json <path>`; `--json -` writes to stdout.
- Exit codes: 0 complete, 2 invalid arguments or unsupported platform, 3 incomplete scan, 4 fatal scan/output error.
- JSON top-level schema remains `pare.scan/1`.

- [ ] **Step 1: Write failing command tests**

Test successful output, incomplete scan exit code 3, unknown flags, missing root, and output write failure. Verify partial output still contains `complete: false` and the coverage errors.

- [ ] **Step 2: Run focused tests**

Run: `go test ./internal/cli ./internal/report -v`
Expected: FAIL because the command and reporter are not implemented.

- [ ] **Step 3: Implement reporting and command wiring**

Use `encoding/json`; write output through the provided writer or an atomically replaced file. Do not write to the scanned root by default.

- [ ] **Step 4: Run full tests and checks**

Run: `go test ./...`
Expected: PASS.

Run: `go vet ./...`
Expected: no diagnostics.

- [ ] **Step 5: Commit**

Commit as `feat: expose read-only scan command`.

### Task 6: Add benchmark and usage documentation

**Files:**
- Create: `internal/scanner/benchmark_test.go`
- Modify: `README.md`
- Modify: `docs/design.md`
- Modify: `docs/development-plan.md`

**Interfaces:**
- Benchmark: `BenchmarkDirectoryScanner` builds a temporary synthetic tree and reports elapsed time, entries per second, and allocations.
- Benchmark output must record Go version, OS, volume type, cache condition, scope and entry count in the report.

- [ ] **Step 1: Add benchmark fixture and command documentation**

The fixture must not read or copy any user files. It creates and removes only its own temporary directory.

- [ ] **Step 2: Run benchmark once**

Run: `go test ./internal/scanner -run '^$' -bench '^BenchmarkDirectoryScanner$' -benchtime=1x -benchmem`
Expected: completes and reports entries, time and allocations; performance target remains unset until repeatable local measurements exist.

- [ ] **Step 3: Verify all checks**

Run: `go test ./...`
Expected: PASS.

Run: `go vet ./...`
Expected: no diagnostics.

- [ ] **Step 4: Commit**

Commit as `test: add scanner benchmark and usage docs`.
