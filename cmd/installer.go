package cmd

import (
	"github.com/spf13/cobra"
)

var installerCmd = &cobra.Command{
	Use:   "installer",
	Short: "Find and remove installer files",
	Long:  "Scan Downloads, Desktop, and package manager caches for installer files (.exe, .msi, .msix).",
	Run: func(cmd *cobra.Command, args []string) {
		// Implemented in T10
	},
}

func init() {
	installerCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without deleting")
	installerCmd.Flags().Int("min-age", 0, "Minimum file age in days")
	installerCmd.Flags().String("min-size", "", "Minimum file size (e.g., 10MB)")
}
