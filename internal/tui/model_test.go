package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func idFmt(n int64) string { return "" } // 테스트는 크기 문자열을 검사하지 않는다

func sampleItems() []Item {
	return []Item{
		{Label: "node_modules", Size: 1000, Paths: []string{"/p/node_modules"}},
		{Label: "os-junk", Size: 6, Paths: []string{"/p/.DS_Store"}},
	}
}

func key(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }
func rune_(r rune) tea.KeyMsg      { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func TestNewSelectsAll(t *testing.T) {
	m := New(sampleItems(), idFmt)
	if got := len(m.SelectedPaths()); got != 2 {
		t.Errorf("SelectedPaths = %d, want 2 (all selected by default)", got)
	}
	if m.SelectedSize() != 1006 {
		t.Errorf("SelectedSize = %d, want 1006", m.SelectedSize())
	}
}

func TestSpaceTogglesSelection(t *testing.T) {
	m := New(sampleItems(), idFmt)
	// 커서는 0(node_modules)에서 시작. 스페이스로 해제.
	updated, _ := m.Update(key(tea.KeySpace))
	m = updated.(Model)
	if m.SelectedSize() != 6 {
		t.Errorf("after deselecting node_modules, SelectedSize = %d, want 6", m.SelectedSize())
	}
	paths := m.SelectedPaths()
	if len(paths) != 1 || paths[0] != "/p/.DS_Store" {
		t.Errorf("SelectedPaths = %v, want [/p/.DS_Store]", paths)
	}
}

func TestCursorDownThenToggle(t *testing.T) {
	m := New(sampleItems(), idFmt)
	updated, _ := m.Update(key(tea.KeyDown)) // 커서 1(os-junk)로
	m = updated.(Model)
	updated, _ = m.Update(key(tea.KeySpace)) // os-junk 해제
	m = updated.(Model)
	if m.SelectedSize() != 1000 {
		t.Errorf("SelectedSize = %d, want 1000", m.SelectedSize())
	}
}

func TestEnterConfirmsAndQuits(t *testing.T) {
	m := New(sampleItems(), idFmt)
	updated, cmd := m.Update(key(tea.KeyEnter))
	m = updated.(Model)
	if !m.Confirmed() {
		t.Error("enter must set Confirmed")
	}
	if cmd == nil {
		t.Fatal("enter must return a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("enter's command must be tea.Quit")
	}
}

func TestQuitCancels(t *testing.T) {
	m := New(sampleItems(), idFmt)
	updated, cmd := m.Update(rune_('q'))
	m = updated.(Model)
	if m.Confirmed() {
		t.Error("q must not confirm")
	}
	if cmd == nil {
		t.Fatal("q must return a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q's command must be tea.Quit")
	}
}

func TestEscCancels(t *testing.T) {
	m := New(sampleItems(), idFmt)
	updated, cmd := m.Update(key(tea.KeyEsc))
	m = updated.(Model)
	if m.Confirmed() {
		t.Error("esc must not confirm")
	}
	if cmd == nil {
		t.Fatal("esc must return a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("esc's command must be tea.Quit")
	}
}

// 전부 해제하면 선택 경로·합계가 비어야 한다(빈 선택 안전 경로).
func TestDeselectAllYieldsEmpty(t *testing.T) {
	m := New(sampleItems(), idFmt)
	updated, _ := m.Update(key(tea.KeySpace)) // 커서 0 해제
	m = updated.(Model)
	updated, _ = m.Update(key(tea.KeyDown)) // 커서 1로
	m = updated.(Model)
	updated, _ = m.Update(key(tea.KeySpace)) // 커서 1 해제
	m = updated.(Model)
	if len(m.SelectedPaths()) != 0 {
		t.Errorf("SelectedPaths = %v, want empty", m.SelectedPaths())
	}
	if m.SelectedSize() != 0 {
		t.Errorf("SelectedSize = %d, want 0", m.SelectedSize())
	}
}
