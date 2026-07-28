package cmd

import (
	"fmt"
	"io"

	"github.com/seeminglyjs/PurgeFs/internal/history"
	"github.com/spf13/cobra"
)

var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Restore the most recent trash purge",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := historyDir()
		if err != nil {
			return err
		}
		return runUndoDir(cmd.OutOrStdout(), dir)
	},
}

// runUndoDir 은 dir 안의 최신 매니페스트를 읽어 파일을 되돌린다. 완전 삭제(--hard)는 기록이
// 없어 되돌릴 수 없다.
func runUndoDir(w io.Writer, dir string) error {
	m, ok, err := history.LoadLatest(dir)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(w, "복원할 기록이 없습니다.")
		return nil
	}
	res := history.Restore(m)
	// 되돌린 매니페스트는 소비해 다음 undo 가 그 이전 purge 로 넘어가게 한다. 단 실패한 항목이
	// 있으면 남겨둔다 — 원인(권한, 사라진 부모 디렉토리 등)을 고친 뒤 다시 시도할 수 있어야 한다.
	// 건너뛴 항목은 다시 해도 결과가 같으므로 소비를 막지 않는다.
	if len(res.Failed) == 0 {
		if err := history.Consume(dir, m); err != nil {
			fmt.Fprintf(w, "  (기록 정리 실패: %v)\n", err)
		}
	}
	fmt.Fprintf(w, "복원 %d개", len(res.Restored))
	if len(res.Skipped) > 0 {
		fmt.Fprintf(w, ", 건너뜀 %d개", len(res.Skipped))
	}
	if len(res.Failed) > 0 {
		fmt.Fprintf(w, ", 실패 %d개", len(res.Failed))
	}
	fmt.Fprintln(w)
	return nil
}

func init() {
	rootCmd.AddCommand(undoCmd)
}
