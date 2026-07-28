package engine

import (
	"os"
	"path/filepath"
)

// Walk 은 root 를 순회하고 하위 트리 용량이 집계된 root Entry 를 반환한다. 심볼릭링크는
// leaf 엔트리(자기 크기)로 기록하고 절대 따라가지 않는다. 경로별 에러(예: 권한 거부)는
// 반환 슬라이스에 모으고 순회는 계속된다. 반환 error 는 root 자체를 stat 하지 못할 때만
// non-nil 이다.
func Walk(root string) (*Entry, []WalkError, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, nil, err
	}
	var werrs []WalkError
	e := walkEntry(root, info, &werrs)
	return e, werrs, nil
}

// walkEntry 는 path 의 Entry 를 만들고, 디렉토리면 자식으로 재귀한다. 용량은 위로
// 올라온다: 재귀가 돌아오면서 디렉토리 Size 에 자식 크기가 누적되므로 root 가 총합을
// 갖게 된다. werrs 는 포인터로 넘겨, 트리 깊은 곳의 실패가 하나의 공유 슬라이스에 모인다.
func walkEntry(path string, info os.FileInfo, werrs *[]WalkError) *Entry {
	e := &Entry{
		Path:    path,
		IsDir:   info.IsDir(),
		ModTime: info.ModTime().Unix(),
	}
	// 기저 조건: 파일은 leaf — 자기 크기만 기록하고 멈춘다.
	if !info.IsDir() {
		e.Size = info.Size()
		return e
	}

	// 디렉토리 목록 읽기가 실패할 수 있다(권한). 기록하고, 전체 순회를 중단하는 대신
	// 해당 디렉토리를 빈 노드로 반환한다.
	dirents, err := os.ReadDir(path)
	if err != nil {
		*werrs = append(*werrs, WalkError{Path: path, Err: err})
		return e
	}

	for _, de := range dirents {
		childPath := filepath.Join(path, de.Name())
		// de.Info() 는 lstat 기반이라 심볼릭링크가 가리키는 대상이 아니라 심볼릭링크
		// 그 자체로 보고된다 — 이게 링크를 안 따라가는 방법이다.
		ci, err := de.Info()
		if err != nil {
			*werrs = append(*werrs, WalkError{Path: childPath, Err: err})
			continue
		}
		// 심볼릭링크: 링크 자체를 leaf(자기 작은 크기)로 기록하고 내려가지 않는다.
		// 따라가면 스캔 루트를 벗어나거나, 실제 하위 트리를 중복 집계하거나, 나중에
		// 사용자가 고르지 않은 것을 삭제할 수 있다.
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
		// 일반 파일 또는 디렉토리: 재귀한 뒤 자식 크기를 합산한다.
		child := walkEntry(childPath, ci, werrs)
		e.Children = append(e.Children, child)
		e.Size += child.Size
	}
	return e
}
