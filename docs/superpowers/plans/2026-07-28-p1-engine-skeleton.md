# P1 Engine Skeleton Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `purgefs scan PATH` walks a directory tree, aggregates subtree sizes, and prints a size summary — the reusable engine core all later phases build on.

**Architecture:** A frontend-agnostic `internal/engine` package builds a tree of `Entry` nodes with aggregated sizes (`Walk`), then `Scan` wraps that into a `Report` with counts. The `cmd/scan` cobra command calls `engine.Scan` and prints a human-readable summary. No rules, trash, or TUI yet — those are P2–P5.

**Tech Stack:** Go, cobra (already wired). Standard library only for the engine (`os`, `io/fs`, `path/filepath`).

## Global Constraints

- Go module `github.com/seeminglyjs/PurgeFs`; `go.mod` floor `go 1.24`.
- CLI framework: `cobra`. Engine packages use standard library only.
- Engine lives under `internal/engine`; frontends (`cmd/`) depend on the engine, never the reverse.
- Do NOT follow symlinks — record the link's own size, never descend into or delete targets.
- Traversal continues past per-path errors (permission, etc.); errors are collected and returned, never abort the whole walk.
- Keep dependency-light; justify any new module (P1 adds none).
- Commits: do NOT add Claude as co-author. Conventional-commit prefixes (`feat`/`test`/`refactor`/`docs`).

## File Structure

- Create `internal/engine/model.go` — `Entry`, `Report`, `WalkError` types (no logic).
- Create `internal/engine/walk.go` — `Walk(root)`: build the tree with aggregated sizes, skip symlinks, collect errors.
- Create `internal/engine/scan.go` — `Scan(root)`: wrap `Walk` into a `Report` with file/dir counts.
- Create `internal/engine/walk_test.go`, `scan_test.go`, `testhelpers_test.go`.
- Modify `cmd/scan.go` — replace the stub with a real call to `engine.Scan` + human-readable output.
- Create `cmd/format.go` — `humanBytes(n)` helper.
- Create `cmd/format_test.go`.

---

### Task 1: Engine model + Walk

**Files:**
- Create: `internal/engine/model.go`
- Create: `internal/engine/walk.go`
- Test: `internal/engine/walk_test.go`, `internal/engine/testhelpers_test.go`

**Interfaces:**
- Consumes: nothing (first task).
- Produces:
  - `type Entry struct { Path string; Size int64; IsDir bool; ModTime int64; Children []*Entry }`
  - `type WalkError struct { Path string; Err error }`
  - `func Walk(root string) (*Entry, []WalkError, error)` — root `*Entry` with `Size` aggregated over the subtree; symlinks recorded as leaf entries (own size, no children); per-path errors in `[]WalkError`; the final `error` is non-nil only if `root` itself cannot be `Lstat`'d.

- [ ] **Step 1: Write the failing tests**

Create `internal/engine/testhelpers_test.go`:

```go
package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func childByPath(e *Entry, path string) *Entry {
	for _, c := range e.Children {
		if c.Path == path {
			return c
		}
	}
	return nil
}
```

Create `internal/engine/walk_test.go`:

```go
package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWalkAggregatesSizes(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), "abc")   // 3 bytes
	mustMkdir(t, filepath.Join(root, "sub"))
	mustWrite(t, filepath.Join(root, "sub", "b.txt"), "hello") // 5 bytes

	e, werrs, err := Walk(root)
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}
	if len(werrs) != 0 {
		t.Fatalf("unexpected walk errors: %v", werrs)
	}
	if !e.IsDir {
		t.Errorf("root should be a dir")
	}
	if e.Size != 8 {
		t.Errorf("root aggregated size = %d, want 8", e.Size)
	}
	if len(e.Children) != 2 {
		t.Errorf("root children = %d, want 2", len(e.Children))
	}
	sub := childByPath(e, filepath.Join(root, "sub"))
	if sub == nil || sub.Size != 5 {
		t.Errorf("sub size = %v, want 5", sub)
	}
}

func TestWalkSkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real")
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(target, "big.txt"), "0123456789") // 10 bytes
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	e, _, err := Walk(root)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	linkEntry := childByPath(e, link)
	if linkEntry == nil {
		t.Fatal("link entry missing")
	}
	if linkEntry.IsDir {
		t.Error("symlink must not be treated as a directory")
	}
	if len(linkEntry.Children) != 0 {
		t.Error("symlink must not be descended into")
	}
}

func TestWalkCollectsPermissionErrors(t *testing.T) {
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	mustMkdir(t, locked)
	mustWrite(t, filepath.Join(locked, "secret.txt"), "xyz")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	_, werrs, err := Walk(root)
	if err != nil {
		t.Fatalf("Walk should not fail on a per-dir permission error: %v", err)
	}
	if len(werrs) == 0 {
		t.Error("expected a WalkError for the unreadable directory")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/engine/ -run TestWalk -v`
Expected: FAIL — `undefined: Walk` / `undefined: Entry`.

- [ ] **Step 3: Write the model types**

Create `internal/engine/model.go`:

