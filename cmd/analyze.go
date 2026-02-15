package cmd

import (
	"github.com/spf13/cobra"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze [path]",
	Short: "Explore disk usage",
	Long:  "Interactive disk space analyzer with visual tree view.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Implemented in T7
	},
}

func init() {
	analyzeCmd.Flags().Int("depth", 0, "Maximum directory depth to display")
	analyzeCmd.Flags().String("min-size", "", "Minimum size to display (e.g., 100MB)")
	analyzeCmd.Flags().StringSlice("exclude", nil, "Directories to exclude from scan")
}
