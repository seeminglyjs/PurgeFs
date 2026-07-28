package tui

import (
	"fmt"
	"strings"
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

func TestInitHasNoCommand(t *testing.T) {
	if cmd := New(sampleItems(), idFmt).Init(); cmd != nil {
		t.Errorf("Init = %v, want nil", cmd)
	}
}

// 커서는 목록 밖으로 나가지 않는다.
func TestCursorStopsAtBounds(t *testing.T) {
	m := New(sampleItems(), idFmt)
	up, _ := m.Update(key(tea.KeyUp)) // 이미 맨 위
	if up.(Model).cursor != 0 {
		t.Errorf("cursor = %d after up at the top, want 0", up.(Model).cursor)
	}
	down, _ := m.Update(key(tea.KeyDown))
	down, _ = down.(Model).Update(key(tea.KeyDown)) // 이미 맨 아래
	if got := down.(Model).cursor; got != 1 {
		t.Errorf("cursor = %d after down at the bottom, want 1", got)
	}
}

// 키가 아닌 메시지는 상태를 바꾸지 않는다.
func TestNonKeyMessageIsIgnored(t *testing.T) {
	m := New(sampleItems(), idFmt)
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 10, Height: 10})
	if cmd != nil {
		t.Errorf("cmd = %v, want nil", cmd)
	}
	if updated.(Model).SelectedSize() != m.SelectedSize() {
		t.Error("a non-key message must not change the selection")
	}
}

func TestViewShowsItemsChecksAndCursor(t *testing.T) {
	m := New(sampleItems(), func(n int64) string { return fmt.Sprintf("%dB", n) })
	out := m.View()

	for _, want := range []string{"node_modules", "os-junk", "1000B", "6B"} {
		if !strings.Contains(out, want) {
			t.Errorf("View missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "> [x] node_modules") {
		t.Errorf("View must mark the cursor row and the checked box:\n%s", out)
	}
	// 해제하면 체크가 빠진다.
	updated, _ := m.Update(key(tea.KeySpace))
	if out := updated.(Model).View(); !strings.Contains(out, "> [ ] node_modules") {
		t.Errorf("View must show an empty box after deselecting:\n%s", out)
	}
}

// 확정·취소 후에는 화면을 비워 터미널에 잔상이 남지 않게 한다.
func TestViewEmptyAfterQuit(t *testing.T) {
	for _, k := range []tea.KeyMsg{key(tea.KeyEnter), rune_('q')} {
		updated, _ := New(sampleItems(), idFmt).Update(k)
		if out := updated.(Model).View(); out != "" {
			t.Errorf("View after %v = %q, want empty", k, out)
		}
	}
}

func TestSizeBar(t *testing.T) {
	cases := []struct {
		size, max int64
		want      string
	}{
		{10, 10, "██████████"},
		{5, 10, "█████░░░░░"},
		{0, 10, "░░░░░░░░░░"},
		{20, 10, "██████████"}, // width 를 넘지 않는다
	}
	for _, c := range cases {
		if got := sizeBar(c.size, c.max, 10); got != c.want {
			t.Errorf("sizeBar(%d, %d, 10) = %q, want %q", c.size, c.max, got, c.want)
		}
	}
	if got := sizeBar(5, 0, 10); got != "" {
		t.Errorf("sizeBar with max 0 = %q, want empty", got)
	}
}
