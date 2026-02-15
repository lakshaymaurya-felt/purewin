package cmd

import (
	"github.com/spf13/cobra"
)

var purgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Clean project build artifacts",
	Long:  "Find and remove build artifacts (node_modules, target, build, dist, etc.) from project directories.",
	Run: func(cmd *cobra.Command, args []string) {
		// Implemented in T9
	},
}

func init() {
	purgeCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without deleting")
	purgeCmd.Flags().Bool("paths", false, "Configure project scan directories")
	purgeCmd.Flags().Int("min-age", 7, "Minimum age in days (recent projects are skipped)")
	purgeCmd.Flags().String("min-size", "", "Minimum artifact size to show (e.g., 50MB)")
}
