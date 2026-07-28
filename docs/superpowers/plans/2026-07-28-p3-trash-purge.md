# P3 휴지통 + purge 구현 계획

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**목표:** `purgefs purge PATH` 로 감지된 junk를 삭제한다. 기본은 macOS 휴지통(`~/.Trash`)으로 이동(복구 가능), `--hard` 면 완전 삭제. 삭제 전 반드시 확인하며, `--yes` 로 생략 가능. 위험한 루트(`/`, 홈)는 거부한다.

**아키텍처:** 새 `internal/trash` 패키지가 `Trasher` 인터페이스와 두 구현(macOS 휴지통, 완전 삭제)을 제공한다. `cmd/purge.go` 의 `runPurge` 가 P2 의 `engine.Classify` 로 삭제 대상을 모아 요약을 보여주고, 확인을 받은 뒤 주입된 `Trasher` 로 처리한다. `Trasher`·입력(`io.Reader`)·출력(`io.Writer`)을 주입해 테스트 가능하게 한다.

**기술 스택:** Go, 표준 라이브러리만. `cobra` 이미 연결됨.

## 전역 제약 (모든 태스크에 적용)

- Go 모듈 `github.com/seeminglyjs/PurgeFs`; `go.mod` 하한 `go 1.24`.
- `internal/trash` 는 표준 라이브러리만. `internal/engine` 은 P1~P2 그대로 두고 수정하지 않는다.
- **코드 주석은 반드시 한국어**, **문서는 한국어**(코드 블록·셸 명령·식별자·에러 문자열·커밋 메시지는 그대로).
- 커밋에 Claude / 어떤 AI 도 co-author·트레일러로 넣지 않는다. Conventional-commit 접두어.
- **삭제는 위험하다.** 스캔 루트 밖은 절대 삭제하지 않는다. 기본은 휴지통(복구 가능), `--hard` 만 완전 삭제. 확인 없이 삭제 금지(`--yes` 예외).

## 설계 판단

- 삭제 대상 = `engine.Classify` 결과의 `CategoryGroup.Paths` 를 평탄화한 목록. 새 엔진 함수 없이 P2 를 재사용한다.
- `Trasher.Trash(paths)` 는 경로별 실패를 모으고 계속 진행한다(하나 실패해도 전체 중단 안 함). 반환 `Result{Trashed, Failed}`.
- macOS 휴지통 이동은 `os.Rename` 으로 한다. 같은 볼륨이면 즉시. 다른 볼륨(예: 외장 디스크)이면 `os.Rename` 이 실패하고 `Failed` 에 기록된다(cross-device 복사 폴백은 나중 최적화).
- 매니페스트 기록/`undo` 는 P5. P3 는 삭제까지만.

## 파일 구조

- 생성 `internal/trash/trash.go` — `Trasher`, `Result`, `Failure`, `macTrasher`(+`NewMacTrasher`), `hardDeleter`(+`NewHardDeleter`), `uniqueDest`.
- 생성 `internal/trash/trash_test.go`.
- 수정 `cmd/purge.go` — 스텁을 `runPurge` 코어 + `purgeCmd` 배선 + `guardRoot` 로 교체.
- 생성 `cmd/purge_test.go`.

---

### Task 1: Trasher 인터페이스 + 구현

**파일:**
- 생성: `internal/trash/trash.go`
- 테스트: `internal/trash/trash_test.go`

**인터페이스:**
- 소비: 표준 라이브러리만.
- 생산:
  - `type Trasher interface { Trash(paths []string) Result }`
  - `type Result struct { Trashed []string; Failed []Failure }`, `type Failure struct { Path string; Err error }`
  - `func NewMacTrasher() (Trasher, error)` — `~/.Trash` 사용.
  - `func NewHardDeleter() Trasher` — 완전 삭제.

- [ ] **단계 1: 실패 테스트 작성**

`internal/trash/trash_test.go` 생성:

