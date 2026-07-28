package cmd

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"

	"github.com/seeminglyjs/PurgeFs/internal/engine"
	"github.com/spf13/cobra"
)

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

// runScan scans path and writes a size summary to w.
//
// The root is resolved through filepath.EvalSymlinks before scanning: engine.Walk
// deliberately does not follow symlinks (including a symlinked root), and on macOS
// common roots are themselves symlinks (e.g. /tmp -> /private/tmp), so scanning the
// raw path would report an empty tree. If resolution fails (e.g. path does not
// exist), the original path is scanned so engine.Scan can surface the real error.
func runScan(w io.Writer, path string) error {
	scanPath := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		scanPath = resolved
	}

	report, werrs, err := engine.Scan(scanPath)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Scanned %s\n", scanPath)
	fmt.Fprintf(w, "Total: %s across %d files, %d dirs\n",
		humanBytes(report.TotalSize), report.FileCount, report.DirCount)

	// Top-level children, largest first.
	children := append([]*engine.Entry(nil), report.Root.Children...)
	sort.Slice(children, func(i, j int) bool {
		return children[i].Size > children[j].Size
	})
	for _, c := range children {
		fmt.Fprintf(w, "  %10s  %s\n", humanBytes(c.Size), c.Path)
	}

	if len(werrs) > 0 {
		fmt.Fprintf(w, "\nSkipped %d path(s) due to errors\n", len(werrs))
	}
	return nil
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
