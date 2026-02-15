package cmd

import (
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update WinMole",
	Long:  "Check for and install the latest version of WinMole from GitHub releases.",
	Run: func(cmd *cobra.Command, args []string) {
		// Implemented in T11
	},
}

func init() {
	updateCmd.Flags().Bool("force", false, "Force reinstall latest version")
}