```go
package trash

import (
	"os"
	"path/filepath"
	"testing"
)

// newMacTrasherAt 는 테스트용으로 지정한 디렉토리를 휴지통으로 쓰는 macTrasher 를 만든다.
func newMacTrasherAt(t *testing.T, dir string) *macTrasher {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir trash: %v", err)
	}
	return &macTrasher{trashDir: dir}
}

func TestMacTrasherMovesToTrash(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "junk.txt")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	trashDir := filepath.Join(base, "Trash")
	tr := newMacTrasherAt(t, trashDir)

	res := tr.Trash([]string{src})

	if len(res.Trashed) != 1 || len(res.Failed) != 0 {
		t.Fatalf("Result = %+v, want 1 trashed 0 failed", res)
	}
	if _, err := os.Lstat(src); !os.IsNotExist(err) {
		t.Errorf("source still exists: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(trashDir, "junk.txt")); err != nil {
		t.Errorf("not moved into trash: %v", err)
	}
}

func TestMacTrasherRenamesOnCollision(t *testing.T) {
	base := t.TempDir()
	trashDir := filepath.Join(base, "Trash")
	tr := newMacTrasherAt(t, trashDir)
	// 휴지통에 이미 "dup" 이 있음
	if err := os.WriteFile(filepath.Join(trashDir, "dup"), []byte("old"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	src := filepath.Join(base, "dup")
	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	res := tr.Trash([]string{src})

	if len(res.Trashed) != 1 {
		t.Fatalf("Result = %+v, want 1 trashed", res)
	}
	if _, err := os.Lstat(filepath.Join(trashDir, "dup 2")); err != nil {
		t.Errorf("collision not renamed to \"dup 2\": %v", err)
	}
}

func TestMacTrasherRecordsFailure(t *testing.T) {
	base := t.TempDir()
	trashDir := filepath.Join(base, "Trash")
	tr := newMacTrasherAt(t, trashDir)

	res := tr.Trash([]string{filepath.Join(base, "does-not-exist")})

	if len(res.Trashed) != 0 || len(res.Failed) != 1 {
		t.Fatalf("Result = %+v, want 0 trashed 1 failed", res)
	}
}

func TestHardDeleterRemoves(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "cache")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "f"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	res := NewHardDeleter().Trash([]string{dir})

	if len(res.Trashed) != 1 || len(res.Failed) != 0 {
		t.Fatalf("Result = %+v, want 1 trashed", res)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Errorf("dir still exists: %v", err)
	}
}
```

- [ ] **단계 2: 테스트 실행해 실패 확인**

실행: `export PATH="/opt/homebrew/bin:$PATH"; go test ./internal/trash/ -v`
기대: FAIL — `undefined: macTrasher` / `undefined: NewHardDeleter`.

- [ ] **단계 3: 구현 작성**

`internal/trash/trash.go` 생성 (주석 한국어):

