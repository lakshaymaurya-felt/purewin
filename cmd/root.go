package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Global flags
	debug  bool
	dryRun bool

	// Version info populated from main
	appVersion = "dev"
	appCommit  = "none"
	appDate    = "unknown"
)

// SetVersionInfo sets build-time version information.
func SetVersionInfo(version, commit, date string) {
	appVersion = version
	appCommit = commit
	appDate = date
}

var rootCmd = &cobra.Command{
	Use:   "wm",
	Short: "Deep clean and optimize your Windows",
	Long: `WinMole - Deep clean and optimize your Windows.

A Windows port of Mole (https://github.com/tw93/Mole).
All-in-one toolkit for system cleanup, app uninstallation,
disk analysis, system optimization, and live monitoring.`,
	Run: func(cmd *cobra.Command, args []string) {
		// When invoked without subcommand, show interactive menu
		runInteractiveMenu()
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Show detailed operation logs")

	// Register all subcommands
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(optimizeCmd)
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(purgeCmd)
	rootCmd.AddCommand(installerCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(versionCmd)
}

// runInteractiveMenu launches the full-screen interactive main menu.
func runInteractiveMenu() {
	// Placeholder — will be implemented in T13 (interactive menu)
	fmt.Println("WinMole - Deep clean and optimize your Windows")
	fmt.Println("Run 'wm --help' for available commands.")
	fmt.Println()
	fmt.Printf("Version %s (%s) built %s\n", appVersion, appCommit, appDate)
	os.Exit(0)
}
