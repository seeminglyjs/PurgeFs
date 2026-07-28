package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/seeminglyjs/PurgeFs/internal/engine"
	"github.com/seeminglyjs/PurgeFs/internal/history"
	"github.com/seeminglyjs/PurgeFs/internal/trash"
	"github.com/seeminglyjs/PurgeFs/internal/tui"
	"github.com/spf13/cobra"
)

var (
	purgeYes         bool
	purgeHard        bool
	purgeInteractive bool
	purgePreset      string
)

var purgeCmd = &cobra.Command{
	Use:   "purge [path]",
	Short: "Purge junk under a path (asks before deleting)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) == 1 {
			path = args[0]
		}
		if err := guardRoot(path); err != nil {
			return err
		}

		var tr trash.Trasher
		if purgeHard {
			tr = trash.NewHardDeleter()
		} else {
			t, err := trash.NewTrasher()
			if err != nil {
				return err
			}
			tr = t
		}
		rules := engine.DefaultRules()
		if purgePreset != "" {
			r, ok := engine.Preset(purgePreset)
			if !ok {
				return fmt.Errorf("unknown preset %q (available: dev-caches)", purgePreset)
			}
			rules = r
		}
		if purgeInteractive {
			return runPurgeInteractive(cmd.OutOrStdout(), path, tr, rules)
		}
		return runPurge(cmd.OutOrStdout(), cmd.InOrStdin(), path, tr, purgeHard, purgeYes, rules)
	},
}

// runPurge 는 path 를 스캔·분류해 삭제 대상을 요약하고, assumeYes 가 아니면 in 에서 확인을
// 받은 뒤 tr 로 처리한다. hard 는 확인 문구를 완전 삭제용으로 바꾸는 데만 쓴다. junk 가
// 없으면 아무것도 삭제하지 않는다.
func runPurge(w io.Writer, in io.Reader, path string, tr trash.Trasher, hard, assumeYes bool, rules []engine.Rule) error {
	// 상대 경로면 절대 경로로 바꾼다. 그래야 매니페스트에 기록되는 원본 경로가 절대 경로가
	// 되어 undo 를 다른 디렉토리에서 실행해도 올바른 위치로 복원된다.
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	report, werrs, err := engine.Scan(path)
	if err != nil {
		return err
	}
	groups := engine.Classify(report, rules)
	if len(groups) == 0 {
		fmt.Fprintln(w, "정리할 junk가 없습니다.")
		reportWalkErrors(w, werrs)
		return nil
	}

	var paths []string
	var total int64
	for _, g := range groups {
		paths = append(paths, g.Paths...)
		total += g.Size
	}

	action := "휴지통으로 이동"
	if hard {
		action = "완전 삭제(복구 불가)"
	}
	fmt.Fprintf(w, "%s 대상: %s, 항목 %d개\n", action, humanBytes(total), len(paths))
	for _, g := range groups {
		fmt.Fprintf(w, "  %10s  %-14s (%d %s)\n",
			humanBytes(g.Size), g.Category, g.Count, plural(g.Count, "item", "items"))
	}
	reportWalkErrors(w, werrs)

	if !assumeYes {
		fmt.Fprint(w, "진행할까요? [y/N] ")
		if !confirmed(in) {
			fmt.Fprintln(w, "취소했습니다.")
			return nil
		}
	}

	return reportTrashResult(w, purgePaths(w, tr, paths))
}

// reportWalkErrors 는 순회 중 못 읽은 경로가 있었음을 알린다. 조용히 넘기면 사용자는 권한
// 때문에 빠진 부분까지 다 정리된 줄 안다.
func reportWalkErrors(w io.Writer, werrs []engine.WalkError) {
	if len(werrs) == 0 {
		return
	}
	fmt.Fprintf(w, "읽지 못해 건너뛴 경로 %d개 — 실제 정리 대상은 더 많을 수 있습니다\n", len(werrs))
}