```go
// Package engine walks a filesystem tree, aggregates sizes, and (in later
// phases) matches junk rules. It is frontend-agnostic: CLI, TUI, and a future
// GUI all consume it.
package engine

// Entry is a file or directory discovered during a scan.
type Entry struct {
	Path     string
	Size     int64  // bytes; for a directory, the aggregated size of its subtree
	IsDir    bool
	ModTime  int64  // unix seconds; used for staleness in later phases
	Children []*Entry
}

// WalkError records a path that could not be processed during a walk.
type WalkError struct {
	Path string
	Err  error
}

// Report is the result of a Scan.
type Report struct {
	Root      *Entry
	TotalSize int64
	FileCount int
	DirCount  int
}
```

- [ ] **Step 4: Write the Walk implementation**

Create `internal/engine/walk.go`:

```go
package engine

import (
	"os"
	"path/filepath"
)

// Walk traverses root and returns the root Entry with subtree sizes aggregated.
// Symlinks are recorded as leaf entries (their own size) and never followed.
// Per-path errors (e.g. permission denied) are collected in the returned slice
// and traversal continues. The error return is non-nil only when root itself
// cannot be stat'd.
func Walk(root string) (*Entry, []WalkError, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, nil, err
	}
	var werrs []WalkError
	e := walkEntry(root, info, &werrs)
	return e, werrs, nil
}

func walkEntry(path string, info os.FileInfo, werrs *[]WalkError) *Entry {
	e := &Entry{
		Path:    path,
		IsDir:   info.IsDir(),
		ModTime: info.ModTime().Unix(),
	}
	if !info.IsDir() {
		e.Size = info.Size()
		return e
	}

	dirents, err := os.ReadDir(path)
	if err != nil {
		*werrs = append(*werrs, WalkError{Path: path, Err: err})
		return e
	}

	for _, de := range dirents {
		childPath := filepath.Join(path, de.Name())
		ci, err := de.Info() // lstat-based: does not follow symlinks
		if err != nil {
			*werrs = append(*werrs, WalkError{Path: childPath, Err: err})
			continue
		}
		if ci.Mode()&os.ModeSymlink != 0 {
			child := &Entry{
				Path:    childPath,
				IsDir:   false,
				Size:    ci.Size(),
				ModTime: ci.ModTime().Unix(),
			}
			e.Children = append(e.Children, child)
			e.Size += child.Size
			continue
		}
		child := walkEntry(childPath, ci, werrs)
		e.Children = append(e.Children, child)
		e.Size += child.Size
	}
	return e
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/engine/ -run TestWalk -v`
Expected: PASS (all three `TestWalk*` tests).

- [ ] **Step 6: Commit**

```bash
git add internal/engine/model.go internal/engine/walk.go internal/engine/walk_test.go internal/engine/testhelpers_test.go
git commit -m "feat: add engine Walk with size aggregation and symlink/error handling"
```

---

### Task 2: Scan orchestration + counts

**Files:**
- Create: `internal/engine/scan.go`
- Test: `internal/engine/scan_test.go`

**Interfaces:**
- Consumes: `Walk`, `Entry`, `Report`, `WalkError` from Task 1.
- Produces:
  - `func Scan(root string) (*Report, []WalkError, error)` — builds a `Report` whose `TotalSize` equals the root entry's aggregated size, and `FileCount`/`DirCount` count every node in the subtree (the root directory is included in `DirCount`).

- [ ] **Step 1: Write the failing test**

Create `internal/engine/scan_test.go`:

```go
package engine

import (
	"path/filepath"
	"testing"
)

func TestScanCounts(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), "abc")   // file
	mustMkdir(t, filepath.Join(root, "sub"))            // dir
	mustWrite(t, filepath.Join(root, "sub", "b.txt"), "hello") // file

	r, werrs, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(werrs) != 0 {
		t.Fatalf("unexpected walk errors: %v", werrs)
	}
	if r.TotalSize != 8 {
		t.Errorf("TotalSize = %d, want 8", r.TotalSize)
	}
	if r.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", r.FileCount)
	}
	if r.DirCount != 2 { // root + sub
		t.Errorf("DirCount = %d, want 2", r.DirCount)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/engine/ -run TestScanCounts -v`
Expected: FAIL — `undefined: Scan`.

- [ ] **Step 3: Write the Scan implementation**

Create `internal/engine/scan.go`:

```go
package engine

// Scan walks root and produces a Report with aggregated size and node counts.
func Scan(root string) (*Report, []WalkError, error) {
	rootEntry, werrs, err := Walk(root)
	if err != nil {
		return nil, werrs, err
	}
	r := &Report{
		Root:      rootEntry,
		TotalSize: rootEntry.Size,
	}
	countEntries(rootEntry, r)
	return r, werrs, nil
}

func countEntries(e *Entry, r *Report) {
	if e.IsDir {
		r.DirCount++
	} else {
		r.FileCount++
	}
	for _, c := range e.Children {
		countEntries(c, r)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/engine/ -run TestScanCounts -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/engine/scan.go internal/engine/scan_test.go
git commit -m "feat: add engine Scan producing a Report with counts"
```

