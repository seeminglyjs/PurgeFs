# P2 룰 시스템 구현 계획

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**목표:** 스캔한 트리를 junk 카테고리(node_modules, 빌드 캐시, __pycache__, .DS_Store)로 분류하고, `scan` 출력에 카테고리별 회수 가능 용량 요약을 보여준다.

**아키텍처:** `Rule` 인터페이스와 내장 규칙이 `internal/engine` 에 있다. 새 `Classify` 패스가 (P1 의 `Walk` 가 그대로 만든) 기존 `Report` 트리를 순회해 매치된 엔트리를 카테고리로 묶는다. 매치된 디렉토리(예: `node_modules`)는 하나의 회수 단위이며 그 크기는 P1 이 이미 집계했다. 그 하위는 더 분류하지 않는다. `scan` 은 기존 용량 리스트 아래에 카테고리 요약 섹션을 얻는다.

**기술 스택:** Go, 엔진은 표준 라이브러리만. `cobra` 는 이미 연결됨.

## 전역 제약 (모든 태스크에 적용)

- Go 모듈 `github.com/seeminglyjs/PurgeFs`; `go.mod` 하한 `go 1.24`.
- `internal/engine` 은 표준 라이브러리만 쓴다 — 새 의존성 금지. 엔진은 `cmd/` 를 절대 import 하지 않는다.
- **코드 주석은 반드시 한국어** — 모든 `.go` 소스·테스트 파일, 문서 주석·인라인 주석 전부. 식별자·API 이름·카테고리 문자열·에러 문자열은 그대로 둔다.
- **문서(스펙·계획·설계)는 한국어로 작성** — 산문·제목·설명은 한국어. 코드 블록, 셸 명령, 파일 경로, 식별자, 커밋 메시지는 그대로.
- 커밋에 Claude / 어떤 AI 도 co-author·트레일러로 넣지 않는다. Conventional-commit 접두어(`feat`/`test`/`docs`) 사용.
- P2 는 아무것도 삭제하지 않는다. 규칙은 빌드/캐시/OS junk 만 대상 — 사용자 데이터는 절대 아님.
- P1 의 `Walk` 는 수정 금지 — 분류는 기존 트리 위 별도 패스다.

## 설계 판단 (스펙의 예시 텍스트와 다른 점)

- `Rule.Match` 는 `fs.DirEntry` 가 아니라 `*Entry` 를 받는다. 스펙의 `fs.DirEntry` 시그니처는 순회 중(during-walk) 설계를 가정했지만, 우리는 순회 **후**(classify-after) `Entry` 트리를 분류하므로 `*Entry` 가 맞다.
- 내장 규칙은 `internal/engine/rules` 서브패키지가 아니라 `engine` 패키지 자체에 둔다. import 사이클 회피(`Classify` 가 기본 규칙을 필요로 하는데, 서브패키지는 `engine` 의 `Rule`/`Entry` 를 필요로 함). 규칙 집합이 커지면 나중에 서브패키지로 승격.
- 매치된 디렉토리도 P1 의 `Walk` 가 크기를 위해 순회한다(순회 중 프루닝은 나중 최적화). P1 의 순차 순회처럼 정확성 우선.

## 파일 구조

- 생성 `internal/engine/rule.go` — `Rule` 인터페이스, `dirNameRule`/`fileNameRule` 구현, `DefaultRules()`, 공용 `matchRules` 헬퍼.
- 생성 `internal/engine/classify.go` — `CategoryGroup` 타입과 `Classify(*Report, []Rule) []CategoryGroup`.
- 생성 `internal/engine/rule_test.go`, `internal/engine/classify_test.go`.
- 수정 `cmd/scan.go` — `runScan` 출력에 카테고리 요약 + 회수 가능 총량 추가.
- 수정 `cmd/scan_test.go` — 카테고리 요약이 나오는지 확인하는 테스트 추가.

---

### 태스크 1: Rule 인터페이스 + 내장 규칙

**파일:**
- 생성: `internal/engine/rule.go`
- 테스트: `internal/engine/rule_test.go`

**인터페이스:**
- 소비: P1 의 `Entry` (`internal/engine/model.go`).
- 생산:
  - `type Rule interface { Match(e *Entry) (matched bool, category string, skipChildren bool) }`
  - `func DefaultRules() []Rule`
  - `func matchRules(rules []Rule, e *Entry) (category string, skipChildren bool, ok bool)` — 규칙을 순서대로 시도해 첫 매치를 반환.

- [ ] **단계 1: 실패 테스트 작성**

`internal/engine/rule_test.go` 생성:

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

- [ ] **단계 2: 테스트 실행해 실패 확인**

실행: `export PATH="/opt/homebrew/bin:$PATH"; go test ./internal/engine/ -run 'Rule|DefaultRules' -v`
기대: FAIL — `undefined: dirNameRule` / `undefined: DefaultRules`.

- [ ] **단계 3: 구현 작성**

`internal/engine/rule.go` 생성 (주석 한국어):

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

- [ ] **단계 4: 테스트 실행해 통과 확인**

실행: `go test ./internal/engine/ -run 'Rule|DefaultRules' -v`
기대: PASS.

- [ ] **단계 5: 커밋**

```bash
git add internal/engine/rule.go internal/engine/rule_test.go
git commit -m "feat: add engine Rule interface and built-in junk rules"
```

---

### 태스크 2: 트리를 카테고리 그룹으로 분류

**파일:**
- 생성: `internal/engine/classify.go`
- 테스트: `internal/engine/classify_test.go`

