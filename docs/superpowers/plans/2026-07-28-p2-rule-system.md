# P2 Rule System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Classify the scanned tree into junk categories (node_modules, build caches, __pycache__, .DS_Store) and show a per-category reclaimable-size summary in `scan` output.

**Architecture:** A `Rule` interface plus built-in rules live in `internal/engine`. A new `Classify` pass walks the existing `Report` tree (produced unchanged by P1's `Walk`) and groups matched entries by category. A matched directory (e.g. `node_modules`) is one reclaim unit whose size is already aggregated by P1; its children are not classified further. `scan` gains a category summary section below the existing size list.

**Tech Stack:** Go, standard library only for the engine. `cobra` already wired.

## Global Constraints

- Go module `github.com/seeminglyjs/PurgeFs`; `go.mod` floor `go 1.24`.
- `internal/engine` uses the standard library only — no new dependencies. Engine never imports `cmd/`.
- **Code comments MUST be in Korean (한국어 주석)** — every `.go` source and test file, doc comments and inline alike. Keep identifiers, API names, category strings, and error strings as-is.
- Do NOT add Claude / any AI as co-author or trailer on commits. Conventional-commit prefixes (`feat`/`test`/`docs`).
- P2 does not delete anything. Rules target build/cache/OS junk only — never user data.
- Do NOT modify P1's `Walk` — classification is a separate pass over the existing tree.

## Design Decisions (deviations from the spec's illustrative text)

- `Rule.Match` takes `*Entry` (not `fs.DirEntry`). The spec's `fs.DirEntry` signature assumed a during-walk design; we classify **after** the walk over the `Entry` tree, so `*Entry` is the right input.
- Built-in rules live in the `engine` package itself (not an `internal/engine/rules` subpackage) to avoid an import cycle (`Classify` needs the defaults; a subpackage would need `engine`'s `Rule`/`Entry`). Promote to a subpackage later only if the rule set grows large.
- Matched directories are still walked for size by P1's `Walk` (prune-during-walk is a later optimization). Correctness first, like P1's sequential walk.

## File Structure

- Create `internal/engine/rule.go` — `Rule` interface, `dirNameRule`/`fileNameRule` implementations, `DefaultRules()`, and the shared `matchRules` helper.
- Create `internal/engine/classify.go` — `CategoryGroup` type and `Classify(*Report, []Rule) []CategoryGroup`.
- Create `internal/engine/rule_test.go`, `internal/engine/classify_test.go`.
- Modify `cmd/scan.go` — append a category summary + reclaimable total to `runScan`.
- Modify `cmd/scan_test.go` — add a test asserting the category summary appears.

---

### Task 1: Rule interface + built-in rules

**Files:**
- Create: `internal/engine/rule.go`
- Test: `internal/engine/rule_test.go`

**Interfaces:**
- Consumes: `Entry` from P1 (`internal/engine/model.go`).
- Produces:
  - `type Rule interface { Match(e *Entry) (matched bool, category string, skipChildren bool) }`
  - `func DefaultRules() []Rule`
  - `func matchRules(rules []Rule, e *Entry) (category string, skipChildren bool, ok bool)` — tries rules in order, returns the first match.

- [ ] **Step 1: Write the failing tests**

Create `internal/engine/rule_test.go`:

```go
package engine

import "testing"

func TestDirNameRuleMatchesDir(t *testing.T) {
	r := dirNameRule{name: "node_modules", category: "node_modules"}
	m, cat, skip := r.Match(&Entry{Path: "/x/node_modules", IsDir: true})
	if !m || cat != "node_modules" || !skip {
		t.Fatalf("Match = (%v, %q, %v), want (true, \"node_modules\", true)", m, cat, skip)
	}
}

func TestDirNameRuleIgnoresFileAndOtherNames(t *testing.T) {
	r := dirNameRule{name: "node_modules", category: "node_modules"}
	if m, _, _ := r.Match(&Entry{Path: "/x/node_modules", IsDir: false}); m {
		t.Error("must not match a file named node_modules")
	}
	if m, _, _ := r.Match(&Entry{Path: "/x/src", IsDir: true}); m {
		t.Error("must not match an unrelated directory")
	}
}

func TestFileNameRuleMatchesFile(t *testing.T) {
	r := fileNameRule{name: ".DS_Store", category: "os-junk"}
	m, cat, skip := r.Match(&Entry{Path: "/x/.DS_Store", IsDir: false})
	if !m || cat != "os-junk" || skip {
		t.Fatalf("Match = (%v, %q, %v), want (true, \"os-junk\", false)", m, cat, skip)
	}
	if m, _, _ := r.Match(&Entry{Path: "/x/.DS_Store", IsDir: true}); m {
		t.Error("must not match a directory named .DS_Store")
	}
}

func TestDefaultRulesMatchExpected(t *testing.T) {
	rules := DefaultRules()
	cases := []struct {
		e       *Entry
		wantCat string
	}{
		{&Entry{Path: "/p/node_modules", IsDir: true}, "node_modules"},
		{&Entry{Path: "/p/target", IsDir: true}, "build-cache"},
		{&Entry{Path: "/p/build", IsDir: true}, "build-cache"},
		{&Entry{Path: "/p/.gradle", IsDir: true}, "build-cache"},
		{&Entry{Path: "/p/dist", IsDir: true}, "build-cache"},
		{&Entry{Path: "/p/__pycache__", IsDir: true}, "python-cache"},
		{&Entry{Path: "/p/.DS_Store", IsDir: false}, "os-junk"},
	}
	for _, c := range cases {
		cat, _, ok := matchRules(rules, c.e)
		if !ok || cat != c.wantCat {
			t.Errorf("matchRules(%s) = (%q, ok=%v), want %q", c.e.Path, cat, ok, c.wantCat)
		}
	}
	if _, _, ok := matchRules(rules, &Entry{Path: "/p/src", IsDir: true}); ok {
		t.Error("a normal source dir must not match any default rule")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `export PATH="/opt/homebrew/bin:$PATH"; go test ./internal/engine/ -run 'Rule|DefaultRules' -v`
Expected: FAIL — `undefined: dirNameRule` / `undefined: DefaultRules`.

- [ ] **Step 3: Write the implementation**

Create `internal/engine/rule.go` (comments in Korean):

```go
package engine

import "path/filepath"

// Rule 은 하나의 junk 판별 규칙이다. Match 는 e 가 이 규칙에 해당하면
// (true, 카테고리, skipChildren) 을 반환한다. skipChildren 이 true 면 매치된
// 디렉토리를 단일 회수 단위로 보고 그 하위는 더 분류하지 않는다(node_modules 처럼).
type Rule interface {
	Match(e *Entry) (matched bool, category string, skipChildren bool)
}

// dirNameRule 은 특정 이름의 디렉토리를 매치한다. 디렉토리 통째가 하나의 회수 단위라
// skipChildren 은 true 다.
type dirNameRule struct {
	name     string
	category string
}

func (r dirNameRule) Match(e *Entry) (bool, string, bool) {
	if e.IsDir && filepath.Base(e.Path) == r.name {
		return true, r.category, true
	}
	return false, "", false
}

// fileNameRule 은 특정 이름의 파일을 매치한다. 파일이므로 하위가 없어 skipChildren 은
// 무의미하며 false 다.
type fileNameRule struct {
	name     string
	category string
}

func (r fileNameRule) Match(e *Entry) (bool, string, bool) {
	if !e.IsDir && filepath.Base(e.Path) == r.name {
		return true, r.category, false
	}
	return false, "", false
}

// DefaultRules 는 P2 의 내장 규칙 집합을 순서대로 반환한다. 앞의 규칙이 먼저 매치된다.
func DefaultRules() []Rule {
	return []Rule{
		dirNameRule{name: "node_modules", category: "node_modules"},
		dirNameRule{name: "target", category: "build-cache"},
		dirNameRule{name: "build", category: "build-cache"},
		dirNameRule{name: ".gradle", category: "build-cache"},
		dirNameRule{name: "dist", category: "build-cache"},
		dirNameRule{name: "__pycache__", category: "python-cache"},
		fileNameRule{name: ".DS_Store", category: "os-junk"},
	}
}

// matchRules 는 규칙들을 순서대로 시도해 첫 매치의 (카테고리, skipChildren, true) 를
// 반환한다. 아무 것도 매치하지 않으면 ("", false, false).
func matchRules(rules []Rule, e *Entry) (string, bool, bool) {
	for _, r := range rules {
		if m, cat, skip := r.Match(e); m {
			return cat, skip, true
		}
	}
	return "", false, false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/engine/ -run 'Rule|DefaultRules' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/engine/rule.go internal/engine/rule_test.go
git commit -m "feat: add engine Rule interface and built-in junk rules"
```

---

### Task 2: Classify tree into category groups

**Files:**
- Create: `internal/engine/classify.go`
- Test: `internal/engine/classify_test.go`

**Interfaces:**
- Consumes: `Entry`, `Report` (P1); `Rule`, `matchRules` (Task 1).
- Produces:
  - `type CategoryGroup struct { Category string; Size int64; Count int; Paths []string }`
  - `func Classify(report *Report, rules []Rule) []CategoryGroup` — groups matched entries by category, sorted by `Size` descending; a matched entry with `skipChildren` is counted once and its subtree is not classified further; returns an empty slice when nothing matches.

- [ ] **Step 1: Write the failing test**

Create `internal/engine/classify_test.go`:

```go
package engine

import "testing"

// 트리:
// /p
//   node_modules (dir, size 1000; 안에 lib 와 .DS_Store 있지만 단일 단위로 셈)
//   src (dir, size 200)
//     __pycache__ (dir, size 200)
//   .DS_Store (file, size 6)
func sampleTree() *Report {
	root := &Entry{Path: "/p", IsDir: true, Size: 1206, Children: []*Entry{
		{Path: "/p/node_modules", IsDir: true, Size: 1000, Children: []*Entry{
			{Path: "/p/node_modules/lib", IsDir: true, Size: 1000, Children: []*Entry{
				{Path: "/p/node_modules/lib/.DS_Store", IsDir: false, Size: 6},
			}},
		}},
		{Path: "/p/src", IsDir: true, Size: 200, Children: []*Entry{
			{Path: "/p/src/__pycache__", IsDir: true, Size: 200},
		}},
		{Path: "/p/.DS_Store", IsDir: false, Size: 6},
	}}
	return &Report{Root: root, TotalSize: 1206}
}

func TestClassifyGroupsAndSkipsChildren(t *testing.T) {
	groups := Classify(sampleTree(), DefaultRules())

	byCat := map[string]CategoryGroup{}
	for _, g := range groups {
		byCat[g.Category] = g
	}

	if g := byCat["node_modules"]; g.Size != 1000 || g.Count != 1 {
		t.Errorf("node_modules = size %d count %d, want 1000/1", g.Size, g.Count)
	}
	if g := byCat["python-cache"]; g.Size != 200 || g.Count != 1 {
		t.Errorf("python-cache = size %d count %d, want 200/1", g.Size, g.Count)
	}
	// .DS_Store inside node_modules must NOT be counted separately (subtree skipped);
	// only the top-level one counts.
	if g := byCat["os-junk"]; g.Size != 6 || g.Count != 1 {
		t.Errorf("os-junk = size %d count %d, want 6/1 (inner .DS_Store skipped)", g.Size, g.Count)
	}
}

func TestClassifySortedBySizeDesc(t *testing.T) {
	groups := Classify(sampleTree(), DefaultRules())
	if len(groups) < 2 {
		t.Fatalf("expected >=2 groups, got %d", len(groups))
	}
	for i := 1; i < len(groups); i++ {
		if groups[i-1].Size < groups[i].Size {
			t.Errorf("groups not sorted desc: %d before %d", groups[i-1].Size, groups[i].Size)
		}
	}
}

func TestClassifyEmptyWhenNoJunk(t *testing.T) {
	root := &Entry{Path: "/p", IsDir: true, Size: 3, Children: []*Entry{
		{Path: "/p/main.go", IsDir: false, Size: 3},
	}}
	groups := Classify(&Report{Root: root, TotalSize: 3}, DefaultRules())
	if len(groups) != 0 {
		t.Errorf("clean tree should yield no groups, got %d", len(groups))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/engine/ -run TestClassify -v`
Expected: FAIL — `undefined: Classify` / `undefined: CategoryGroup`.

- [ ] **Step 3: Write the implementation**

Create `internal/engine/classify.go` (comments in Korean):

```go
package engine

import "sort"

// CategoryGroup 은 한 카테고리로 묶인 회수 가능 항목들의 요약이다.
type CategoryGroup struct {
	Category string
	Size     int64
	Count    int
	Paths    []string
}

// Classify 는 report 트리를 rules 로 분류해 카테고리별 그룹을 만든다. 매치된 엔트리는
// 크기·개수·경로가 해당 카테고리에 누적되고, skipChildren 이면 그 하위는 더 분류하지
// 않는다(디렉토리 통째가 하나의 회수 단위). 결과는 Size 내림차순으로 정렬되며, 아무 것도
// 매치하지 않으면 빈 슬라이스다.
func Classify(report *Report, rules []Rule) []CategoryGroup {
	groups := map[string]*CategoryGroup{}
	classifyEntry(report.Root, rules, groups)

	out := make([]CategoryGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Size > out[j].Size
	})
	return out
}

func classifyEntry(e *Entry, rules []Rule, groups map[string]*CategoryGroup) {
	if cat, skipChildren, ok := matchRules(rules, e); ok {
		g := groups[cat]
		if g == nil {
			g = &CategoryGroup{Category: cat}
			groups[cat] = g
		}
		g.Size += e.Size
		g.Count++
		g.Paths = append(g.Paths, e.Path)
		if skipChildren {
			return
		}
	}
	for _, c := range e.Children {
		classifyEntry(c, rules, groups)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/engine/ -run TestClassify -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/engine/classify.go internal/engine/classify_test.go
git commit -m "feat: add engine Classify grouping matched entries by category"
```

---

### Task 3: Category summary in scan output

**Files:**
- Modify: `cmd/scan.go`
- Modify: `cmd/scan_test.go`

**Interfaces:**
- Consumes: `engine.Classify`, `engine.DefaultRules`, `engine.CategoryGroup` (Task 1–2); `humanBytes`, `plural` (P1).
- Produces: no new exported symbol — extends `runScan`'s output with a reclaimable-category summary printed after the existing size list. When no category matches, nothing extra is printed.

- [ ] **Step 1: Write the failing test**

Add to `cmd/scan_test.go` (uses `os`/`path/filepath`/`bytes`/`strings`, already imported there):

```go
func TestRunScanShowsCategorySummary(t *testing.T) {
	root := t.TempDir()
	// node_modules with a file inside
	nm := filepath.Join(root, "node_modules")
	if err := os.MkdirAll(nm, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nm, "big.bin"), make([]byte, 2048), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// a stray .DS_Store at the root
	if err := os.WriteFile(filepath.Join(root, ".DS_Store"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var buf bytes.Buffer
	if err := runScan(&buf, root); err != nil {
		t.Fatalf("runScan: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Reclaimable") {
		t.Errorf("output missing reclaimable summary:\n%s", out)
	}
	if !strings.Contains(out, "node_modules") {
		t.Errorf("output missing node_modules category:\n%s", out)
	}
	if !strings.Contains(out, "os-junk") {
		t.Errorf("output missing os-junk category:\n%s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestRunScanShowsCategorySummary -v`
Expected: FAIL — no "Reclaimable" text in output.

- [ ] **Step 3: Extend runScan**

In `cmd/scan.go`, add the category summary after the existing children loop and before the `werrs` block. Insert this block (comments in Korean):

```go
	// junk 카테고리로 분류해 회수 가능 용량을 요약한다. 매치가 없으면 아무 것도 안 찍는다.
	groups := engine.Classify(report, engine.DefaultRules())
	if len(groups) > 0 {
		var reclaim int64
		for _, g := range groups {
			reclaim += g.Size
		}
		fmt.Fprintf(w, "\nReclaimable: %s across %d %s\n",
			humanBytes(reclaim), len(groups), plural(len(groups), "category", "categories"))
		for _, g := range groups {
			fmt.Fprintf(w, "  %10s  %-14s (%d %s)\n",
				humanBytes(g.Size), g.Category, g.Count, plural(g.Count, "item", "items"))
		}
	}
```

For reference, the resulting `runScan` order is: header → total → sorted child list → **category summary (new)** → skipped-errors note.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/ -run TestRunScanShowsCategorySummary -v`
Expected: PASS.

- [ ] **Step 5: Full build + test + manual smoke**

Run:
```bash
go build -o purgefs . && go vet ./... && go test ./... && ./purgefs scan .
rm -f purgefs
```
Expected: build OK; vet clean; all tests PASS; `scan .` prints the total, child list, and (if this repo has any) a Reclaimable category summary.

- [ ] **Step 6: Commit**

```bash
git add cmd/scan.go cmd/scan_test.go
git commit -m "feat: show reclaimable category summary in scan output"
```

---

## Self-Review

**1. Spec coverage (P2 scope):**
- Rule interface + registry → Task 1 (`Rule`, `DefaultRules`, `matchRules`). ✓
- Built-in rules (node_modules, target, build, .gradle, __pycache__, dist, .DS_Store) → Task 1 `DefaultRules`. ✓
- Category grouping → Task 2 `Classify` / `CategoryGroup`. ✓
- Reclaim preview → Task 3 (reclaimable total + per-category sizes). ✓
- Out of P2 scope by design: trash/purge (P3), TUI (P4), undo/preset (P5), staleness scoring, config-file rules.

**2. Placeholder scan:** No TBD/TODO/"handle edge cases" — every code and test step is complete. ✓

**3. Type consistency:** `Rule.Match(e *Entry) (bool, string, bool)`, `matchRules(rules, e) (string, bool, bool)`, `Classify(*Report, []Rule) []CategoryGroup`, `CategoryGroup{Category, Size, Count, Paths}` are used identically across Tasks 1–3 and the tests. Category strings (`node_modules`, `build-cache`, `python-cache`, `os-junk`) match between `DefaultRules` and the classify/scan tests. ✓

## Notes

- Korean comments are mandatory (see Global Constraints); the code blocks above already follow this — implementers should preserve them verbatim.
- `Classify` starts at `report.Root`, so scanning a junk directory directly (e.g. `purgefs scan ./node_modules`) will classify the root itself as one group. That is correct and harmless.
- Perf: matched directories are walked for size by P1's `Walk`. Prune-during-walk is a deliberate later optimization, not part of P2.
