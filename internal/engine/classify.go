package engine

import "sort"

// CategoryGroup 은 한 카테고리로 묶인 회수 가능 항목들의 요약이다.
type CategoryGroup struct {
	Category string
	Size     int64
	Count    int
	Paths    []string
}

// sortedGroups 는 순회 중 모은 그룹을 Size 내림차순으로 펴서 돌려준다. 같은 크기면 카테고리
// 이름 오름차순으로 tie-break 해 순서를 결정적으로 만든다(map 순회가 무작위라 tie-break 이
// 없으면 실행마다 순서가 달라질 수 있음).
func sortedGroups(groups map[string]*CategoryGroup) []CategoryGroup {
	out := make([]CategoryGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Size != out[j].Size {
			return out[i].Size > out[j].Size
		}
		return out[i].Category < out[j].Category
	})
	return out
}