**인터페이스:**
- 소비: `Entry`, `Report` (P1); `Rule`, `matchRules` (태스크 1).
- 생산:
  - `type CategoryGroup struct { Category string; Size int64; Count int; Paths []string }`
  - `func Classify(report *Report, rules []Rule) []CategoryGroup` — 매치된 엔트리를 카테고리로 묶고 `Size` 내림차순 정렬. `skipChildren` 인 엔트리는 한 번만 세고 그 하위는 분류하지 않음. 매치가 없으면 빈 슬라이스 반환.

- [ ] **단계 1: 실패 테스트 작성**

`internal/engine/classify_test.go` 생성:

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
	// node_modules 안의 .DS_Store 는 따로 세면 안 됨(하위 skip); 최상위 것만 셈.
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

- [ ] **단계 2: 테스트 실행해 실패 확인**

실행: `go test ./internal/engine/ -run TestClassify -v`
기대: FAIL — `undefined: Classify` / `undefined: CategoryGroup`.

- [ ] **단계 3: 구현 작성**

`internal/engine/classify.go` 생성 (주석 한국어):

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

- [ ] **단계 4: 테스트 실행해 통과 확인**

실행: `go test ./internal/engine/ -run TestClassify -v`
기대: PASS.

- [ ] **단계 5: 커밋**

```bash
git add internal/engine/classify.go internal/engine/classify_test.go
git commit -m "feat: add engine Classify grouping matched entries by category"
```

---

### 태스크 3: scan 출력에 카테고리 요약

**파일:**
- 수정: `cmd/scan.go`
- 수정: `cmd/scan_test.go`

**인터페이스:**
- 소비: `engine.Classify`, `engine.DefaultRules`, `engine.CategoryGroup` (태스크 1~2); `humanBytes`, `plural` (P1).
- 생산: 새 export 심볼 없음 — 기존 용량 리스트 뒤에 회수 가능 카테고리 요약을 출력하도록 `runScan` 을 확장. 매치가 없으면 아무것도 추가 출력하지 않음.

- [ ] **단계 1: 실패 테스트 작성**

`cmd/scan_test.go` 에 추가 (`os`/`path/filepath`/`bytes`/`strings` 는 이미 import 됨):

```go
func TestRunScanShowsCategorySummary(t *testing.T) {
	root := t.TempDir()
	// 안에 파일이 있는 node_modules
	nm := filepath.Join(root, "node_modules")
	if err := os.MkdirAll(nm, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nm, "big.bin"), make([]byte, 2048), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// 루트에 떠도는 .DS_Store
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

- [ ] **단계 2: 테스트 실행해 실패 확인**

실행: `go test ./cmd/ -run TestRunScanShowsCategorySummary -v`
기대: FAIL — 출력에 "Reclaimable" 없음.

- [ ] **단계 3: runScan 확장**

`cmd/scan.go` 에서 기존 자식 루프 뒤, `werrs` 블록 앞에 카테고리 요약을 추가한다. 이 블록을 삽입 (주석 한국어):

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

참고 — 최종 `runScan` 출력 순서: 헤더 → 총량 → 정렬된 자식 리스트 → **카테고리 요약(신규)** → skip 에러 안내.

- [ ] **단계 4: 테스트 실행해 통과 확인**

실행: `go test ./cmd/ -run TestRunScanShowsCategorySummary -v`
기대: PASS.

- [ ] **단계 5: 전체 빌드 + 테스트 + 수동 확인**

실행:
```bash
go build -o purgefs . && go vet ./... && go test ./... && ./purgefs scan .
rm -f purgefs
```
기대: 빌드 OK; vet 클린; 전체 테스트 PASS; `scan .` 이 총량·자식 리스트·(이 repo 에 junk 가 있으면) 회수 가능 카테고리 요약을 출력.

- [ ] **단계 6: 커밋**

```bash
git add cmd/scan.go cmd/scan_test.go
git commit -m "feat: show reclaimable category summary in scan output"
```

---

## 자체 검토

**1. 스펙 커버리지 (P2 범위):**
- Rule 인터페이스 + 레지스트리 → 태스크 1 (`Rule`, `DefaultRules`, `matchRules`). ✓
- 내장 규칙(node_modules, target, build, .gradle, __pycache__, dist, .DS_Store) → 태스크 1 `DefaultRules`. ✓
- 카테고리 그룹 → 태스크 2 `Classify` / `CategoryGroup`. ✓
- 회수량 미리보기 → 태스크 3 (회수 가능 총량 + 카테고리별 용량). ✓
- 설계상 P2 범위 밖: trash/purge(P3), TUI(P4), undo/preset(P5), staleness 스코어링, config 파일 규칙.

**2. Placeholder 스캔:** TBD/TODO/"엣지케이스 처리" 없음 — 모든 코드·테스트 단계가 완결. ✓

**3. 타입 일관성:** `Rule.Match(e *Entry) (bool, string, bool)`, `matchRules(rules, e) (string, bool, bool)`, `Classify(*Report, []Rule) []CategoryGroup`, `CategoryGroup{Category, Size, Count, Paths}` 가 태스크 1~3 과 테스트에서 동일하게 쓰임. 카테고리 문자열(`node_modules`, `build-cache`, `python-cache`, `os-junk`)이 `DefaultRules` 와 classify/scan 테스트 사이에서 일치. ✓

## 참고

- 주석 한국어는 필수(전역 제약 참고). 위 코드 블록은 이미 이를 따르므로 구현자는 그대로 보존할 것.
- `Classify` 는 `report.Root` 부터 시작하므로, junk 디렉토리를 직접 스캔하면(예: `purgefs scan ./node_modules`) 루트 자체가 한 그룹으로 분류된다. 정확하고 무해함.
- 성능: 매치된 디렉토리도 P1 의 `Walk` 가 크기를 위해 순회한다. 순회 중 프루닝은 의도된 나중 최적화이며 P2 범위 아님.
