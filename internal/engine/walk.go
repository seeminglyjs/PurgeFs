package engine

import (
	"os"
	"path/filepath"
)

// walker 는 한 번의 순회 상태다. 노드를 트리로 쌓지 않고 지나가면서 크기를 집계하고 규칙에
// 걸리는 것만 그룹에 남긴다 — 그래서 메모리가 트리 크기가 아니라 결과 크기에 비례한다.
type walker struct {
	rules  []Rule
	groups map[string]*CategoryGroup
	report *Report
	werrs  []WalkError
}

// walk 는 path 하위를 순회하고 하위 트리 전체 크기를 반환한다. siblings 는 path 와 같은
// 디렉토리에 있는 항목 이름들로, 마커를 보는 규칙이 쓴다.
//
// classify 가 false 면 크기만 집계하고 분류는 건너뛴다. 이미 매치된 디렉토리(node_modules
// 처럼 통째가 한 회수 단위)의 하위에서 쓴다 — 부모 집계와 그룹 크기에 하위 크기가 필요하니
// 순회는 계속하되, 그 안을 또 분류하면 안 된다.
func (w *walker) walk(path string, info os.FileInfo, siblings map[string]bool, classify bool) int64 {
	e := Entry{Path: path, IsDir: info.IsDir(), ModTime: info.ModTime().Unix()}

	// 기저 조건: 파일은 leaf — 자기 크기만 세고 멈춘다.
	if !e.IsDir {
		w.report.FileCount++
		e.Size = info.Size()
		if classify {
			if cat, _, ok := matchRules(w.rules, &e, siblings); ok {
				w.add(cat, e.Path, e.Size)
			}
		}
		return e.Size
	}
	w.report.DirCount++

	// 디렉토리 목록 읽기가 실패할 수 있다(권한). 기록하고, 전체 순회를 중단하는 대신
	// 크기 0 으로 넘어간다.
	dirents, err := os.ReadDir(path)
	if err != nil {
		w.werrs = append(w.werrs, WalkError{Path: path, Err: err})
		return 0
	}

	// 디렉토리는 하위를 다 돌아야 크기를 알 수 있다. 판정은 여기서 하고(하위를 분류할지
	// 정해야 하므로) 그룹 반영은 크기가 확정되는 아래에서 한다.
	var cat string
	var matched bool
	if classify {
		var skipChildren bool
		cat, skipChildren, matched = matchRules(w.rules, &e, siblings)
		if skipChildren {
			classify = false
		}
	}

	// 자식들의 형제 이름 집합은 여기서 한 번만 만든다. 규칙마다 다시 훑으면 큰 디렉토리에서
	// 비용이 규칙 수만큼 곱해진다. 재귀가 돌아오면 버려지므로 한 번에 경로 깊이만큼만 산다.
	childSiblings := make(map[string]bool, len(dirents))
	for _, de := range dirents {
		childSiblings[de.Name()] = true
	}

	var total int64
	for _, de := range dirents {
		childPath := filepath.Join(path, de.Name())
		// de.Info() 는 lstat 기반이라 심볼릭링크가 가리키는 대상이 아니라 심볼릭링크
		// 그 자체로 보고된다 — 이게 링크를 안 따라가는 방법이다.
		ci, err := de.Info()
		if err != nil {
			w.werrs = append(w.werrs, WalkError{Path: childPath, Err: err})
			continue
		}
		// 심볼릭링크: 링크 자체를 leaf 로 세고 내려가지 않는다. 따라가면 스캔 루트를 벗어나거나,
		// 실제 하위 트리를 중복 집계하거나, 나중에 사용자가 고르지 않은 것을 삭제할 수 있다.
		if ci.Mode()&os.ModeSymlink != 0 {
			w.report.FileCount++
			total += ci.Size()
			continue
		}
		total += w.walk(childPath, ci, childSiblings, classify)
	}

	if matched {
		w.add(cat, path, total)
	}
	return total
}

// walkTop 은 root 를 순회하되 root 바로 아래 항목의 크기를 따로 모아 Report.Children 에 담는다.
// scan 이 "무엇이 용량을 먹는지" 보여주는 데 필요한 유일한 노드별 정보다.
func (w *walker) walkTop(root string, info os.FileInfo) int64 {
	if !info.IsDir() {
		return w.walk(root, info, nil, true)
	}
	w.report.DirCount++

	dirents, err := os.ReadDir(root)
	if err != nil {
		w.werrs = append(w.werrs, WalkError{Path: root, Err: err})
		return 0
	}
	childSiblings := make(map[string]bool, len(dirents))
	for _, de := range dirents {
		childSiblings[de.Name()] = true
	}

	var total int64
	for _, de := range dirents {
		childPath := filepath.Join(root, de.Name())
		ci, err := de.Info()
		if err != nil {
			w.werrs = append(w.werrs, WalkError{Path: childPath, Err: err})
			continue
		}
		var size int64
		if ci.Mode()&os.ModeSymlink != 0 {
			w.report.FileCount++
			size = ci.Size()
		} else {
			size = w.walk(childPath, ci, childSiblings, true)
		}
		w.report.Children = append(w.report.Children, Child{Path: childPath, Size: size})
		total += size
	}
	return total
}

// add 는 매치된 항목을 카테고리 그룹에 누적한다.
func (w *walker) add(category, path string, size int64) {
	g := w.groups[category]
	if g == nil {
		g = &CategoryGroup{Category: category}
		w.groups[category] = g
	}
	g.Size += size
	g.Count++
	g.Paths = append(g.Paths, path)
}
