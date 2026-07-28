package cmd

import (
	"fmt"

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
		fmt.Printf("[scan] %s — not implemented yet\n", path)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
