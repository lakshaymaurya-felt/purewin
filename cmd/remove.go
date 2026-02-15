package cmd

import (
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove WinMole from system",
	Long:  "Uninstall WinMole, remove configuration files and cached data.",
	Run: func(cmd *cobra.Command, args []string) {
		// Implemented in T11
	},
}
