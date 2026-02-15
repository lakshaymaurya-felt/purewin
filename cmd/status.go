package cmd

import (
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Monitor system health",
	Long:  "Real-time dashboard with CPU, memory, disk, network, GPU, and battery metrics.",
	Run: func(cmd *cobra.Command, args []string) {
		// Implemented in T8
	},
}

func init() {
	statusCmd.Flags().Int("refresh", 1, "Refresh interval in seconds")
	statusCmd.Flags().Bool("json", false, "Output metrics as JSON")
}
