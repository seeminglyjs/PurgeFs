# P5 undo 복원 + 개발자 프리셋 구현 계획

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**목표:** 휴지통으로 옮긴 purge를 되돌리는 `purgefs undo` 와, 개발 캐시만 겨냥하는 `purgefs purge --preset dev-caches` 를 추가한다. 휴지통 이동은 매니페스트(`~/.purgefs/history`)에 원본↔휴지통 경로를 기록하고, `undo` 가 최신 기록을 읽어 되돌린다. `--hard`(완전 삭제)는 되돌릴 수 없다.

**아키텍처:** `internal/trash` 가 이동 매핑(원본→목적지)을 결과에 담는다. 새 `internal/history` 패키지가 매니페스트를 저장·조회·복원한다. `purge` 는 휴지통 이동 후 매니페스트를 남기고, `undo` 커맨드가 최신 매니페스트를 복원한다. 프리셋은 `engine` 이 이름별 규칙 집합을 주고, `purge --preset` 이 그 규칙으로 분류한다.

**기술 스택:** Go, 표준 라이브러리만(신규 외부 의존성 없음). `cobra`·`bubbletea` 기존 그대로.

## 전역 제약 (모든 태스크에 적용)

- Go 모듈 `github.com/seeminglyjs/PurgeFs`; `go.mod` 하한 `go 1.24`.
- `internal/engine`·`internal/history` 는 표준 라이브러리만. 새 외부 의존성 없음. 엔진은 `cmd/` 를 import 하지 않는다.
- **코드 주석은 한국어**, **문서는 한국어**(코드 블록·셸 명령·식별자·에러 문자열·커밋 메시지는 그대로).
- 커밋에 Claude / 어떤 AI 도 co-author·트레일러로 넣지 않는다. Conventional-commit 접두어.
- 삭제 안전 원칙 유지. `undo` 는 원본 자리에 이미 뭔가 있으면 덮어쓰지 않고 건너뛴다. `--hard` 는 매니페스트를 남기지 않으므로 되돌릴 수 없다.

## 설계 판단

- `trash.Result` 에 기존 `Trashed []string`(요약용)은 그대로 두고, `Moved []Moved{Original,Dest}` 를 **추가**한다. 기존 호출부·테스트(개수만 사용)는 안 깨지고, undo 는 `Moved` 로 매핑을 얻는다. `hardDeleter` 는 `Moved` 를 비운다(되돌릴 수 없음).
- 매니페스트 파일명은 `<CreatedAt>.json`(unix 초). `CreatedAt` 은 호출부가 주입해 테스트를 결정적으로 만든다(엔진 코드에서 `time.Now()` 를 직접 호출하는 곳은 `cmd` 뿐).
- 프리셋 규칙은 `engine.Preset(name)` 이 제공. `dev-caches` = 빌드/의존성 캐시 디렉토리(node_modules·target·build·.gradle·dist·__pycache__), OS junk(.DS_Store) 제외.

## 파일 구조

- 수정 `internal/trash/trash.go` — `Moved` 타입, `Result.Moved`, `macTrasher` 가 매핑 기록.
- 수정 `internal/trash/trash_test.go` — `Moved` 검증.
- 생성 `internal/history/history.go` — `Item`/`Manifest`/`Save`/`LoadLatest`/`Restore`/`RestoreResult`/`Failure`.
- 생성 `internal/history/history_test.go`.
- 수정 `cmd/purge.go` — `historyDir`/`movedToItems`/`recordHistory` + 트래시 후 기록 연결.
- 생성 `cmd/undo.go` — `undoCmd`/`runUndoDir`.
- 생성 `cmd/undo_test.go`.
- 수정 `internal/engine/rule.go` — `Preset(name)`.
- 수정 `internal/engine/rule_test.go` — 프리셋 검증.
- 수정 `cmd/purge.go` — `--preset` 플래그, `runPurge`/`runPurgeInteractive` 가 규칙을 받도록.
- 수정 `cmd/purge_test.go` — 변경된 `runPurge` 시그니처 반영.

