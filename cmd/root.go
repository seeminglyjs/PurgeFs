// Package cmd 는 purgefs 명령줄 인터페이스를 구성한다.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version 은 현재 빌드 버전. 빌드 시 -ldflags 로 덮어쓸 수 있다.
var version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:     "purgefs",
	Short:   "Fast terminal disk cleaner for macOS/Linux",
	Long:    "PurgeFs scans a path for junk files and directories and purges them.\nNo GUI, no paywall.",
	Version: version,
}

// Execute 는 root 커맨드를 실행하고, 에러 시 0 이 아닌 코드로 종료한다.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
