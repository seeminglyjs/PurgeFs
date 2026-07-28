package trash

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// xdgTrasher 는 freedesktop.org Trash 규격대로 정리한다. 파일은 <root>/files 로 옮기고,
// 같은 이름의 <root>/info/<name>.trashinfo 에 원본 경로와 삭제 시각을 남긴다. 이 짝이 있어야
// 파일 관리자가 휴지통 항목으로 인식하고 "복원"을 제공한다 — ~/.Trash 로 그냥 옮기면 리눅스
// 데스크톱은 아무것도 알아보지 못한다.
type xdgTrasher struct {
	filesDir string
	infoDir  string
}

// NewXDGTrasher 는 $XDG_DATA_HOME/Trash(없으면 ~/.local/share/Trash)를 쓰는 Trasher 를 만든다.
func NewXDGTrasher() (Trasher, error) {
	root := os.Getenv("XDG_DATA_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		root = filepath.Join(home, ".local", "share")
	}
	return newXDGTrasherIn(filepath.Join(root, "Trash"))
}

// newXDGTrasherIn 은 root 아래 files·info 디렉토리를 만들고 Trasher 를 돌려준다.
func newXDGTrasherIn(root string) (*xdgTrasher, error) {
	x := &xdgTrasher{
		filesDir: filepath.Join(root, "files"),
		infoDir:  filepath.Join(root, "info"),
	}
	for _, dir := range []string{x.filesDir, x.infoDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
	}
	return x, nil
}

// Trash 는 각 경로를 <root>/files 로 옮기고 짝이 되는 .trashinfo 를 쓴다. 규격대로 info 를
// 먼저 써서 이름을 선점하고, 이동에 실패하면 방금 쓴 info 를 지워 짝 없는 항목을 남기지 않는다.
func (x *xdgTrasher) Trash(paths []string) Result {
	var r Result
	for _, p := range paths {
		dest := uniqueDest(x.filesDir, filepath.Base(p))
		// dest 이름이 files 안에서 비어 있으므로 같은 이름의 info 도 짝이 없는 잔재다 — 덮어쓴다.
		info := filepath.Join(x.infoDir, filepath.Base(dest)+".trashinfo")
		if err := os.WriteFile(info, trashInfo(p, time.Now()), 0o600); err != nil {
			r.Failed = append(r.Failed, Failure{Path: p, Err: err})
			continue
		}
		if err := os.Rename(p, dest); err != nil {
			os.Remove(info)
			r.Failed = append(r.Failed, Failure{Path: p, Err: err})
			continue
		}
		r.Trashed = append(r.Trashed, p)
		r.Moved = append(r.Moved, Moved{Original: p, Dest: dest, Sidecar: info})
	}
	return r
}

// trashInfo 는 .trashinfo 본문을 만든다. 규격상 Path 는 URL 처럼 인코딩하되 디렉토리 구분자는
// 유지하고, DeletionDate 는 지역 시각의 ISO 8601 형식이다.
func trashInfo(original string, deletedAt time.Time) []byte {
	encoded := (&url.URL{Path: original}).EscapedPath()
	return fmt.Appendf(nil, "[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		encoded, deletedAt.Format("2006-01-02T15:04:05"))
}