// reportTrashResult 는 처리·실패 개수를 출력한다. 실패가 있으면 첫 이유를 보여주고, 하나도
// 처리 못 했으면 스크립트가 감지하도록 에러를 반환한다.
func reportTrashResult(w io.Writer, res trash.Result) error {
	fmt.Fprintf(w, "처리 %d개", len(res.Trashed))
	if len(res.Failed) > 0 {
		fmt.Fprintf(w, ", 실패 %d개", len(res.Failed))
	}
	fmt.Fprintln(w)
	if len(res.Failed) > 0 {
		fmt.Fprintf(w, "  첫 실패: %s: %v\n", res.Failed[0].Path, res.Failed[0].Err)
		if len(res.Trashed) == 0 {
			return fmt.Errorf("purge failed: %d개 항목 모두 실패", len(res.Failed))
		}
	}
	return nil
}

// groupsToItems 는 분류 결과를 TUI 항목으로 바꾼다.
func groupsToItems(groups []engine.CategoryGroup) []tui.Item {
	items := make([]tui.Item, 0, len(groups))
	for _, g := range groups {
		items = append(items, tui.Item{Label: g.Category, Size: g.Size, Paths: g.Paths})
	}
	return items
}

// runPurgeInteractive 는 분류 결과를 TUI 로 띄워 사용자가 고른 항목만 tr 로 처리한다. tty 가
// 필요한 tea.Program 실행이라 단위 테스트하지 않는다(선택 로직은 internal/tui 에서 테스트).
func runPurgeInteractive(w io.Writer, path string, tr trash.Trasher, rules []engine.Rule) error {
	// 상대 경로면 절대 경로로 바꾼다. 그래야 매니페스트에 기록되는 원본 경로가 절대 경로가
	// 되어 undo 를 다른 디렉토리에서 실행해도 올바른 위치로 복원된다.
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	report, werrs, err := engine.Scan(path)
	if err != nil {
		return err
	}
	groups := engine.Classify(report, rules)
	if len(groups) == 0 {
		fmt.Fprintln(w, "정리할 junk가 없습니다.")
		reportWalkErrors(w, werrs)
		return nil
	}
	reportWalkErrors(w, werrs)

	final, err := tea.NewProgram(tui.New(groupsToItems(groups), humanBytes)).Run()
	if err != nil {
		return err
	}
	m, ok := final.(tui.Model)
	if !ok || !m.Confirmed() {
		fmt.Fprintln(w, "취소했습니다.")
		return nil
	}
	paths := m.SelectedPaths()
	if len(paths) == 0 {
		fmt.Fprintln(w, "선택한 항목이 없습니다.")
		return nil
	}
	return reportTrashResult(w, purgePaths(w, tr, paths))
}

// confirmed 는 in 에서 한 줄 읽어 y/yes(대소문자 무시)면 true.
func confirmed(in io.Reader) bool {
	s := bufio.NewScanner(in)
	if !s.Scan() {
		return false
	}
	ans := strings.ToLower(strings.TrimSpace(s.Text()))
	return ans == "y" || ans == "yes"
}

// systemRoots 는 정리 대상이 되어선 안 되는 시스템 디렉토리다. macOS 에서 /var, /etc 는
// /private 아래로 가는 심볼릭링크라 비교 전에 함께 resolve 한다.
var systemRoots = []string{
	"/System", "/Library", "/Applications",
	"/usr", "/bin", "/sbin", "/etc", "/var", "/opt", "/boot",
}

// guardRoot 는 위험한 루트를 거부한다: 파일시스템 루트(/), 홈 디렉토리와 그 조상, 시스템
// 디렉토리. 사용자가 실수로 거대한 영역을 통째로 정리하는 것을 막는다. engine.Scan 이 root 를
// EvalSymlinks 로 resolve 하므로, 가드도 같은 resolve 를 거쳐 실제로 순회될 경로를 검사한다
// (심볼릭링크로 가드를 우회하는 것을 막는다).
func guardRoot(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	abs = resolveClean(abs)
	if abs == string(filepath.Separator) {
		return fmt.Errorf("refusing to purge filesystem root %q", abs)
	}
	// 홈 자신뿐 아니라 홈의 조상도 거부한다. /Users 처럼 한 단계 위를 지정하면 홈 전체가
	// 그대로 정리 대상에 들어오기 때문이다.
	if home, err := os.UserHomeDir(); err == nil {
		h := resolveClean(home)
		if h == abs {
			return fmt.Errorf("refusing to purge home directory %q", abs)
		}
		if strings.HasPrefix(h, abs+string(filepath.Separator)) {
			return fmt.Errorf("refusing to purge %q: it contains the home directory %q", abs, h)
		}
	}
	for _, s := range systemRoots {
		if resolveClean(s) == abs {
			return fmt.Errorf("refusing to purge system directory %q", abs)
		}
	}
	return nil
}

