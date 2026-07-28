package cmd

import (
	"fmt"
	"io"
	"sort"

	"github.com/seeminglyjs/PurgeFs/internal/engine"
	"github.com/spf13/cobra"
)

// scanCmd 은 읽기 전용 `scan` 서브커맨드다: 용량만 보고하고 절대 삭제하지 않는다.
// path 인자는 선택이며 기본값은 현재 디렉토리다.
var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan a path and report junk files/directories (no deletion)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) == 1 {
			path = args[0]
		}
		return runScan(cmd.OutOrStdout(), path)
	},
}

// runScan 은 path 를 스캔하고 용량 요약을 w 에 쓴다. (stdout 을 직접 쓰지 않고) io.Writer 를
// 받아 테스트가 출력을 캡처할 수 있게 한다. 순회는 엔진이 하고, 이 함수는 순수하게 출력만
// 담당한다.
func runScan(w io.Writer, path string) error {
	report, werrs, err := engine.Scan(path)
	if err != nil {
		return err
	}

	// 헤더는 report.Root.Path — 엔진이 심볼릭링크 root 를 resolve 한 뒤 실제로 순회한
	// 경로 — 를 써서, 아래 자식 경로들과 접두어가 맞아떨어지게 한다.
	fmt.Fprintf(w, "Scanned %s\n", report.Root.Path)
	fmt.Fprintf(w, "Total: %s across %d %s, %d %s\n",
		humanBytes(report.TotalSize),
		report.FileCount, plural(report.FileCount, "file", "files"),
		report.DirCount, plural(report.DirCount, "dir", "dirs"))

	// 정렬 전에 슬라이스를 복사해, 출력 순서가 엔진의 Report 를 절대 건드리지 않게 한다
	// (이후 프론트엔드가 Children 을 원래 순서로 읽을 수 있다).
	children := append([]*engine.Entry(nil), report.Root.Children...)
	sort.Slice(children, func(i, j int) bool {
		return children[i].Size > children[j].Size // 큰 것부터
	})
	for _, c := range children {
		fmt.Fprintf(w, "  %10s  %s\n", humanBytes(c.Size), c.Path)
	}

	// 못 읽은 경로는 치명적이 아니라 건너뛴 것 — 총량이 실제보다 적을 수 있음을 알린다.
	if len(werrs) > 0 {
		fmt.Fprintf(w, "\nSkipped %d path(s) due to errors\n", len(werrs))
	}
	return nil
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
