package cmd

import (
	"fmt"
	"io"
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
func runScan(w io.Writer, path string) error {
	report, werrs, err := engine.Scan(path)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Scanned %s\n", report.Root.Path)
	fmt.Fprintf(w, "Total: %s across %d %s, %d %s\n",
		humanBytes(report.TotalSize),
		report.FileCount, plural(report.FileCount, "file", "files"),
		report.DirCount, plural(report.DirCount, "dir", "dirs"))

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
