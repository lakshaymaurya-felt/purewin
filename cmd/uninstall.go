package cmd

import (
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove apps completely",
	Long:  "Thoroughly remove applications along with their registry entries, data, and hidden remnants.",
	Run: func(cmd *cobra.Command, args []string) {
		// Implemented in T5
	},
}

func init() {
	uninstallCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without uninstalling")
	uninstallCmd.Flags().Bool("quiet", false, "Suppress confirmation prompts")
	uninstallCmd.Flags().Bool("show-all", false, "Show system components too")
	uninstallCmd.Flags().String("search", "", "Search for apps by name")
}
