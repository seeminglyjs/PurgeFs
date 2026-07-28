package engine

import (
	"os"
	"path/filepath"
)

// Scan 은 root 를 순회하며 용량을 집계하고 rules 로 junk 를 분류한 Report 를 만든다. 순회와
// 분류를 한 번에 하므로 노드별 데이터가 남지 않는다 — 큰 트리에서도 결과 크기만큼만 쓴다.
//
// 명명된 root 는 먼저 filepath.EvalSymlinks 로 resolve 한다(순회는 심볼릭링크를, 심볼릭링크인
// root 까지도 따라가지 않는데 macOS 의 흔한 root 인 /tmp 등이 심볼릭링크다). resolve 가
// 실패하면(예: root 가 없음) 원본 root 를 순회해 진짜 에러를 드러내게 한다.
func Scan(root string, rules []Rule) (*Report, []WalkError, error) {
	scanRoot := root
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		scanRoot = resolved
	}
	info, err := os.Lstat(scanRoot)
	if err != nil {
		return nil, nil, err
	}

	w := &walker{
		rules:  rules,
		groups: map[string]*CategoryGroup{},
		report: &Report{Root: scanRoot},
	}
	w.report.TotalSize = w.walkTop(scanRoot, info)
	w.report.Groups = sortedGroups(w.groups)
	return w.report, w.werrs, nil
}