```go
// Package trash 는 파일·디렉토리를 macOS 휴지통(~/.Trash)으로 옮기거나 완전 삭제한다.
// Trasher 하나로 두 전략을 추상화해, 상위(cmd)는 어느 쪽인지 몰라도 된다.
package trash

import (
	"os"
	"path/filepath"
	"strconv"
)

// Failure 는 처리하지 못한 경로 하나를 기록한다.
type Failure struct {
	Path string
	Err  error
}

// Result 는 한 번의 정리 작업 결과다. 경로별 실패는 Failed 에 모이고 나머지는 계속 처리된다.
type Result struct {
	Trashed []string
	Failed  []Failure
}

// Trasher 는 경로들을 정리하는 방식을 추상화한다. 구현: macOS 휴지통 이동 또는 완전 삭제.
type Trasher interface {
	Trash(paths []string) Result
}

// macTrasher 는 지정한 휴지통 디렉토리(~/.Trash)로 이동한다.
type macTrasher struct {
	trashDir string
}

// NewMacTrasher 는 홈의 ~/.Trash 를 쓰는 Trasher 를 만든다.
func NewMacTrasher() (Trasher, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".Trash")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &macTrasher{trashDir: dir}, nil
}

// Trash 는 각 경로를 휴지통으로 옮긴다. 이름 충돌 시 "name 2" 식으로 바꿔 옮긴다. 다른
// 볼륨이면 os.Rename 이 실패하고 Failed 에 기록된다.
func (m *macTrasher) Trash(paths []string) Result {
	var r Result
	for _, p := range paths {
		dest := uniqueDest(m.trashDir, filepath.Base(p))
		if err := os.Rename(p, dest); err != nil {
			r.Failed = append(r.Failed, Failure{Path: p, Err: err})
			continue
		}
		r.Trashed = append(r.Trashed, p)
	}
	return r
}

// hardDeleter 는 완전 삭제한다(복구 불가).
type hardDeleter struct{}

// NewHardDeleter 는 완전 삭제하는 Trasher 를 만든다.
func NewHardDeleter() Trasher { return hardDeleter{} }

// Trash 는 각 경로를 완전히 지운다.
func (hardDeleter) Trash(paths []string) Result {
	var r Result
	for _, p := range paths {
		if err := os.RemoveAll(p); err != nil {
			r.Failed = append(r.Failed, Failure{Path: p, Err: err})
			continue
		}
		r.Trashed = append(r.Trashed, p)
	}
	return r
}

// uniqueDest 는 dir 안에서 name 이 충돌하지 않는 목적지 경로를 만든다. 이미 있으면
// "name 2", "name 3" ... 을 시도한다.
func uniqueDest(dir, name string) string {
	dest := filepath.Join(dir, name)
	if !exists(dest) {
		return dest
	}
	for i := 2; ; i++ {
		cand := filepath.Join(dir, name+" "+strconv.Itoa(i))
		if !exists(cand) {
			return cand
		}
	}
}

// exists 는 경로가 존재하면 true(심볼릭링크 자체도 존재로 침).
func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}
```

- [ ] **단계 4: 테스트 실행해 통과 확인**

실행: `go test ./internal/trash/ -v`
기대: PASS.

- [ ] **단계 5: 커밋**

```bash
git add internal/trash/trash.go internal/trash/trash_test.go
git commit -m "feat: add trash package with macOS trash and hard delete"
```

---

### Task 2: purge 코어(runPurge)

**파일:**
- 수정: `cmd/purge.go`
- 생성: `cmd/purge_test.go`

**인터페이스:**
- 소비: `engine.Scan`, `engine.Classify`, `engine.DefaultRules`, `engine.CategoryGroup`; `trash.Trasher`, `trash.Result`; `humanBytes`, `plural`.
- 생산: `func runPurge(w io.Writer, in io.Reader, path string, tr trash.Trasher, hard, assumeYes bool) error` — 스캔·분류해 삭제 대상을 요약하고, `assumeYes` 가 아니면 `in` 에서 확인을 받은 뒤 `tr.Trash` 로 처리한다. junk 가 없으면 아무것도 삭제하지 않고 안내만 한다.

- [ ] **단계 1: 실패 테스트 작성**

`cmd/purge_test.go` 생성:

```go
package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seeminglyjs/PurgeFs/internal/trash"
)

// fakeTrasher 는 실제로 지우지 않고 받은 경로만 기록한다.
type fakeTrasher struct {
	got []string
}

func (f *fakeTrasher) Trash(paths []string) trash.Result {
	f.got = append(f.got, paths...)
	return trash.Result{Trashed: paths}
}

// junkDir 는 node_modules(파일 포함)와 .DS_Store 가 있는 임시 디렉토리를 만든다.
func junkDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	nm := filepath.Join(root, "node_modules")
	if err := os.MkdirAll(nm, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nm, "big.bin"), make([]byte, 1024), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".DS_Store"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return root
}

func TestRunPurgeConfirmedTrashes(t *testing.T) {
	root := junkDir(t)
	f := &fakeTrasher{}
	var buf bytes.Buffer

	err := runPurge(&buf, strings.NewReader("y\n"), root, f, false, false)
	if err != nil {
		t.Fatalf("runPurge: %v", err)
	}
	if len(f.got) == 0 {
		t.Error("nothing was sent to the trasher after confirmation")
	}
	if !strings.Contains(buf.String(), "node_modules") {
		t.Errorf("summary missing node_modules:\n%s", buf.String())
	}
}

func TestRunPurgeCancelledDoesNothing(t *testing.T) {
	root := junkDir(t)
	f := &fakeTrasher{}
	var buf bytes.Buffer

	err := runPurge(&buf, strings.NewReader("n\n"), root, f, false, false)
	if err != nil {
		t.Fatalf("runPurge: %v", err)
	}
	if len(f.got) != 0 {
		t.Errorf("trasher was called despite cancel: %v", f.got)
	}
	if !strings.Contains(buf.String(), "취소") {
		t.Errorf("output missing cancel notice:\n%s", buf.String())
	}
}

func TestRunPurgeAssumeYesSkipsPrompt(t *testing.T) {
	root := junkDir(t)
	f := &fakeTrasher{}
	var buf bytes.Buffer

	// 입력이 비어도 --yes 면 진행해야 한다.
	err := runPurge(&buf, strings.NewReader(""), root, f, false, true)
	if err != nil {
		t.Fatalf("runPurge: %v", err)
	}
	if len(f.got) == 0 {
		t.Error("--yes should proceed without input")
	}
}

func TestRunPurgeNoJunk(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	f := &fakeTrasher{}
	var buf bytes.Buffer

	err := runPurge(&buf, strings.NewReader(""), root, f, false, true)
	if err != nil {
		t.Fatalf("runPurge: %v", err)
	}
	if len(f.got) != 0 {
		t.Errorf("clean dir should trash nothing: %v", f.got)
	}
}
```

- [ ] **단계 2: 테스트 실행해 실패 확인**

실행: `go test ./cmd/ -run TestRunPurge -v`
기대: FAIL — `undefined: runPurge`.

- [ ] **단계 3: purge.go 의 스텁을 runPurge 로 교체**

`cmd/purge.go` 전체를 다음으로 바꾼다 (주석 한국어). `purgeCmd` 배선·플래그·가드는 Task 3 에서 추가하므로, 지금은 `runPurge` 만 두고 `init`/`purgeCmd` 는 임시로 남긴다:

```go
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/seeminglyjs/PurgeFs/internal/engine"
	"github.com/seeminglyjs/PurgeFs/internal/trash"
	"github.com/spf13/cobra"
)

var purgeCmd = &cobra.Command{
	Use:   "purge [path]",
	Short: "Purge junk under a path (asks before deleting)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("purge wiring lands in P3 Task 3")
	},
}

// runPurge 는 path 를 스캔·분류해 삭제 대상을 요약하고, assumeYes 가 아니면 in 에서 확인을
// 받은 뒤 tr 로 처리한다. hard 는 확인 문구를 완전 삭제용으로 바꾸는 데만 쓴다. junk 가
// 없으면 아무것도 삭제하지 않는다.
func runPurge(w io.Writer, in io.Reader, path string, tr trash.Trasher, hard, assumeYes bool) error {
	report, _, err := engine.Scan(path)
	if err != nil {
		return err
	}
	groups := engine.Classify(report, engine.DefaultRules())
	if len(groups) == 0 {
		fmt.Fprintln(w, "정리할 junk가 없습니다.")
		return nil
	}

	var paths []string
	var total int64
	for _, g := range groups {
		paths = append(paths, g.Paths...)
		total += g.Size
	}

	action := "휴지통으로 이동"
	if hard {
		action = "완전 삭제(복구 불가)"
	}
	fmt.Fprintf(w, "%s 대상: %s, 항목 %d개\n", action, humanBytes(total), len(paths))
	for _, g := range groups {
		fmt.Fprintf(w, "  %10s  %-14s (%d %s)\n",
			humanBytes(g.Size), g.Category, g.Count, plural(g.Count, "item", "items"))
	}

	if !assumeYes {
		fmt.Fprint(w, "진행할까요? [y/N] ")
		if !confirmed(in) {
			fmt.Fprintln(w, "취소했습니다.")
			return nil
		}
	}

	res := tr.Trash(paths)
	fmt.Fprintf(w, "처리 %d개", len(res.Trashed))
	if len(res.Failed) > 0 {
		fmt.Fprintf(w, ", 실패 %d개", len(res.Failed))
	}
	fmt.Fprintln(w)
	return nil
}

// confirmed 는 in 에서 한 줄 읽어 y/yes(대소문자 무시)면 true.
func confirmed(in io.Reader) bool {
	s := bufio.NewScanner(in)
	if !s.Scan() {
		return false
	}
	ans := strings.ToLower(strings.TrimSpace(s.Text()))
	return ans == "y" || ans == "yes"
}

func init() {
	rootCmd.AddCommand(purgeCmd)
}
```