// resolveClean 은 심볼릭링크를 풀고 경로를 정규화한다. resolve 가 실패하면(없는 경로 등)
// 원본을 정규화만 해서 돌려준다.
func resolveClean(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	return filepath.Clean(p)
}

// historyDir 은 매니페스트를 저장하는 ~/.purgefs/history 경로를 반환한다.
func historyDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".purgefs", "history"), nil
}

// movedToItems 는 휴지통 이동 매핑을 매니페스트 항목으로 바꾼다.
func movedToItems(moved []trash.Moved) []history.Item {
	items := make([]history.Item, 0, len(moved))
	for _, mv := range moved {
		items = append(items, history.Item{Original: mv.Original, Dest: mv.Dest, Sidecar: mv.Sidecar})
	}
	return items
}

// recordHistory 는 휴지통 이동이 있으면 매니페스트를 저장한다. 완전 삭제(Moved 비어 있음)는
// 되돌릴 수 없으므로 아무것도 남기지 않는다.
func recordHistory(dir string, res trash.Result, createdAt int64) error {
	if len(res.Moved) == 0 {
		return nil
	}
	_, err := history.Save(dir, history.Manifest{CreatedAt: createdAt, Items: movedToItems(res.Moved)})
	return err
}

// trashAndRecord 는 경로를 하나씩 처리하고, 이동이 생길 때마다 지금까지의 매핑으로 매니페스트를
// 다시 쓴다. 전부 끝난 뒤에 한 번만 기록하면 도중에 Ctrl-C·kill·패닉이 났을 때 이미 휴지통으로
// 옮겨진 파일의 매핑이 하나도 남지 않아 undo 가 불가능해진다. 매니페스트 파일명은 createdAt
// 으로 고정이라 매번 같은 파일을 덮어쓴다.
//
// 기록에 실패해도 이미 옮긴 것은 되돌리지 않고 계속 진행하며, 첫 에러를 반환해 호출자가 알린다.
func trashAndRecord(tr trash.Trasher, paths []string, dir string, createdAt int64) (trash.Result, error) {
	var acc trash.Result
	var recErr error
	for _, p := range paths {
		r := tr.Trash([]string{p})
		acc.Trashed = append(acc.Trashed, r.Trashed...)
		acc.Moved = append(acc.Moved, r.Moved...)
		acc.Failed = append(acc.Failed, r.Failed...)
		if len(r.Moved) == 0 {
			continue // 완전 삭제이거나 실패 — 기록할 매핑이 없다
		}
		if err := recordHistory(dir, acc, createdAt); err != nil && recErr == nil {
			recErr = err
		}
	}
	return acc, recErr
}

// purgePaths 는 경로들을 정리하고 undo 매니페스트를 남긴다. 홈을 못 찾아 기록 디렉토리를
// 정할 수 없으면 기록 없이 정리만 한다(현재 동작 유지).
func purgePaths(w io.Writer, tr trash.Trasher, paths []string) trash.Result {
	dir, err := historyDir()
	if err != nil {
		return tr.Trash(paths)
	}
	res, herr := trashAndRecord(tr, paths, dir, time.Now().UnixNano())
	if herr != nil {
		fmt.Fprintf(w, "  (기록 실패: %v)\n", herr)
	}
	return res
}

func init() {
	purgeCmd.Flags().BoolVar(&purgeYes, "yes", false, "확인 없이 진행")
	purgeCmd.Flags().BoolVar(&purgeHard, "hard", false, "휴지통이 아니라 완전 삭제")
	purgeCmd.Flags().BoolVarP(&purgeInteractive, "interactive", "i", false, "대화형 TUI로 선택해 정리")
	purgeCmd.Flags().StringVar(&purgePreset, "preset", "", "규칙 프리셋(예: dev-caches)")
	rootCmd.AddCommand(purgeCmd)
}