---

### Task 1: trash 이동 매핑 기록

**파일:**
- 수정: `internal/trash/trash.go`
- 수정: `internal/trash/trash_test.go`

**인터페이스:**
- 생산: `type Moved struct { Original, Dest string }`; `Result` 에 `Moved []Moved` 필드 추가. `macTrasher.Trash` 가 성공한 이동마다 `Moved{Original, Dest}` 를 채운다. `hardDeleter` 는 `Moved` 를 비운다.

- [ ] **단계 1: 실패 테스트 작성(기존 테스트에 단언 추가)**

`internal/trash/trash_test.go` 의 `TestMacTrasherMovesToTrash` 끝에 추가:

```go
	if len(res.Moved) != 1 {
		t.Fatalf("Moved = %+v, want 1 entry", res.Moved)
	}
	if res.Moved[0].Original != src {
		t.Errorf("Moved[0].Original = %q, want %q", res.Moved[0].Original, src)
	}
	if res.Moved[0].Dest != filepath.Join(trashDir, "junk.txt") {
		t.Errorf("Moved[0].Dest = %q, want %q", res.Moved[0].Dest, filepath.Join(trashDir, "junk.txt"))
	}
```

그리고 `TestMacTrasherRenamesOnCollision` 끝에 추가:

```go
	if len(res.Moved) != 1 || res.Moved[0].Dest != filepath.Join(trashDir, "dup 2") {
		t.Errorf("Moved = %+v, want dest .../dup 2", res.Moved)
	}
```

그리고 새 테스트로 hardDeleter 는 Moved 가 비어야 함을 확인:

```go
func TestHardDeleterHasNoMoved(t *testing.T) {
	base := t.TempDir()
	f := filepath.Join(base, "cache")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	res := NewHardDeleter().Trash([]string{f})
	if len(res.Moved) != 0 {
		t.Errorf("hardDeleter must not record Moved, got %+v", res.Moved)
	}
	if len(res.Trashed) != 1 {
		t.Errorf("Trashed = %d, want 1", len(res.Trashed))
	}
}
```

- [ ] **단계 2: 테스트 실행해 실패 확인**

실행: `export PATH="/opt/homebrew/bin:$PATH"; go test ./internal/trash/ -v`
기대: FAIL — `res.Moved` 없음(컴파일 에러 `res.Moved undefined`).

- [ ] **단계 3: 구현 수정**

`internal/trash/trash.go` 에서 `Result` 위에 `Moved` 타입을 추가하고 `Result` 에 필드를 더한다:

```go
// Moved 는 휴지통으로 옮긴 항목의 원본→목적지 매핑이다(undo 용). 완전 삭제는 이 정보가 없다.
type Moved struct {
	Original string
	Dest     string
}

// Result 는 한 번의 정리 작업 결과다. 경로별 실패는 Failed 에 모이고 나머지는 계속 처리된다.
type Result struct {
	Trashed []string // 처리된 원본 경로(개수·요약용)
	Moved   []Moved  // 휴지통 이동 매핑(undo 용). 완전 삭제 시 비어 있음.
	Failed  []Failure
}
```

`macTrasher.Trash` 의 성공 분기에서 `Moved` 도 채운다:

```go
		r.Trashed = append(r.Trashed, p)
		r.Moved = append(r.Moved, Moved{Original: p, Dest: dest})
```

`hardDeleter.Trash` 는 그대로 둔다(Moved 안 채움).

- [ ] **단계 4: 테스트 실행해 통과 확인**

실행: `go test ./internal/trash/ -v`
기대: PASS.

- [ ] **단계 5: 커밋**

```bash
git add internal/trash/trash.go internal/trash/trash_test.go
git commit -m "feat: record original->trash destination mapping in trash Result"
```

