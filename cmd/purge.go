package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var purgeYes bool

var purgeCmd = &cobra.Command{
	Use:   "purge [path]",
	Short: "Purge junk under a path (asks before deleting)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) == 1 {
			path = args[0]
		}
		fmt.Printf("[purge] %s (yes=%t) — not implemented yet\n", path, purgeYes)
		return nil
	},
}

func init() {
	purgeCmd.Flags().BoolVar(&purgeYes, "yes", false, "skip confirmation prompt")
	rootCmd.AddCommand(purgeCmd)
}