- [ ] **단계 4: 테스트 실행해 통과 확인**

실행: `go test ./cmd/ -run TestRunPurge -v`
기대: PASS.

- [ ] **단계 5: 커밋**

```bash
git add cmd/purge.go cmd/purge_test.go
git commit -m "feat: add runPurge core (classify, confirm, trash)"
```

---

### Task 3: purge 커맨드 배선 + 위험 루트 가드

**파일:**
- 수정: `cmd/purge.go`
- 수정: `cmd/purge_test.go`

**인터페이스:**
- 소비: `runPurge` (Task 2); `trash.NewMacTrasher`, `trash.NewHardDeleter` (Task 1).
- 생산: `func guardRoot(path string) error` — 위험한 루트(`/`, 홈 디렉토리)면 에러; 안전하면 nil. `purgeCmd.RunE` 가 플래그(`--yes`, `--hard`)를 읽고, `guardRoot` 를 통과하면 알맞은 `Trasher` 로 `runPurge` 를 호출한다.

- [ ] **단계 1: 실패 테스트 작성**

`cmd/purge_test.go` 에 추가:

```go
func TestGuardRootRefusesDangerous(t *testing.T) {
	if err := guardRoot("/"); err == nil {
		t.Error("guardRoot(/) must return an error")
	}
	home, err := os.UserHomeDir()
	if err == nil {
		if err := guardRoot(home); err == nil {
			t.Errorf("guardRoot(home=%q) must return an error", home)
		}
	}
}

func TestGuardRootAllowsSubdir(t *testing.T) {
	if err := guardRoot(t.TempDir()); err != nil {
		t.Errorf("guardRoot(tempdir) should be allowed: %v", err)
	}
}
```

- [ ] **단계 2: 테스트 실행해 실패 확인**

실행: `go test ./cmd/ -run TestGuardRoot -v`
기대: FAIL — `undefined: guardRoot`.

- [ ] **단계 3: guardRoot + purgeCmd 배선 작성**

`cmd/purge.go` 에서 (1) import 에 `os`, `path/filepath` 추가, (2) 임시 `purgeCmd` 를 아래로 교체, (3) `guardRoot` 추가, (4) `init` 에 플래그 등록.

`purgeCmd` 와 `init` 교체:

```go
var (
	purgeYes  bool
	purgeHard bool
)

var purgeCmd = &cobra.Command{
	Use:   "purge [path]",
	Short: "Purge junk under a path (asks before deleting)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) == 1 {
			path = args[0]
		}
		if err := guardRoot(path); err != nil {
			return err
		}

		var tr trash.Trasher
		if purgeHard {
			tr = trash.NewHardDeleter()
		} else {
			t, err := trash.NewMacTrasher()
			if err != nil {
				return err
			}
			tr = t
		}
		return runPurge(cmd.OutOrStdout(), cmd.InOrStdin(), path, tr, purgeHard, purgeYes)
	},
}

// guardRoot 는 위험한 루트를 거부한다: 파일시스템 루트(/)와 홈 디렉토리. 사용자가 실수로
// 거대한 영역을 통째로 정리하는 것을 막는다.
func guardRoot(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	abs = filepath.Clean(abs)
	if abs == string(filepath.Separator) {
		return fmt.Errorf("refusing to purge filesystem root %q", abs)
	}
	if home, err := os.UserHomeDir(); err == nil && abs == filepath.Clean(home) {
		return fmt.Errorf("refusing to purge home directory %q", abs)
	}
	return nil
}

func init() {
	purgeCmd.Flags().BoolVar(&purgeYes, "yes", false, "확인 없이 진행")
	purgeCmd.Flags().BoolVar(&purgeHard, "hard", false, "휴지통이 아니라 완전 삭제")
	rootCmd.AddCommand(purgeCmd)
}
```

