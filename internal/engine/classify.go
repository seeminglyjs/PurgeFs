package engine

import (
	"path/filepath"
	"sort"
)

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
	// root 는 부모가 없어 형제도 없다.
	classifyEntry(report.Root, nil, rules, groups)

	out := make([]CategoryGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, *g)
	}
	// Size 내림차순. 같은 크기면 카테고리 이름 오름차순으로 tie-break 해서 순서를 결정적으로 만든다
	// (map 순회가 무작위라 tie-break 없으면 실행마다 순서가 달라질 수 있음).
	sort.Slice(out, func(i, j int) bool {
		if out[i].Size != out[j].Size {
			return out[i].Size > out[j].Size
		}
		return out[i].Category < out[j].Category
	})
	return out
}

func classifyEntry(e *Entry, siblings map[string]bool, rules []Rule, groups map[string]*CategoryGroup) {
	if cat, skipChildren, ok := matchRules(rules, e, siblings); ok {
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
	if len(e.Children) == 0 {
		return
	}
	// 자식들의 형제 집합은 이 디렉토리 안에서 한 번만 만들어 모든 자식이 공유한다. 규칙마다
	// 다시 훑으면 큰 디렉토리에서 비용이 규칙 수만큼 곱해진다.
	childSiblings := make(map[string]bool, len(e.Children))
	for _, c := range e.Children {
		childSiblings[filepath.Base(c.Path)] = true
	}
	for _, c := range e.Children {
		classifyEntry(c, childSiblings, rules, groups)
	}
}
