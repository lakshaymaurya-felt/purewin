package uninstall

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/windows/registry"
)

const (
	// uninstallTimeout is the maximum time to wait for an uninstall process.
	uninstallTimeout = 120 * time.Second
)

// msiGUIDPattern matches MSI product GUIDs like {XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX}.
var msiGUIDPattern = regexp.MustCompile(`\{[0-9A-Fa-f-]+\}`)

// uninstallStringPattern splits quoted and unquoted segments in UninstallString.
var uninstallStringPattern = regexp.MustCompile(`[^\s"]+|"([^"]*)"`)

// uninsPattern matches InnoSetup uninstaller executables like unins000.exe.
var uninsPattern = regexp.MustCompile(`unins\d+\.exe`)

// InstallerType represents the detected installer technology.
type InstallerType int

const (
	InstallerMSI InstallerType = iota
	InstallerSquirrel
	InstallerNSIS
	InstallerInnoSetup
	InstallerEdge
	InstallerGenericEXE
)

// ─── Public API ──────────────────────────────────────────────────────────────

// UninstallApp executes the uninstall command for the given application.
// If quiet is true and a QuietUninstallString is available, it is preferred.
// The process is given a 120-second timeout.
func UninstallApp(app InstalledApp, quiet bool) error {
	cmdStr := chooseUninstallCommand(app, quiet)
	if cmdStr == "" {
		return fmt.Errorf("no uninstall command found for %q", app.Name)
	}

	// Detect installer type and handle MSI specially.
	installerType := detectInstallerType(cmdStr)
	if installerType == InstallerMSI {
		return runMSIUninstall(cmdStr, quiet)
	}

	// Edge requires registry preparation before uninstall can proceed.
	if installerType == InstallerEdge {
		if err := prepareEdgeUninstall(); err != nil {
			return fmt.Errorf("failed to prepare Edge uninstall: %w", err)
		}
		// Run the uninstall. On failure: clean up stub AND restart Edge services
		// so Edge isn't left in a broken state. On success: stub MUST remain to
		// prevent Windows from re-provisioning Edge on future updates.
		uninstallErr := runUninstallCommand(cmdStr, installerType, quiet)
		if uninstallErr != nil {
			cleanupEdgeStub()
			restartEdgeServices()
		}
		return uninstallErr
	}

	// For non-MSI installers, parse the command and apply silent flags if needed.
	return runUninstallCommand(cmdStr, installerType, quiet)
}

// ─── Internal Helpers ────────────────────────────────────────────────────────

// parseUninstallString splits an UninstallString into executable path and arguments.
// It handles quoted paths with spaces correctly.
// Example: `"C:\Program Files\App\uninstall.exe" /S` → ["C:\Program Files\App\uninstall.exe", "/S"]
func parseUninstallString(cmdStr string) (string, []string) {
	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return "", nil
	}

	// Use regex to split on spaces while respecting quoted segments.
	matches := uninstallStringPattern.FindAllStringSubmatch(cmdStr, -1)
	var parts []string
	for _, match := range matches {
		// If match[1] is non-empty, it's a quoted string.
		if match[1] != "" {
			parts = append(parts, match[1])
		} else {
			parts = append(parts, match[0])
		}
	}

	if len(parts) == 0 {
		return "", nil
	}

	// First part is the executable, rest are arguments.
	exe := parts[0]
	args := parts[1:]

	// Strip surrounding quotes from executable if present.
	exe = strings.Trim(exe, `"`)

	return exe, args
}

// detectInstallerType analyzes the uninstall command to determine installer technology.
// Detection is based on the executable name (not the full path) to avoid false matches
// from directory names containing patterns like "update.exe" or "uninst".
func detectInstallerType(cmdStr string) InstallerType {
	lower := strings.ToLower(cmdStr)

	// Extract executable name for all path-based detection.
	exe, _ := parseUninstallString(cmdStr)
	exeName := strings.ToLower(filepath.Base(exe))

	// Check for MSI (msiexec.exe).
	if exeName == "msiexec.exe" || exeName == "msiexec" {
		return InstallerMSI
	}

	// Check for Microsoft Edge (setup.exe --uninstall --msedge).
	// --msedge is in args, so we check the full string for it, but exe must be setup.exe.
	if exeName == "setup.exe" && strings.Contains(lower, "--msedge") {
		return InstallerEdge
	}

	// Check for Squirrel/Electron (Update.exe).
	if exeName == "update.exe" {
		return InstallerSquirrel
	}

	// Check for InnoSetup (unins000.exe, unins001.exe, etc.).
	if uninsPattern.MatchString(exeName) {
		return InstallerInnoSetup
	}

	// Check for NSIS (commonly named uninst.exe, uninstall.exe, etc.).
	if strings.Contains(exeName, "uninst") {
		return InstallerNSIS
	}

	// Default to generic EXE.
	return InstallerGenericEXE
}