---

### Task 2: history 패키지(저장·조회·복원)

**파일:**
- 생성: `internal/history/history.go`
- 테스트: `internal/history/history_test.go`

**인터페이스:**
- 생산:
  - `type Item struct { Original, Dest string }`, `type Manifest struct { CreatedAt int64; Items []Item }`
  - `func Save(dir string, m Manifest) (string, error)`
  - `func LoadLatest(dir string) (Manifest, bool, error)`
  - `type RestoreResult struct { Restored, Skipped []string; Failed []Failure }`, `type Failure struct { Item Item; Err error }`
  - `func Restore(m Manifest) RestoreResult`

- [ ] **단계 1: 실패 테스트 작성**

`internal/history/history_test.go` 생성:

```go
package history

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadLatestPicksNewest(t *testing.T) {
	dir := t.TempDir()
	if _, err := Save(dir, Manifest{CreatedAt: 100, Items: []Item{{Original: "/a", Dest: "/t/a"}}}); err != nil {
		t.Fatalf("save 1: %v", err)
	}
	if _, err := Save(dir, Manifest{CreatedAt: 200, Items: []Item{{Original: "/b", Dest: "/t/b"}}}); err != nil {
		t.Fatalf("save 2: %v", err)
	}
	m, ok, err := LoadLatest(dir)
	if err != nil || !ok {
		t.Fatalf("LoadLatest = (%+v, %v, %v)", m, ok, err)
	}
	if m.CreatedAt != 200 || len(m.Items) != 1 || m.Items[0].Original != "/b" {
		t.Errorf("latest = %+v, want CreatedAt 200 with /b", m)
	}
}

func TestLoadLatestEmptyDir(t *testing.T) {
	_, ok, err := LoadLatest(t.TempDir())
	if err != nil || ok {
		t.Errorf("empty dir: ok=%v err=%v, want ok=false err=nil", ok, err)
	}
}

func TestLoadLatestMissingDir(t *testing.T) {
	_, ok, err := LoadLatest(filepath.Join(t.TempDir(), "nope"))
	if err != nil || ok {
		t.Errorf("missing dir: ok=%v err=%v, want ok=false err=nil", ok, err)
	}
}

func TestRestoreMovesBackAndSkips(t *testing.T) {
	base := t.TempDir()
	// 휴지통에 있는 파일(Dest 존재), 원본 자리는 비어 있음 → 복원되어야 함
	dest := filepath.Join(base, "trash-node_modules")
	if err := os.WriteFile(dest, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed dest: %v", err)
	}
	original := filepath.Join(base, "node_modules")

	// 원본 자리에 이미 뭔가 있는 두 번째 항목 → 건너뛰어야 함
	occupied := filepath.Join(base, "occupied")
	if err := os.WriteFile(occupied, []byte("keep"), 0o644); err != nil {
		t.Fatalf("seed occupied: %v", err)
	}

	m := Manifest{Items: []Item{
		{Original: original, Dest: dest},
		{Original: occupied, Dest: filepath.Join(base, "missing-dest")},
	}}
	res := Restore(m)

	if len(res.Restored) != 1 || res.Restored[0] != original {
		t.Errorf("Restored = %v, want [%s]", res.Restored, original)
	}
	if len(res.Skipped) != 1 {
		t.Errorf("Skipped = %v, want 1", res.Skipped)
	}
	if _, err := os.Lstat(original); err != nil {
		t.Errorf("original not restored: %v", err)
	}
}
```

- [ ] **단계 2: 테스트 실행해 실패 확인**

실행: `go test ./internal/history/ -v`
기대: FAIL — `undefined: Save` 등.

- [ ] **단계 3: 구현 작성**

`internal/history/history.go` 생성 (주석 한국어):