---

### Task 3: Human-readable byte formatting

**Files:**
- Create: `cmd/format.go`
- Test: `cmd/format_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `func humanBytes(n int64) string` — `0 B`, `512 B`, `1.0 KB`, `1.5 KB`, `1.0 MB`, `1.0 GB` (base-1024, one decimal above bytes).

- [ ] **Step 1: Write the failing test**

Create `cmd/format_test.go`:

```go
package cmd

import "testing"

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, c := range cases {
		if got := humanBytes(c.n); got != c.want {
			t.Errorf("humanBytes(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestHumanBytes -v`
Expected: FAIL — `undefined: humanBytes`.

- [ ] **Step 3: Write the implementation**

Create `cmd/format.go`:

```go
package cmd

import "fmt"

// humanBytes formats a byte count as a base-1024 human-readable string.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/ -run TestHumanBytes -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/format.go cmd/format_test.go
git commit -m "feat: add humanBytes formatting helper"
```

---

### Task 4: Wire scan command to the engine

**Files:**
- Modify: `cmd/scan.go`
- Test: `cmd/scan_test.go` (create)

**Interfaces:**
- Consumes: `engine.Scan`, `engine.Report`, `engine.Entry` (Tasks 1–2); `humanBytes` (Task 3).
- Produces: `func runScan(w io.Writer, path string) error` — scans `path`, writes a summary to `w`, and returns an error only when `engine.Scan`'s final error is non-nil. The cobra `RunE` delegates to `runScan(cmd.OutOrStdout(), path)`.

- [ ] **Step 1: Write the failing test**

Create `cmd/scan_test.go`:

```go
package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"os"
)

func TestRunScanReportsTotal(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("abc"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var buf bytes.Buffer
	if err := runScan(&buf, root); err != nil {
		t.Fatalf("runScan: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "3 B") {
		t.Errorf("output missing total size %q:\n%s", "3 B", out)
	}
	if !strings.Contains(out, "1 file") {
		t.Errorf("output missing file count:\n%s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestRunScanReportsTotal -v`
Expected: FAIL — `undefined: runScan`.

- [ ] **Step 3: Replace the scan command stub**

Replace the entire contents of `cmd/scan.go` with:

```go
package cmd

import (
	"fmt"
	"io"
	"sort"

	"github.com/seeminglyjs/PurgeFs/internal/engine"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan a path and report junk files/directories (no deletion)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) == 1 {
			path = args[0]
		}
		return runScan(cmd.OutOrStdout(), path)
	},
}

// runScan scans path and writes a size summary to w.
func runScan(w io.Writer, path string) error {
	report, werrs, err := engine.Scan(path)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Scanned %s\n", path)
	fmt.Fprintf(w, "Total: %s across %d files, %d dirs\n",
		humanBytes(report.TotalSize), report.FileCount, report.DirCount)

	// Top-level children, largest first.
	children := append([]*engine.Entry(nil), report.Root.Children...)
	sort.Slice(children, func(i, j int) bool {
		return children[i].Size > children[j].Size
	})
	for _, c := range children {
		fmt.Fprintf(w, "  %10s  %s\n", humanBytes(c.Size), c.Path)
	}

	if len(werrs) > 0 {
		fmt.Fprintf(w, "\nSkipped %d path(s) due to errors\n", len(werrs))
	}
	return nil
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/ -run TestRunScanReportsTotal -v`
Expected: PASS.

- [ ] **Step 5: Full build + test + manual smoke**

Run:
```bash
go build -o purgefs . && go test ./... && ./purgefs scan .
rm -f purgefs
```
Expected: build OK; all tests PASS; `scan .` prints a total and per-child sizes.

- [ ] **Step 6: Commit**

```bash
git add cmd/scan.go cmd/scan_test.go
git commit -m "feat: wire scan command to engine with size summary output"
```

---

## Self-Review

**1. Spec coverage (P1 scope only):**
- Walker + size aggregation → Task 1. ✓
- Report model → Tasks 1–2. ✓
- `scan` prints tree/summary (no TUI) → Task 4. ✓
- Tests → every task is TDD. ✓
- Symlink safety, error-skip → Task 1 tests + impl. ✓
- `ModTime` field for later staleness → present on `Entry` (Task 1), unused in P1 by design. ✓
- Out of P1 scope by design: rules (P2), trash/purge (P3), TUI (P4), undo/preset (P5).

**2. Placeholder scan:** No TBD/TODO/"handle edge cases" — every code and test step is complete. ✓

**3. Type consistency:** `Entry`, `Report`, `WalkError`, `Walk`, `Scan`, `humanBytes`, `runScan` names and signatures match across Tasks 1–4. `runScan(io.Writer, string)` is consumed by `RunE` and the test identically. ✓

## Notes

- Concurrency: spec mentions a "concurrent" walker. P1 implements a correct sequential walk first (correctness before optimization). A concurrent variant, if profiling justifies it, is a later optimization task and must preserve `Walk`'s signature and semantics.
- P2–P5 each get their own plan after P1 lands and is reviewed.
