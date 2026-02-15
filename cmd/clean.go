package cmd

import (
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Free up disk space",
	Long:  "Deep cleanup of caches, logs, temp files, and browser leftovers to reclaim disk space.",
	Run: func(cmd *cobra.Command, args []string) {
		// Implemented in T4
	},
}

func init() {
	cleanCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview the cleanup plan without deleting")
	cleanCmd.Flags().Bool("whitelist", false, "Manage protected caches")
	cleanCmd.Flags().Bool("all", false, "Clean all categories")
	cleanCmd.Flags().Bool("user", false, "Clean user caches only")
	cleanCmd.Flags().Bool("system", false, "Clean system caches only (requires admin)")
	cleanCmd.Flags().Bool("browser", false, "Clean browser caches only")
	cleanCmd.Flags().Bool("dev", false, "Clean developer tool caches only")
}