```go
// Package history 는 purge 매니페스트를 저장하고, 최신 것을 불러와 복원한다. 매니페스트는
// 원본↔휴지통 경로 매핑을 담아 undo 가 파일을 되돌릴 수 있게 한다.
package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
)

// Item 은 복원할 항목 하나다(원본 자리 ← 휴지통 경로).
type Item struct {
	Original string `json:"original"`
	Dest     string `json:"dest"`
}

// Manifest 는 한 번의 휴지통 purge 기록이다.
type Manifest struct {
	CreatedAt int64  `json:"created_at"` // unix 초
	Items     []Item `json:"items"`
}

// Failure 는 복원하지 못한 항목이다.
type Failure struct {
	Item Item
	Err  error
}

// RestoreResult 는 복원 결과다.
type RestoreResult struct {
	Restored []string // 복원된 원본 경로
	Skipped  []string // 원본이 이미 있거나 휴지통 파일이 없어 건너뜀
	Failed   []Failure
}

// Save 는 매니페스트를 dir/<CreatedAt>.json 으로 쓰고 그 경로를 반환한다.
func Save(dir string, m Manifest) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, strconv.FormatInt(m.CreatedAt, 10)+".json")
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// LoadLatest 는 dir 안의 매니페스트 중 CreatedAt 이 가장 큰 것을 반환한다. 디렉토리가 없거나
// 매니페스트가 하나도 없으면 ok=false 를 돌려준다(에러 아님).
func LoadLatest(dir string) (Manifest, bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, false, nil
		}
		return Manifest{}, false, err
	}
	var latest Manifest
	found := false
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return Manifest{}, false, err
		}
		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			return Manifest{}, false, err
		}
		if !found || m.CreatedAt > latest.CreatedAt {
			latest = m
			found = true
		}
	}
	return latest, found, nil
}

// Restore 는 매니페스트의 각 항목을 휴지통(Dest)에서 원본(Original)으로 되돌린다. 원본 자리에
// 이미 뭔가 있거나 Dest 가 없으면 덮어쓰지 않고 건너뛴다.
func Restore(m Manifest) RestoreResult {
	var r RestoreResult
	for _, it := range m.Items {
		if _, err := os.Lstat(it.Original); err == nil {
			r.Skipped = append(r.Skipped, it.Original) // 원본 자리에 이미 존재
			continue
		}
		if _, err := os.Lstat(it.Dest); err != nil {
			r.Skipped = append(r.Skipped, it.Original) // 휴지통에 없음
			continue
		}
		if err := os.Rename(it.Dest, it.Original); err != nil {
			r.Failed = append(r.Failed, Failure{Item: it, Err: err})
			continue
		}
		r.Restored = append(r.Restored, it.Original)
	}
	return r
}
```

- [ ] **단계 4: 테스트 실행해 통과 확인**

실행: `go test ./internal/history/ -v`
기대: PASS.

- [ ] **단계 5: 커밋**

```bash
git add internal/history/history.go internal/history/history_test.go
git commit -m "feat: add history package (save/load/restore purge manifests)"
```

---

### Task 3: purge 기록 + undo 커맨드

**파일:**
- 수정: `cmd/purge.go`
- 생성: `cmd/undo.go`
- 생성: `cmd/undo_test.go`

**인터페이스:**
- 생산:
  - `func historyDir() (string, error)` — `~/.purgefs/history`.
  - `func movedToItems(moved []trash.Moved) []history.Item`
  - `func recordHistory(dir string, res trash.Result, createdAt int64) error` — `res.Moved` 가 있으면 매니페스트 저장, 없으면 no-op.
  - `func runUndoDir(w io.Writer, dir string) error` + `undoCmd`.
- 소비: `internal/history`, `internal/trash`.

- [ ] **단계 1: 실패 테스트 작성**

`cmd/undo_test.go` 생성:

```go
package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seeminglyjs/PurgeFs/internal/history"
	"github.com/seeminglyjs/PurgeFs/internal/trash"
)

func TestRecordHistoryWritesManifest(t *testing.T) {
	dir := t.TempDir()
	res := trash.Result{Moved: []trash.Moved{{Original: "/p/node_modules", Dest: "/t/node_modules"}}}
	if err := recordHistory(dir, res, 1234); err != nil {
		t.Fatalf("recordHistory: %v", err)
	}
	m, ok, err := history.LoadLatest(dir)
	if err != nil || !ok {
		t.Fatalf("LoadLatest = (%+v, %v, %v)", m, ok, err)
	}
	if len(m.Items) != 1 || m.Items[0].Original != "/p/node_modules" {
		t.Errorf("manifest = %+v", m)
	}
}

func TestRecordHistoryNoMovedIsNoop(t *testing.T) {
	dir := t.TempDir()
	if err := recordHistory(dir, trash.Result{Trashed: []string{"/x"}}, 1); err != nil {
		t.Fatalf("recordHistory: %v", err)
	}
	if _, ok, _ := history.LoadLatest(dir); ok {
		t.Error("no Moved should write no manifest")
	}
}

func TestRunUndoRestores(t *testing.T) {
	base := t.TempDir()
	histDir := filepath.Join(base, "history")
	dest := filepath.Join(base, "trash-nm")
	if err := os.WriteFile(dest, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	original := filepath.Join(base, "nm")
	if _, err := history.Save(histDir, history.Manifest{CreatedAt: 5, Items: []history.Item{{Original: original, Dest: dest}}}); err != nil {
		t.Fatalf("save: %v", err)
	}

	var buf bytes.Buffer
	if err := runUndoDir(&buf, histDir); err != nil {
		t.Fatalf("runUndoDir: %v", err)
	}
	if _, err := os.Lstat(original); err != nil {
		t.Errorf("original not restored: %v", err)
	}
	if !strings.Contains(buf.String(), "복원") {
		t.Errorf("output missing restore notice:\n%s", buf.String())
	}
}

func TestRunUndoNoHistory(t *testing.T) {
	var buf bytes.Buffer
	if err := runUndoDir(&buf, filepath.Join(t.TempDir(), "empty")); err != nil {
		t.Fatalf("runUndoDir: %v", err)
	}
	if !strings.Contains(buf.String(), "복원할 기록이 없습니다") {
		t.Errorf("output = %q", buf.String())
	}
}
```

- [ ] **단계 2: 테스트 실행해 실패 확인**

실행: `go test ./cmd/ -run 'TestRecordHistory|TestRunUndo' -v`
기대: FAIL — `undefined: recordHistory` / `undefined: runUndoDir`.

- [ ] **단계 3: purge.go 에 헬퍼 추가 + 기록 연결**

`cmd/purge.go` 의 import 에 `"time"`, `"path/filepath"`(이미 있음), `"os"`(이미 있음), `"github.com/seeminglyjs/PurgeFs/internal/history"` 를 추가한다. 그리고 아래 헬퍼를 추가한다:

```go
// historyDir 은 매니페스트를 저장하는 ~/.purgefs/history 경로를 반환한다.
func historyDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".purgefs", "history"), nil
}

// movedToItems 는 휴지통 이동 매핑을 매니페스트 항목으로 바꾼다.
func movedToItems(moved []trash.Moved) []history.Item {
	items := make([]history.Item, 0, len(moved))
	for _, mv := range moved {
		items = append(items, history.Item{Original: mv.Original, Dest: mv.Dest})
	}
	return items
}

// recordHistory 는 휴지통 이동이 있으면 매니페스트를 저장한다. 완전 삭제(Moved 비어 있음)는
// 되돌릴 수 없으므로 아무것도 남기지 않는다.
func recordHistory(dir string, res trash.Result, createdAt int64) error {
	if len(res.Moved) == 0 {
		return nil
	}
	_, err := history.Save(dir, history.Manifest{CreatedAt: createdAt, Items: movedToItems(res.Moved)})
	return err
}
```

