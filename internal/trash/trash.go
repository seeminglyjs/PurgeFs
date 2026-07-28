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
		r.Moved = append(r.Moved, Moved{Original: p, Dest: dest})
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