이때 파일에는 `init` 이 하나만 있어야 한다(Task 2 의 임시 `init` 을 이 것으로 대체).

- [ ] **단계 4: 테스트 실행해 통과 확인**

실행: `go test ./cmd/ -run TestGuardRoot -v`
기대: PASS.

- [ ] **단계 5: 전체 빌드 + 테스트 + 수동 확인**

실행:
```bash
go build -o purgefs . && go vet ./... && go test ./...
mkdir -p /tmp/pf-demo/node_modules && echo x > /tmp/pf-demo/.DS_Store && head -c 2048 /dev/zero > /tmp/pf-demo/node_modules/a.bin
printf 'n\n' | ./purgefs purge /tmp/pf-demo    # 취소 경로 확인
./purgefs purge /tmp/pf-demo --yes             # 휴지통 이동
ls ~/.Trash | grep node_modules || true
rm -rf /tmp/pf-demo purgefs
```
기대: 빌드 OK; vet 클린; 전체 테스트 PASS; 취소 시 아무것도 안 지움; `--yes` 시 `node_modules`·`.DS_Store` 가 `~/.Trash` 로 이동.

- [ ] **단계 6: 커밋**

```bash
git add cmd/purge.go cmd/purge_test.go
git commit -m "feat: wire purge command with flags and dangerous-root guard"
```

---

## 자체 검토

**1. 스펙 커버리지 (P3 범위):**
- macOS 휴지통(~/.Trash 이동, 충돌 rename) → Task 1 `macTrasher`. ✓
- `--hard` 완전 삭제 → Task 1 `hardDeleter`, Task 3 플래그. ✓
- `purge` 확인 후 실행 → Task 2 `runPurge`(확인), Task 3 배선. ✓
- 안전(스캔 루트 밖 삭제 안 함 — 대상은 스캔 결과에서만 나옴; 위험 루트 거부) → Task 3 `guardRoot`. ✓
- 설계상 P3 범위 밖: 매니페스트/`undo`(P5), TUI 선택(P4), 카테고리별 선택 삭제.

**2. Placeholder 스캔:** TBD/TODO 없음. Task 2 의 임시 `purgeCmd`(P3 Task 3 에서 교체 명시)는 의도된 단계적 구현이며 Task 3 에서 완성된다. ✓

**3. 타입 일관성:** `Trasher.Trash(paths) Result`, `Result{Trashed, Failed}`, `runPurge(io.Writer, io.Reader, string, trash.Trasher, bool, bool) error`, `guardRoot(string) error` 가 태스크와 테스트에서 동일. `fakeTrasher` 가 `Trasher` 인터페이스를 만족(`Trash([]string) trash.Result`). ✓

## 참고

- 크로스 볼륨: `os.Rename` 은 다른 파일시스템에서 실패한다. P3 는 이를 `Failed` 로 보고만 하고, 복사+삭제 폴백은 나중 최적화다.
- P3 는 매니페스트를 남기지 않으므로 `--hard` 는 되돌릴 수 없다. 휴지통 이동은 Finder 에서 복구 가능, 프로그램적 `undo` 는 P5.
- P2 최종 리뷰에서 넘어온 사항: `build`/`dist` 는 이름만으로 매치되므로, purge 가 손대기 전에 정말 빌드 캐시인지 가드를 P3 이후 논의. 현재는 확인 절차가 사용자 보호선.