그리고 `runPurge` 의 `res := tr.Trash(paths)` 뒤, `runPurgeInteractive` 의 `tr.Trash(paths)` 처리 지점 양쪽에 기록을 끼운다. 두 함수의 트래시 직후를 다음 형태로 만든다(이미 `reportTrashResult` 로 끝나는 부분을 이렇게 바꾼다):

`runPurge` 의 끝부분:

```go
	res := tr.Trash(paths)
	if dir, err := historyDir(); err == nil {
		if herr := recordHistory(dir, res, time.Now().Unix()); herr != nil {
			fmt.Fprintf(w, "  (기록 실패: %v)\n", herr)
		}
	}
	return reportTrashResult(w, res)
```

`runPurgeInteractive` 의 끝부분(`return reportTrashResult(w, tr.Trash(paths))` 를 풀어서):

```go
	res := tr.Trash(paths)
	if dir, err := historyDir(); err == nil {
		if herr := recordHistory(dir, res, time.Now().Unix()); herr != nil {
			fmt.Fprintf(w, "  (기록 실패: %v)\n", herr)
		}
	}
	return reportTrashResult(w, res)
```

- [ ] **단계 4: undo 커맨드 작성**

`cmd/undo.go` 생성 (주석 한국어):

```go
package cmd

import (
	"fmt"
	"io"

	"github.com/seeminglyjs/PurgeFs/internal/history"
	"github.com/spf13/cobra"
)

var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Restore the most recent trash purge",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := historyDir()
		if err != nil {
			return err
		}
		return runUndoDir(cmd.OutOrStdout(), dir)
	},
}

// runUndoDir 은 dir 안의 최신 매니페스트를 읽어 파일을 되돌린다. 완전 삭제(--hard)는 기록이
// 없어 되돌릴 수 없다.
func runUndoDir(w io.Writer, dir string) error {
	m, ok, err := history.LoadLatest(dir)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(w, "복원할 기록이 없습니다.")
		return nil
	}
	res := history.Restore(m)
	fmt.Fprintf(w, "복원 %d개", len(res.Restored))
	if len(res.Skipped) > 0 {
		fmt.Fprintf(w, ", 건너뜀 %d개", len(res.Skipped))
	}
	if len(res.Failed) > 0 {
		fmt.Fprintf(w, ", 실패 %d개", len(res.Failed))
	}
	fmt.Fprintln(w)
	return nil
}

func init() {
	rootCmd.AddCommand(undoCmd)
}
```

- [ ] **단계 5: 테스트 실행해 통과 확인 + 전체**

실행:
```bash
go test ./cmd/ -run 'TestRecordHistory|TestRunUndo' -v
go build -o purgefs . && go vet ./... && go test ./...
rm -f purgefs
```
기대: 대상 테스트 PASS; 전체 빌드·vet·테스트 PASS(기존 `TestRunPurge*` 도 통과 — 출력 꼬리에 기록 연결이 추가돼도 `~/.purgefs` 저장은 성공 시 출력 없음).

- [ ] **단계 6: 커밋**

```bash
git add cmd/purge.go cmd/undo.go cmd/undo_test.go
git commit -m "feat: record trash manifest on purge and add undo command"
```

---

### Task 4: 개발자 프리셋(--preset)

**파일:**
- 수정: `internal/engine/rule.go`
- 수정: `internal/engine/rule_test.go`
- 수정: `cmd/purge.go`
- 수정: `cmd/purge_test.go`

**인터페이스:**
- 생산: `func Preset(name string) ([]Rule, bool)` (engine); `purgeCmd` 의 `--preset` 플래그. `runPurge`/`runPurgeInteractive` 가 규칙 목록을 인자로 받도록 바뀐다.

- [ ] **단계 1: 실패 테스트 작성**

`internal/engine/rule_test.go` 에 추가:

```go
func TestPresetDevCaches(t *testing.T) {
	rules, ok := Preset("dev-caches")
	if !ok {
		t.Fatal("dev-caches preset must exist")
	}
	if cat, _, ok := matchRules(rules, &Entry{Path: "/p/node_modules", IsDir: true}); !ok || cat != "node_modules" {
		t.Errorf("dev-caches should match node_modules, got (%q, %v)", cat, ok)
	}
	// dev-caches 는 OS junk(.DS_Store)를 포함하지 않는다.
	if _, _, ok := matchRules(rules, &Entry{Path: "/p/.DS_Store", IsDir: false}); ok {
		t.Error("dev-caches must not match .DS_Store")
	}
}

func TestPresetUnknown(t *testing.T) {
	if _, ok := Preset("nope"); ok {
		t.Error("unknown preset must return ok=false")
	}
}
```

`cmd/purge_test.go` 의 기존 `runPurge(...)` 호출은 새 시그니처에 맞춰 마지막 인자로 `engine.DefaultRules()` 를 넘기도록 바꾼다(아래 단계 3에서 시그니처 확정). 그리고 프리셋으로 분류 범위가 좁혀지는지 확인하는 테스트를 추가한다:

```go
func TestRunPurgeWithPresetSkipsOsJunk(t *testing.T) {
	root := junkDir(t) // node_modules + .DS_Store
	f := &fakeTrasher{}
	var buf bytes.Buffer

	rules, _ := engine.Preset("dev-caches")
	if err := runPurge(&buf, strings.NewReader("y\n"), root, f, false, false, rules); err != nil {
		t.Fatalf("runPurge: %v", err)
	}
	for _, p := range f.got {
		if strings.HasSuffix(p, ".DS_Store") {
			t.Errorf("dev-caches preset must not purge .DS_Store, got %q", p)
		}
	}
}
```

- [ ] **단계 2: 테스트 실행해 실패 확인**

실행: `go test ./internal/engine/ -run TestPreset -v`
기대: FAIL — `undefined: Preset`.

- [ ] **단계 3: 구현**

`internal/engine/rule.go` 에 `Preset` 을 추가한다:

```go
// Preset 은 이름에 해당하는 규칙 집합을 반환한다. 없으면 ok=false. dev-caches 는 빌드·의존성
// 캐시 디렉토리만 대상으로 하고 OS junk(.DS_Store)는 제외한다.
func Preset(name string) ([]Rule, bool) {
	switch name {
	case "dev-caches":
		return []Rule{
			dirNameRule{name: "node_modules", category: "node_modules"},
			dirNameRule{name: "target", category: "build-cache"},
			dirNameRule{name: "build", category: "build-cache"},
			dirNameRule{name: ".gradle", category: "build-cache"},
			dirNameRule{name: "dist", category: "build-cache"},
			dirNameRule{name: "__pycache__", category: "python-cache"},
		}, true
	default:
		return nil, false
	}
}
```

`cmd/purge.go` 에서 `runPurge` 와 `runPurgeInteractive` 가 규칙을 인자로 받도록 바꾼다:

- `runPurge` 시그니처: `func runPurge(w io.Writer, in io.Reader, path string, tr trash.Trasher, hard, assumeYes bool, rules []engine.Rule) error`, 내부의 `engine.Classify(report, engine.DefaultRules())` 를 `engine.Classify(report, rules)` 로.
- `runPurgeInteractive` 시그니처: `func runPurgeInteractive(w io.Writer, path string, tr trash.Trasher, rules []engine.Rule) error`, 내부의 `engine.DefaultRules()` 를 `rules` 로.

`purgeCmd` 에 `--preset` 플래그 변수·등록을 더하고, `RunE` 에서 규칙을 고른다:

```go
var purgePreset string
```

`init` 에 추가:

```go
	purgeCmd.Flags().StringVar(&purgePreset, "preset", "", "규칙 프리셋(예: dev-caches)")
```

