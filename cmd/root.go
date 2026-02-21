package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/lakshaymaurya-felt/purewin/internal/core"
	"github.com/lakshaymaurya-felt/purewin/internal/shell"
	"github.com/lakshaymaurya-felt/purewin/internal/ui"
)

var (
	// Global flags
	debug    bool
	dryRun   bool
	runAdmin bool
	noColor  bool

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
	Use:   "pw",
	Short: "Deep clean and optimize your Windows",
	Long: `PureWin - Deep clean and optimize your Windows.

All-in-one toolkit for system cleanup, app uninstallation,
disk analysis, system optimization, and live monitoring.`,
}

// Execute runs the root command.
func Execute() error {
	// Enable Windows Virtual Terminal Processing globally so ANSI escape
	// codes render as colours in cmd.exe / PowerShell for ALL code paths
	// (inline spinners, confirm dialogs, styled fmt.Print output, etc.).
	vtSuccess := ui.EnableVTProcessing()

	// Warn if VT processing failed and we're in an interactive terminal
	if !vtSuccess && ui.IsTerminal() {
		fmt.Fprintln(os.Stderr, "WARNING: Your terminal does not support ANSI colors.")
		fmt.Fprintln(os.Stderr, "For the best experience, use Windows Terminal or PowerShell 7+.")
		fmt.Fprintln(os.Stderr, "PureWin will continue with limited styling.")
		fmt.Fprintln(os.Stderr, "")

		// Set NO_COLOR so lipgloss/termenv don't output garbled escape codes
		os.Setenv("NO_COLOR", "1")
	}

	return rootCmd.Execute()
}

func init() {
	// Assign Run in init() to break the initialization cycle between
	// rootCmd and runInteractiveMenu (which references rootCmd).
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		runInteractiveMenu()
	}

	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Show detailed operation logs")
	rootCmd.PersistentFlags().BoolVar(&runAdmin, "admin", false, "Re-launch PureWin with administrator privileges (UAC)")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")

	// PersistentPreRun: if --admin is set, re-launch elevated and exit.
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		// Handle --no-color flag
		if noColor {
			os.Setenv("NO_COLOR", "1")
		}

		if !runAdmin {
			return
		}
		// Already elevated — nothing to do.
		if core.IsElevated() {
			return
		}
		// Build args without --admin to avoid infinite loop.
		var elevatedArgs []string
		for _, a := range os.Args[1:] {
			if a != "--admin" {
				elevatedArgs = append(elevatedArgs, a)
			}
		}
		if err := core.RunElevated(elevatedArgs); err != nil {
			fmt.Fprintf(os.Stderr, "%s %v\n", ui.IconError, err)
			os.Exit(1)
		}
	}

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

// runInteractiveShell launches the persistent interactive shell with
// slash-command autocomplete. The shell runs in a loop: each iteration
// runs a bubbletea program; when the user invokes a command, the shell
// exits, the command runs with full terminal control, then the shell
// relaunches with preserved state (output history, command history).
func runInteractiveShell() {
	// If VT processing failed, use a simple text-based REPL
	if !ui.IsVTEnabled() {
		runSimpleShell()
		return
	}

	m := shell.NewShellModel(appVersion)

	// Add welcome output on first launch.
	m.AppendOutput("")

	for {
		p := tea.NewProgram(m)
		finalModel, err := p.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s Shell error: %v\n", ui.IconError, err)
			os.Exit(1)
		}

		result, ok := finalModel.(shell.ShellModel)
		if !ok {
			return
		}

		// User quit the shell entirely.
		if result.Quitting {
			return
		}

		// Command dispatch: run the cobra subcommand with full terminal control.
		if result.ExecCmd != "" {
			cmdArgs := append([]string{result.ExecCmd}, result.ExecArgs...)
			result.AppendOutput("")

			// Run the subcommand via cobra.
			rootCmd.SetArgs(cmdArgs)
			if err := rootCmd.Execute(); err != nil {
				result.AppendOutput("  Command failed: " + err.Error())
			}

			result.AppendOutput("")

			// Clear the exec signal and relaunch shell.
			result.ExecCmd = ""
			result.ExecArgs = nil
		}

		// Preserve state for next iteration.
		m = result
	}
}

// runInteractiveMenu is kept for backward compatibility but now
// launches the interactive shell instead of the old menu.
func runInteractiveMenu() {
	runInteractiveShell()
}

// runSimpleShell provides a simple text-based REPL fallback when VT
// processing is unavailable (e.g., old Windows 10 cmd.exe without ANSI support).
func runSimpleShell() {
	fmt.Println()
	fmt.Printf("PureWin %s\n", appVersion)
	fmt.Println("Deep clean and optimize your Windows.")
	fmt.Println("Type /help for commands, /quit to exit.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("pw> ")
		if !scanner.Scan() {
			break
		}
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}

		if !strings.HasPrefix(raw, "/") {
			fmt.Println("  Unknown input. Type /help for commands.")
			continue
		}

		parts := strings.Fields(raw[1:])
		if len(parts) == 0 {
			continue
		}

		cmdName := strings.ToLower(parts[0])
		args := parts[1:]

		switch cmdName {
		case "quit", "exit":
			fmt.Println("Goodbye!")
			return
		case "help":
			printSimpleHelp()
		case "version":
			fmt.Printf("  PureWin %s\n", appVersion)
		default:
			// Dispatch to cobra. Silence cobra's own error/usage output
			// to avoid double-printing in the REPL.
			rootCmd.SilenceErrors = true
			rootCmd.SilenceUsage = true
			cmdArgs := append([]string{cmdName}, args...)
			rootCmd.SetArgs(cmdArgs)
			if err := rootCmd.Execute(); err != nil {
				fmt.Printf("  Error: %s\n", err)
				fmt.Println("  Type /help for available commands.")
			}
			fmt.Println()
		}
	}
}

// printSimpleHelp displays command help for the simple shell fallback.
func printSimpleHelp() {
	fmt.Println()
	fmt.Println("  Available commands:")
	fmt.Println()
	fmt.Println("    /clean        Deep clean system caches and temp files")
	fmt.Println("    /uninstall    Remove installed applications")
	fmt.Println("    /optimize     Speed up Windows with service tuning")
	fmt.Println("    /analyze      Explore disk space usage")
	fmt.Println("    /status       Live system health monitor")
	fmt.Println("    /purge        Clean project build artifacts")
	fmt.Println("    /installer    Find and remove old installer files")
	fmt.Println("    /update       Check for PureWin updates")
	fmt.Println("    /version      Show version info")
	fmt.Println("    /help         Show this help")
	fmt.Println("    /quit         Exit PureWin")
	fmt.Println()
}