// applySilentFlags adds installer-specific silent/required flags to the arguments.
// For most installers, silent flags are only added when quiet=true.
// Exception: Edge ALWAYS needs --force-uninstall regardless of quiet mode.
func applySilentFlags(args []string, installerType InstallerType, quiet bool) []string {
	// Edge always needs --force-uninstall (not a silent flag — required for uninstall to work).
	if installerType == InstallerEdge {
		hasForce := false
		for _, arg := range args {
			if strings.EqualFold(arg, "--force-uninstall") {
				hasForce = true
				break
			}
		}
		if !hasForce {
			args = append(args, "--force-uninstall")
		}
		return args
	}

	if !quiet {
		return args
	}

	switch installerType {
	case InstallerSquirrel:
		// Ensure --uninstall flag is present for Squirrel.
		hasUninstallFlag := false
		for _, arg := range args {
			if strings.EqualFold(arg, "--uninstall") || strings.EqualFold(arg, "-uninstall") {
				hasUninstallFlag = true
				break
			}
		}
		if !hasUninstallFlag {
			args = append(args, "--uninstall")
		}
		// Add silent flag if not present.
		hasSilentFlag := false
		for _, arg := range args {
			if strings.EqualFold(arg, "-s") || strings.EqualFold(arg, "--silent") {
				hasSilentFlag = true
				break
			}
		}
		if !hasSilentFlag {
			args = append(args, "-s")
		}

	case InstallerNSIS:
		// NSIS uses /S for silent (case-sensitive — must be uppercase).
		hasS := false
		for _, arg := range args {
			if arg == "/S" {
				hasS = true
				break
			}
		}
		if !hasS {
			args = append(args, "/S")
		}

	case InstallerInnoSetup:
		// InnoSetup uses /VERYSILENT /SUPPRESSMSGBOXES /NORESTART.
		flags := []string{"/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART"}
		for _, flag := range flags {
			hasFlag := false
			for _, arg := range args {
				if strings.EqualFold(arg, flag) {
					hasFlag = true
					break
				}
			}
			if !hasFlag {
				args = append(args, flag)
			}
		}

	// InstallerEdge is handled above (before the quiet check) — always needs --force-uninstall.

	case InstallerGenericEXE:
		// Try /S as the most common silent flag.
		hasS := false
		for _, arg := range args {
			if strings.EqualFold(arg, "/S") {
				hasS = true
				break
			}
		}
		if !hasS {
			args = append(args, "/S")
		}

	// MSI is handled separately in runMSIUninstall, so no action needed here.
	case InstallerMSI:
		// No-op
	}

	return args
}

// chooseUninstallCommand selects the appropriate uninstall string.
func chooseUninstallCommand(app InstalledApp, quiet bool) string {
	if quiet && app.QuietUninstallString != "" {
		return app.QuietUninstallString
	}
	return app.UninstallString
}

