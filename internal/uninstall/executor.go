package uninstall

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	// uninstallTimeout is the maximum time to wait for an uninstall process.
	uninstallTimeout = 120 * time.Second
)

// msiGUIDPattern matches MSI product GUIDs like {XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX}.
var msiGUIDPattern = regexp.MustCompile(`\{[0-9A-Fa-f-]+\}`)

// ─── Public API ──────────────────────────────────────────────────────────────

// UninstallApp executes the uninstall command for the given application.
// If quiet is true and a QuietUninstallString is available, it is preferred.
// The process is given a 120-second timeout.
func UninstallApp(app InstalledApp, quiet bool) error {
	cmdStr := chooseUninstallCommand(app, quiet)
	if cmdStr == "" {
		return fmt.Errorf("no uninstall command found for %q", app.Name)
	}

	// Detect MSI-based uninstalls and handle them specially.
	if isMSIUninstall(cmdStr) {
		return runMSIUninstall(cmdStr, quiet)
	}

	return runUninstallCommand(cmdStr)
}

// ─── Internal Helpers ────────────────────────────────────────────────────────

// chooseUninstallCommand selects the appropriate uninstall string.
func chooseUninstallCommand(app InstalledApp, quiet bool) string {
	if quiet && app.QuietUninstallString != "" {
		return app.QuietUninstallString
	}
	return app.UninstallString
}

// isMSIUninstall returns true if the command invokes msiexec.
func isMSIUninstall(cmd string) bool {
	return strings.Contains(strings.ToLower(cmd), "msiexec")
}

// runMSIUninstall extracts the GUID and runs msiexec with proper flags.
func runMSIUninstall(cmdStr string, quiet bool) error {
	guid := msiGUIDPattern.FindString(cmdStr)
	if guid == "" {
		// Fallback to running the raw command if we can't parse the GUID.
		return runUninstallCommand(cmdStr)
	}

	args := []string{"/x", guid}
	if quiet {
		args = append(args, "/qn", "/norestart")
	}

	ctx, cancel := context.WithTimeout(context.Background(), uninstallTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "msiexec.exe", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return handleExitError(err, output)
	}
	return nil
}

// runUninstallCommand runs an arbitrary uninstall command string via cmd.exe.
func runUninstallCommand(cmdStr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), uninstallTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "cmd.exe", "/C", cmdStr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return handleExitError(err, output)
	}
	return nil
}

// handleExitError wraps an exec error with contextual information.
// Common MSI exit codes are translated to human-readable messages.
func handleExitError(err error, output []byte) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("uninstall timed out after %s", uninstallTimeout)
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		switch code {
		case 1605:
			return fmt.Errorf("product is not currently installed (exit code 1605)")
		case 1641:
			// Restart required but uninstall itself succeeded.
			return fmt.Errorf("uninstall succeeded — restart required (exit code 1641)")
		case 3010:
			// Restart required but uninstall itself succeeded.
			return fmt.Errorf("uninstall succeeded — restart required (exit code 3010)")
		default:
			outputStr := strings.TrimSpace(string(output))
			if len(outputStr) > 200 {
				outputStr = outputStr[:200] + "..."
			}
			if outputStr != "" {
				return fmt.Errorf("uninstall failed (exit code %d): %s", code, outputStr)
			}
			return fmt.Errorf("uninstall failed (exit code %d)", code)
		}
	}

	return fmt.Errorf("uninstall command error: %w", err)
}
