package cmd

import (
	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion",
	Short: "Set up shell tab completion",
	Long:  "Generate tab completion scripts for PowerShell.",
	Run: func(cmd *cobra.Command, args []string) {
		// Implemented in T12
	},
}
