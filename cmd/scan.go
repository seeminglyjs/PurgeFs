package cmd

import (
	"fmt"
	"io"
	"sort"

	"github.com/seeminglyjs/PurgeFs/internal/engine"
	"github.com/spf13/cobra"
)

// scanCmd is the read-only `scan` subcommand: it reports sizes and never
// deletes. The path argument is optional and defaults to the current directory.
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

// runScan scans path and writes a size summary to w. It takes an io.Writer
// (not stdout directly) so tests can capture the output. The engine does the
// walking; this function is purely presentation.
func runScan(w io.Writer, path string) error {
	report, werrs, err := engine.Scan(path)
	if err != nil {
		return err
	}

	// Header uses report.Root.Path — the path actually walked after the engine
	// resolved any symlinked root — so it lines up with the child paths below.
	fmt.Fprintf(w, "Scanned %s\n", report.Root.Path)
	fmt.Fprintf(w, "Total: %s across %d %s, %d %s\n",
		humanBytes(report.TotalSize),
		report.FileCount, plural(report.FileCount, "file", "files"),
		report.DirCount, plural(report.DirCount, "dir", "dirs"))

	// Copy the slice before sorting so display order never mutates the engine's
	// Report (a later frontend may read Children in its original order).
	children := append([]*engine.Entry(nil), report.Root.Children...)
	sort.Slice(children, func(i, j int) bool {
		return children[i].Size > children[j].Size // largest first
	})
	for _, c := range children {
		fmt.Fprintf(w, "  %10s  %s\n", humanBytes(c.Size), c.Path)
	}

	// Unreadable paths were skipped, not fatal — tell the user the total may be
	// an undercount.
	if len(werrs) > 0 {
		fmt.Fprintf(w, "\nSkipped %d path(s) due to errors\n", len(werrs))
	}
	return nil
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