`RunE` 에서 `guardRoot` 통과·`tr` 선택 뒤, 규칙을 정하고 호출부에 넘긴다:

```go
		rules := engine.DefaultRules()
		if purgePreset != "" {
			r, ok := engine.Preset(purgePreset)
			if !ok {
				return fmt.Errorf("unknown preset %q (available: dev-caches)", purgePreset)
			}
			rules = r
		}
		if purgeInteractive {
			return runPurgeInteractive(cmd.OutOrStdout(), path, tr, rules)
		}
		return runPurge(cmd.OutOrStdout(), cmd.InOrStdin(), path, tr, purgeHard, purgeYes, rules)
```

그리고 `cmd/purge_test.go` 의 기존 `runPurge(&buf, ..., false, false)` 호출들을 `runPurge(&buf, ..., false, false, engine.DefaultRules())` 로 바꾼다(모든 `TestRunPurge*`). `cmd/purge_test.go` 상단 import 에 `engine` 가 없으면 추가한다.

- [ ] **단계 4: 테스트 실행해 통과 확인 + 전체**

실행:
```bash
go test ./internal/engine/ -run TestPreset -v
go build -o purgefs . && go vet ./... && go test ./...
mkdir -p /tmp/pf-demo/node_modules && echo x > /tmp/pf-demo/.DS_Store && head -c 1024 /dev/zero > /tmp/pf-demo/node_modules/a.bin
printf 'y\n' | ./purgefs purge /tmp/pf-demo --preset dev-caches   # .DS_Store 는 남고 node_modules 만
ls /tmp/pf-demo/.DS_Store && echo "(.DS_Store 유지됨)"
rm -rf /tmp/pf-demo purgefs
```
기대: 빌드·vet·전체 테스트 PASS; 프리셋 실행 시 `.DS_Store` 는 남고 `node_modules` 만 정리.

- [ ] **단계 5: 커밋**

```bash
git add internal/engine/rule.go internal/engine/rule_test.go cmd/purge.go cmd/purge_test.go
git commit -m "feat: add --preset flag with dev-caches rule set"
```

---

## 자체 검토

**1. 스펙 커버리지 (P5 범위):**
- history 매니페스트 → Task 2 `internal/history`. ✓
- `undo` 복원 → Task 3 `undoCmd`/`runUndoDir`, Task 1 이동 매핑. ✓
- `--preset dev-caches` → Task 4 `engine.Preset` + `--preset`. ✓
- 설계상 P5 범위 밖: Linux 휴지통(XDG), config 파일 커스텀 규칙, Wails GUI, staleness 스코어링.

**2. Placeholder 스캔:** TBD/TODO 없음. 모든 코드·테스트 단계 완결. ✓

**3. 타입 일관성:** `trash.Moved{Original,Dest}`, `trash.Result.Moved`, `history.Item/Manifest/Save/LoadLatest/Restore`, `recordHistory(dir, res, createdAt)`, `runUndoDir(w, dir)`, `engine.Preset(name)([]Rule,bool)`, 변경된 `runPurge(...,rules)`/`runPurgeInteractive(...,rules)` 가 태스크·테스트에서 동일. `movedToItems([]trash.Moved) []history.Item` 필드명(Original/Dest) 일치. ✓

## 참고

- `undo` 는 **최근 1회**만 되돌린다. 여러 단계 undo·특정 시점 복원은 이후 확장.
- 원본 자리에 이미 뭔가 있으면 덮어쓰지 않고 건너뛴다(데이터 보호). 사용자가 그 사이 새로 만든 파일을 지키기 위함.
- `--hard` 는 매니페스트가 없어 `undo` 대상이 아니다. 이는 완전 삭제의 정의상 당연하며, `undo` 는 "복원할 기록이 없습니다" 로 안내한다.
- P4 이후 이월: `--hard`+`-i` 시 TUI 헤더에 완전삭제 경고 표기.
