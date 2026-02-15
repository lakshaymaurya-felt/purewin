package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"golang.org/x/sys/windows"
)

// ─── ASCII Mole Art ──────────────────────────────────────────────────────────

// moleLines holds the raw ASCII mole art, rendered line-by-line during intro.
var moleLines = []string{
	`     /\_/\     `,
	`    / o o \    `,
	`   (  =^=  )   `,
	`    )     (    `,
	`   (       )   `,
	`  ( /|   |\ )  `,
	`   \| |_| |/   `,
	`    \_____/    `,
}

// groundLine is the terrain beneath the mole.
var groundLine = `  ~~^^^~^^^~~^^^~~`

// brandBanner is the large ASCII wordmark.
var brandLines = []string{
	` __        ___       __  __       _      `,
	` \ \      / (_)_ __ |  \/  | ___ | | ___ `,
	`  \ \ /\ / /| | '_ \| |\/| |/ _ \| |/ _ \`,
	`   \ V  V / | | | | | |  | | (_) | |  __/`,
	`    \_/\_/  |_|_| |_|_|  |_|\___/|_|\___|`,
}

// tagline sits below the brand banner.
const tagline = "Deep clean and optimize your Windows."

// ─── Terminal Detection ──────────────────────────────────────────────────────

// isTerminal returns true if stdout is a terminal (not piped/redirected).
func isTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

// enableVTProcessing enables Virtual Terminal Processing on the Windows console
// so that ANSI escape codes work in cmd.exe and older PowerShell versions.
func enableVTProcessing() {
	stdout := windows.Handle(os.Stdout.Fd())
	var mode uint32
	_ = windows.GetConsoleMode(stdout, &mode)
	_ = windows.SetConsoleMode(stdout, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}

// ─── Intro Animation ─────────────────────────────────────────────────────────

// ShowMoleIntro displays an animated mole appearing line-by-line.
// Only runs in interactive terminals; silently returns otherwise.
// Dusty rose for the mole body, espresso brown for the ground.
func ShowMoleIntro() {
	if !isTerminal() {
		return
	}

	// Ensure ANSI escape sequences work on Windows consoles.
	enableVTProcessing()

	moleStyle := lipgloss.NewStyle().Foreground(ColorPurple)
	groundStyle := lipgloss.NewStyle().Foreground(ColorPrimary)

	// Clear screen.
	fmt.Print("\033[2J\033[H")

	// Animate mole body line by line.
	for _, line := range moleLines {
		fmt.Println(moleStyle.Render(line))
		time.Sleep(100 * time.Millisecond)
	}

	// Ground with a brief pause.
	fmt.Println(groundStyle.Render(groundLine))
	time.Sleep(100 * time.Millisecond)

	// Pause to admire the mole.
	time.Sleep(500 * time.Millisecond)

	// Clear screen before continuing to main UI.
	fmt.Print("\033[2J\033[H")
}

// ─── Brand Banner ────────────────────────────────────────────────────────────

// ShowBrandBanner returns the full ASCII brand banner as a styled string,
// ready to be printed. Espresso wordmark, mocha tagline, caramel URL.
func ShowBrandBanner() string {
	var b strings.Builder

	nameStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	tagStyle := lipgloss.NewStyle().Foreground(ColorTextDim).Italic(true)
	urlStyle := lipgloss.NewStyle().Foreground(ColorSecondary)

	// ASCII wordmark.
	for _, line := range brandLines {
		b.WriteString(nameStyle.Render(line))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')

	// Tagline.
	b.WriteString(tagStyle.Render("  " + tagline))
	b.WriteByte('\n')

	// URL / attribution.
	b.WriteString(urlStyle.Render("  https://github.com/lakshaymaurya-felt/winmole"))
	b.WriteByte('\n')

	return b.String()
}

// ─── Completion Banner ───────────────────────────────────────────────────────

// ShowCompletionBanner prints a post-operation summary with space freed,
// current free space, and a styled checkmark.
func ShowCompletionBanner(freed int64, freeSpace int64) {
	checkStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(ColorText)

	dividerWidth := 48

	fmt.Println()
	fmt.Println(Divider(dividerWidth))
	fmt.Println()

	// Checkmark + headline.
	fmt.Printf("  %s %s\n",
		checkStyle.Render(IconSuccess),
		checkStyle.Render("Cleanup Complete!"),
	)
	fmt.Println()

	// Space freed.
	fmt.Printf("  %s  %s\n",
		labelStyle.Render("Space freed:"),
		FormatSize(freed),
	)

	// Current free space.
	fmt.Printf("  %s  %s\n",
		labelStyle.Render("Free space: "),
		FormatSize(freeSpace),
	)

	fmt.Println()
	fmt.Println(Divider(dividerWidth))
	fmt.Println()
}

// ─── Mole Art (Static) ──────────────────────────────────────────────────────

// MoleArt returns the full mole ASCII art as a single styled string.
// Useful for embedding in help screens or about dialogs.
func MoleArt() string {
	moleStyle := lipgloss.NewStyle().Foreground(ColorPurple)
	groundStyle := lipgloss.NewStyle().Foreground(ColorPrimary)

	var b strings.Builder
	for _, line := range moleLines {
		b.WriteString(moleStyle.Render(line))
		b.WriteByte('\n')
	}
	b.WriteString(groundStyle.Render(groundLine))
	b.WriteByte('\n')
	return b.String()
}
