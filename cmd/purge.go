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
			t, err := trash.NewMacTrasher()
			if err != nil {
				return err
			}
			tr = t
		}
		if purgeInteractive {
			return runPurgeInteractive(cmd.OutOrStdout(), path, tr)
		}
		return runPurge(cmd.OutOrStdout(), cmd.InOrStdin(), path, tr, purgeHard, purgeYes)
	},
}

// runPurge 는 path 를 스캔·분류해 삭제 대상을 요약하고, assumeYes 가 아니면 in 에서 확인을
// 받은 뒤 tr 로 처리한다. hard 는 확인 문구를 완전 삭제용으로 바꾸는 데만 쓴다. junk 가
// 없으면 아무것도 삭제하지 않는다.
func runPurge(w io.Writer, in io.Reader, path string, tr trash.Trasher, hard, assumeYes bool) error {
	report, _, err := engine.Scan(path)
	if err != nil {
		return err
	}
	groups := engine.Classify(report, engine.DefaultRules())
	if len(groups) == 0 {
		fmt.Fprintln(w, "정리할 junk가 없습니다.")
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

	if !assumeYes {
		fmt.Fprint(w, "진행할까요? [y/N] ")
		if !confirmed(in) {
			fmt.Fprintln(w, "취소했습니다.")
			return nil
		}
	}

	res := tr.Trash(paths)
	if dir, err := historyDir(); err == nil {
		if herr := recordHistory(dir, res, time.Now().Unix()); herr != nil {
			fmt.Fprintf(w, "  (기록 실패: %v)\n", herr)
		}
	}
	return reportTrashResult(w, res)
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
func runPurgeInteractive(w io.Writer, path string, tr trash.Trasher) error {
	report, _, err := engine.Scan(path)
	if err != nil {
		return err
	}
	groups := engine.Classify(report, engine.DefaultRules())
	if len(groups) == 0 {
		fmt.Fprintln(w, "정리할 junk가 없습니다.")
		return nil
	}

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
	res := tr.Trash(paths)
	if dir, err := historyDir(); err == nil {
		if herr := recordHistory(dir, res, time.Now().Unix()); herr != nil {
			fmt.Fprintf(w, "  (기록 실패: %v)\n", herr)
		}
	}
	return reportTrashResult(w, res)
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

// guardRoot 는 위험한 루트를 거부한다: 파일시스템 루트(/)와 홈 디렉토리. 사용자가 실수로
// 거대한 영역을 통째로 정리하는 것을 막는다. engine.Scan 이 root 를 EvalSymlinks 로
// resolve 하므로, 가드도 같은 resolve 를 거쳐 실제로 순회될 경로를 검사한다(심볼릭링크로
// 가드를 우회하는 것을 막는다).
func guardRoot(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	abs = filepath.Clean(abs)
	if abs == string(filepath.Separator) {
		return fmt.Errorf("refusing to purge filesystem root %q", abs)
	}
	if home, err := os.UserHomeDir(); err == nil {
		if resolved, err := filepath.EvalSymlinks(home); err == nil {
			home = resolved
		}
		if abs == filepath.Clean(home) {
			return fmt.Errorf("refusing to purge home directory %q", abs)
		}
	}
	return nil
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
		items = append(items, history.Item{Original: mv.Original, Dest: mv.Dest})
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

func init() {
	purgeCmd.Flags().BoolVar(&purgeYes, "yes", false, "확인 없이 진행")
	purgeCmd.Flags().BoolVar(&purgeHard, "hard", false, "휴지통이 아니라 완전 삭제")
	purgeCmd.Flags().BoolVarP(&purgeInteractive, "interactive", "i", false, "대화형 TUI로 선택해 정리")
	rootCmd.AddCommand(purgeCmd)
}
