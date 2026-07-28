package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/seeminglyjs/PurgeFs/internal/engine"
	"github.com/seeminglyjs/PurgeFs/internal/trash"
	"github.com/spf13/cobra"
)

var purgeCmd = &cobra.Command{
	Use:   "purge [path]",
	Short: "Purge junk under a path (asks before deleting)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("purge wiring lands in P3 Task 3")
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
	fmt.Fprintf(w, "처리 %d개", len(res.Trashed))
	if len(res.Failed) > 0 {
		fmt.Fprintf(w, ", 실패 %d개", len(res.Failed))
	}
	fmt.Fprintln(w)
	return nil
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

func init() {
	rootCmd.AddCommand(purgeCmd)
}
