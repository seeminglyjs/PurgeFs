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
