// Package tui 는 정리 대상을 대화형으로 고르는 bubbletea 화면을 제공한다. 선택 로직은
// 순수 함수로 두어 터미널 없이 테스트할 수 있다.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Item 은 정리 대상 한 묶음(카테고리)이다.
type Item struct {
	Label string
	Size  int64
	Paths []string
}

// Model 은 선택 화면의 상태다.
type Model struct {
	items     []Item
	cursor    int
	selected  map[int]bool
	format    func(int64) string
	confirmed bool
	quit      bool
}

// New 는 모든 항목이 선택된 상태로 시작하는 Model 을 만든다. format 은 용량을 사람이 읽는
// 문자열로 바꾼다.
func New(items []Item, format func(int64) string) Model {
	sel := make(map[int]bool, len(items))
	for i := range items {
		sel[i] = true
	}
	return Model{items: items, selected: sel, format: format}
}

// Init 은 초기 명령이 없다.
func (m Model) Init() tea.Cmd { return nil }

// Update 는 키 입력에 따라 커서 이동·토글·확정·취소를 처리한다.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case " ":
		m.selected[m.cursor] = !m.selected[m.cursor]
	case "enter":
		m.confirmed = true
		return m, tea.Quit
	case "q", "esc", "ctrl+c":
		m.quit = true
		return m, tea.Quit
	}
	return m, nil
}

// View 는 항목 목록·체크·용량 막대·선택 합계를 그린다.
func (m Model) View() string {
	if m.quit || m.confirmed {
		return ""
	}
	var b strings.Builder
	b.WriteString("정리할 항목 선택 (↑/↓ 이동, space 토글, enter 실행, q 취소)\n\n")

	var maxSize int64 = 1
	for _, it := range m.items {
		if it.Size > maxSize {
			maxSize = it.Size
		}
	}
	for i, it := range m.items {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		check := " "
		if m.selected[i] {
			check = "x"
		}
		fmt.Fprintf(&b, "%s [%s] %-14s %10s %s\n",
			cursor, check, it.Label, m.format(it.Size), sizeBar(it.Size, maxSize, 20))
	}
	fmt.Fprintf(&b, "\n선택 합계: %s\n", m.format(m.SelectedSize()))
	return b.String()
}

// SelectedSize 는 선택된 항목 크기의 합이다.
func (m Model) SelectedSize() int64 {
	var total int64
	for i, it := range m.items {
		if m.selected[i] {
			total += it.Size
		}
	}
	return total
}

// SelectedPaths 는 선택된 항목들의 경로를 평탄화해 반환한다.
func (m Model) SelectedPaths() []string {
	var paths []string
	for i, it := range m.items {
		if m.selected[i] {
			paths = append(paths, it.Paths...)
		}
	}
	return paths
}

// Confirmed 는 사용자가 enter 로 실행을 확정했는지 여부다.
func (m Model) Confirmed() bool { return m.confirmed }

// sizeBar 는 size/max 비율을 width 칸의 막대로 그린다.
func sizeBar(size, max int64, width int) string {
	if max <= 0 {
		return ""
	}
	n := int(int64(width) * size / max)
	if n < 0 {
		n = 0
	}
	if n > width {
		n = width
	}
	return strings.Repeat("█", n) + strings.Repeat("░", width-n)
}
