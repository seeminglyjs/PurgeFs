// Package cmd wires up the purgefs command-line interface.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is the current build version, overridable at build time via -ldflags.
var version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:     "purgefs",
	Short:   "Fast terminal disk cleaner for macOS/Linux",
	Long:    "PurgeFs scans a path for junk files and directories and purges them.\nNo GUI, no paywall.",
	Version: version,
}

// Execute runs the root command and exits non-zero on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