// runMSIUninstall extracts the GUID and runs msiexec with proper flags.
func runMSIUninstall(cmdStr string, quiet bool) error {
	guid := msiGUIDPattern.FindString(cmdStr)
	if guid == "" {
		// Fallback to running the raw command if we can't parse the GUID.
		// Treat it as generic EXE for the fallback.
		return runUninstallCommand(cmdStr, InstallerGenericEXE, quiet)
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

// prepareEdgeUninstall sets required registry keys and stub files to allow Edge removal.
// Without this, Edge's setup.exe returns exit code 93 (uninstall blocked).
// Based on the proven approach used by Win11Debloat (10k+ stars), ChrisTitusTech/winutil
// (47k+ stars), RyTuneX, and other production tools.
func prepareEdgeUninstall() error {
	// 1. Stop Edge Update services first — the SCM can restart killed processes.
	edgeServices := []string{"edgeupdate", "edgeupdatem"}
	for _, svc := range edgeServices {
		svcCtx, svcCancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = exec.CommandContext(svcCtx, "sc", "stop", svc).Run()
		svcCancel()
	}

	// 2. Kill all Edge processes — they hold file locks and registry handles.
	edgeProcesses := []string{"msedge.exe", "msedgewebview2.exe", "MicrosoftEdgeUpdate.exe"}
	for _, proc := range edgeProcesses {
		killCtx, killCancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = exec.CommandContext(killCtx, "taskkill", "/F", "/IM", proc, "/T").Run()
		killCancel()
	}

	// 3. Create EdgeUpdateDev key with AllowUninstall (empty string value).
	// Must use WOW6432Node path since Edge installer is 32-bit.
	// This is the CRITICAL step — if it fails, restart Edge services so we don't leave
	// Edge in a broken state (processes killed but registry unchanged).
	devKey, _, err := registry.CreateKey(
		registry.LOCAL_MACHINE,
		`SOFTWARE\WOW6432Node\Microsoft\EdgeUpdateDev`,
		registry.SET_VALUE,
	)
	if err != nil {
		restartEdgeServices()
		return fmt.Errorf("failed to create EdgeUpdateDev key (need admin): %w", err)
	}
	if err := devKey.SetStringValue("AllowUninstall", ""); err != nil {
		devKey.Close()
		restartEdgeServices()
		return fmt.Errorf("failed to set AllowUninstall: %w", err)
	}
	devKey.Close()

	// 4. Prevent Edge from reinstalling via Windows Update.
	euKey, _, err := registry.CreateKey(
		registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\EdgeUpdate`,
		registry.SET_VALUE,
	)
	if err == nil {
		_ = euKey.SetDWordValue("DoNotUpdateToEdgeWithChromium", 1) // Best effort.
		euKey.Close()
	}

	// 5. Remove NoRemove flag from Edge uninstall key (allows uninstall button in Settings too).
	edgeUninstallKey, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\Microsoft Edge`,
		registry.SET_VALUE,
	)
	if err == nil {
		_ = edgeUninstallKey.DeleteValue("NoRemove") // Best effort.
		edgeUninstallKey.Close()
	}

	// 6. Create Edge UWP stub directory + file.
	// This tricks the Chromium Edge uninstaller into thinking the legacy Edge UWP app exists,
	// which is a prerequisite for the uninstaller to proceed.
	sysRoot := os.Getenv("SystemRoot")
	if sysRoot == "" {
		restartEdgeServices()
		return fmt.Errorf("SystemRoot environment variable is not set")
	}
	stubDir := filepath.Join(sysRoot, "SystemApps", "Microsoft.MicrosoftEdge_8wekyb3d8bbwe")
	_ = os.MkdirAll(stubDir, 0o755) // Best effort — may already exist.
	stubFile := filepath.Join(stubDir, "MicrosoftEdge.exe")
	if _, statErr := os.Stat(stubFile); os.IsNotExist(statErr) {
		_ = os.WriteFile(stubFile, []byte{}, 0o644) // Empty stub file.
	}

	return nil
}

// restartEdgeServices attempts to restart Edge Update services after a failed preparation.
// Best effort — prevents leaving Edge in a broken state (services stopped but uninstall not proceeding).
func restartEdgeServices() {
	edgeServices := []string{"edgeupdate", "edgeupdatem"}
	for _, svc := range edgeServices {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = exec.CommandContext(ctx, "sc", "start", svc).Run()
		cancel()
	}
}

// cleanupEdgeStub removes ONLY the zero-byte stub file created by prepareEdgeUninstall.
// Does NOT use RemoveAll to avoid destroying pre-existing UWP Edge files on rollback.
func cleanupEdgeStub() {
	sysRoot := os.Getenv("SystemRoot")
	if sysRoot == "" {
		return // Cannot determine path safely — leave as-is.
	}
	stub := filepath.Join(sysRoot, "SystemApps", "Microsoft.MicrosoftEdge_8wekyb3d8bbwe", "MicrosoftEdge.exe")
	info, err := os.Stat(stub)
	if err == nil && info.Size() == 0 {
		_ = os.Remove(stub) // Only remove if it's our zero-byte stub, not a real executable.
	}
}

// runUninstallCommand runs an arbitrary uninstall command.
// This is the CRITICAL FIX for the Logseq bug: we parse the command string properly
// instead of passing it raw to cmd.exe, which allows quoted paths with spaces to work.
func runUninstallCommand(cmdStr string, installerType InstallerType, quiet bool) error {
	// Parse the uninstall string into executable and arguments.
	exe, args := parseUninstallString(cmdStr)
	if exe == "" {
		return fmt.Errorf("unable to parse uninstall command: %q", cmdStr)
	}

	// Apply installer-specific silent flags if quiet mode is enabled.
	args = applySilentFlags(args, installerType, quiet)

	ctx, cancel := context.WithTimeout(context.Background(), uninstallTimeout)
	defer cancel()

	// Execute the command directly (NOT via cmd.exe /C).
	cmd := exec.CommandContext(ctx, exe, args...)
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
		case 93:
			return fmt.Errorf("Edge uninstall blocked by system protection (exit code 93) — run with --admin and retry")
		case 1605:
			return fmt.Errorf("product is not currently installed (exit code 1605)")
		case 1641, 3010:
			// Restart required but uninstall itself succeeded — not an error.
			return nil
		default:
			outputStr := strings.TrimSpace(string(output))
			if len(outputStr) > 200 {
				// Truncate at a valid UTF-8 boundary to avoid producing invalid strings.
				outputStr = outputStr[:200]
				for len(outputStr) > 0 && !utf8.ValidString(outputStr) {
					outputStr = outputStr[:len(outputStr)-1]
				}
				outputStr += "..."
			}
			if outputStr != "" {
				return fmt.Errorf("uninstall failed (exit code %d): %s", code, outputStr)
			}
			return fmt.Errorf("uninstall failed (exit code %d)", code)
		}
	}

	return fmt.Errorf("uninstall command error: %w", err)
}
