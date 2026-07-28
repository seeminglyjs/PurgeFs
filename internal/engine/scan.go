package engine

import "path/filepath"

// Scan 은 root 를 순회해 집계 용량과 노드 수가 담긴 Report 를 만든다. 명명된 root 는 먼저
// filepath.EvalSymlinks 로 resolve 한다(Walk 는 심볼릭링크를, 심볼릭링크인 root 까지도
// 따라가지 않는데, macOS 의 흔한 root 인 /tmp 등이 심볼릭링크다). resolve 가 실패하면
// (예: root 가 없음) 원본 root 를 순회해 Walk 가 진짜 에러를 드러내게 한다.
func Scan(root string) (*Report, []WalkError, error) {
	scanRoot := root
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		scanRoot = resolved
	}
	rootEntry, werrs, err := Walk(scanRoot)
	if err != nil {
		return nil, werrs, err
	}
	r := &Report{
		Root:      rootEntry,
		TotalSize: rootEntry.Size,
	}
	// TotalSize 는 이미 root 에 있다(Walk 가 집계함). 노드 수는 트리를 한 번 더 훑어야 한다.
	countEntries(rootEntry, r)
	return r, werrs, nil
}

// countEntries 는 Walk 가 만든 것과 같은 모양의 트리를 순회하며 파일·디렉토리 노드 수를
// r 에 집계한다(root 자체도 디렉토리로 센다).
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
