package analyze

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/lakshaymaurya-felt/purewin/internal/ui"
)

// ─── Color tokens ────────────────────────────────────────────────────────────

// Short aliases for readability in render functions.
// Coral accent gives the analyzer its own visual identity.
var (
	clrDim    = ui.ColorMuted
	clrDir    = ui.ColorCoral // coral for analyzer directories
	clrFile   = ui.ColorText
	clrOld    = ui.ColorMuted
	clrLarge  = ui.ColorWarning
	clrCursor = ui.ColorPrimary
)

// ─── Top-level view ──────────────────────────────────────────────────────────

func (m AnalyzeModel) renderView() string {
	if m.quitting {
		return ""
	}
	w := m.width
	if w < 40 {
		w = 40
	}

	var s strings.Builder
	s.WriteString(m.renderHeader(w))
	s.WriteString("\n")

	if m.searching {
		s.WriteString(m.renderSearchInput(w))
		s.WriteString("\n")
		s.WriteString(m.renderSearchResults(w))
	} else {
		s.WriteString(m.renderBody(w))
	}

	s.WriteString("\n")
	s.WriteString(m.renderFooter(w))
	return s.String()
}

// ─── Header ──────────────────────────────────────────────────────────────────

func (m AnalyzeModel) renderHeader(w int) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.ColorCoral).
		Render("  " + ui.IconDiamond + " Disk Analyzer")

	sizeStr := ui.FormatSize(m.current.Size)
	pathLine := lipgloss.NewStyle().
		Foreground(ui.ColorTextDim).
		Render(fmt.Sprintf("  %s    %s", m.current.Path, sizeStr))

	// Breadcrumb trail.
	var crumbs []string
	for _, bc := range m.breadcrumb {
		crumbs = append(crumbs, bc.Name)
	}
	crumbs = append(crumbs, m.current.Name)
	bcStr := lipgloss.NewStyle().
		Foreground(ui.ColorMuted).
		Render("  " + strings.Join(crumbs, " "+ui.IconChevron+" "))

	inner := lipgloss.JoinVertical(lipgloss.Left, title, pathLine, bcStr)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorCoral).
		Width(w - 2).
		Render(inner)
}

// ─── Body (file list) ────────────────────────────────────────────────────────

func (m AnalyzeModel) renderBody(w int) string {
	items := m.visibleItems()
	if len(items) == 0 {
		return lipgloss.NewStyle().
			Foreground(ui.ColorMuted).
			Italic(true).
			Render("  (empty directory)")
	}

	vh := m.viewportHeight()
	barWidth := 20
	if w > 110 {
		barWidth = 30
	} else if w > 90 {
		barWidth = 25
	}

	parentSize := m.current.Size
	var lines []string

	for i := m.offset; i < len(items) && i < m.offset+vh; i++ {
		lines = append(lines, m.renderEntry(i+1, items[i], parentSize, barWidth, i == m.cursor))
	}

	// Scrollbar hint.
	if len(items) > vh {
		pct := float64(m.offset) / float64(len(items)-vh) * 100
		scrollHint := lipgloss.NewStyle().
			Foreground(ui.ColorMuted).
			Italic(true).
			Render(fmt.Sprintf("  ── %d/%d items  (%.0f%%) ──", min(m.offset+vh, len(items)), len(items), pct))
		lines = append(lines, scrollHint)
	}

	return strings.Join(lines, "\n")
}

func (m AnalyzeModel) renderEntry(num int, entry *DirEntry, parentSize int64, barWidth int, selected bool) string {
	pct := entry.Percentage(parentSize)

	// ── Size bar ─────────────────────────────────────────────
	bar := ui.GradientBar(pct, barWidth)

	// ── Icon ─────────────────────────────────────────────────
	icon := ui.IconBullet + " "
	if entry.IsDir {
		icon = ui.IconFolder
	}

	// ── Name ─────────────────────────────────────────────────
	nameColor := clrFile
	if entry.IsDir {
		nameColor = clrDir
	}
	if entry.IsOld() {
		nameColor = clrOld
	}
	if !entry.IsDir && entry.Size >= 100*(1<<20) {
		nameColor = clrLarge
	}

	maxName := m.width - barWidth - 38
	if maxName < 12 {
		maxName = 12
	}
	name := entry.Name
	if len(name) > maxName {
		name = name[:maxName-1] + "…"
	}
	nameStr := lipgloss.NewStyle().Foreground(nameColor).Bold(entry.IsDir).Render(name)

	// ── Metadata columns ─────────────────────────────────────
	numStr := lipgloss.NewStyle().Foreground(clrDim).Render(fmt.Sprintf("%3d.", num))
	pctStr := lipgloss.NewStyle().Foreground(ui.ColorTextDim).Render(fmt.Sprintf("%5.1f%%", pct))
	sizeStr := ui.FormatSize(entry.Size)

	age := "     "
	if entry.IsOld() {
		age = ui.TagWarningStyle().Render(" >6mo ")
	}

	// ── Assemble ─────────────────────────────────────────────
	line := fmt.Sprintf("  %s %s  %s  %s %s  %s  %s",
		numStr, bar, pctStr, icon, nameStr, sizeStr, age)

	if selected {
		cursor := lipgloss.NewStyle().Foreground(clrCursor).Bold(true).Render(ui.IconBlock)
		line = " " + cursor + line[2:]
		if m.confirmDelete {
			line += lipgloss.NewStyle().
				Foreground(ui.ColorError).
				Bold(true).
				Render("  " + ui.IconWarning + " Press Enter to delete")
		}
	}

	return line
}

// ─── Search UI ───────────────────────────────────────────────────────────────

func (m AnalyzeModel) renderSearchInput(w int) string {
	prompt := lipgloss.NewStyle().
		Foreground(ui.ColorCoral).
		Bold(true).
		Render("  / ")

	query := lipgloss.NewStyle().
		Foreground(ui.ColorText).
		Render(m.searchQuery)

	cursor := lipgloss.NewStyle().
		Foreground(ui.ColorCoral).
		Render("▎")

	return prompt + query + cursor
}

func (m AnalyzeModel) renderSearchResults(w int) string {
	if m.searchQuery == "" {
		return lipgloss.NewStyle().
			Foreground(ui.ColorMuted).
			Italic(true).
			Render("  Type to search across all files and directories…")
	}

	if len(m.searchResults) == 0 {
		return lipgloss.NewStyle().
			Foreground(ui.ColorMuted).
			Italic(true).
			Render("  No matches found")
	}

	vh := m.viewportHeight()
	var lines []string

	// Calculate scroll offset for search results
	searchOffset := 0
	if m.searchCursor >= vh {
		searchOffset = m.searchCursor - vh + 1
	}

	for i := searchOffset; i < len(m.searchResults) && i < searchOffset+vh; i++ {
		r := m.searchResults[i]
		entry := r.Entry

		// Icon
		icon := ui.IconBullet + " "
		if entry.IsDir {
			icon = ui.IconFolder
		}

		// Name with path context
		nameColor := clrFile
		if entry.IsDir {
			nameColor = clrDir
		}

		// Show parent path for context
		parentPath := ""
		if entry.Parent != nil {
			parentPath = entry.Parent.Path
			// Truncate long paths — rune-safe to avoid splitting multi-byte UTF-8.
			maxPathLen := w - 50
			if maxPathLen < 20 {
				maxPathLen = 20
			}
			if runeCount := utf8.RuneCountInString(parentPath); runeCount > maxPathLen {
				runes := []rune(parentPath)
				parentPath = "…" + string(runes[runeCount-maxPathLen+1:])
			}
		}

		name := lipgloss.NewStyle().Foreground(nameColor).Bold(entry.IsDir).Render(entry.Name)
		path := lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(parentPath)
		size := ui.FormatSize(entry.Size)

		line := fmt.Sprintf("  %s %s  %s  %s", icon, name, size, path)

		if i == m.searchCursor {
			cursor := lipgloss.NewStyle().Foreground(clrCursor).Bold(true).Render(ui.IconBlock)
			line = " " + cursor + line[2:]
		}

		lines = append(lines, line)
	}

	// Result count
	countLine := lipgloss.NewStyle().
		Foreground(ui.ColorMuted).
		Italic(true).
		Render(fmt.Sprintf("  ── %d result(s) ──", len(m.searchResults)))
	lines = append(lines, countLine)

	return strings.Join(lines, "\n")
}

// ─── Footer ──────────────────────────────────────────────────────────────────

func (m AnalyzeModel) renderFooter(w int) string {
	var parts []string

	// Error line.
	if m.err != nil {
		parts = append(parts,
			lipgloss.NewStyle().
				Foreground(ui.ColorError).
				Render("  "+ui.IconError+" "+m.err.Error()))
	}

	if m.searching {
		// Search mode hints
		hints := []string{
			"↑↓ navigate",
			"Enter select",
			"Esc cancel",
		}
		hintStr := strings.Join(hints, " "+ui.IconPipe+" ")
		parts = append(parts, ui.HintBarStyle().Render("  "+hintStr))
		return strings.Join(parts, "\n")
	}

	// Filter indicator.
	if m.largeOnly {
		parts = append(parts,
			"  "+ui.TagWarningStyle().Render(" >100 MiB filter "))
	}

	// Normal mode keybindings — ADD "/ search"
	hints := []string{
		"↑↓ nav",
		"→ drill",
		"← back",
		"/ search",
		"Enter open",
		"⌫ delete",
		"L large",
		"q quit",
	}
	hintStr := strings.Join(hints, " "+ui.IconPipe+" ")
	parts = append(parts, ui.HintBarStyle().Render("  "+hintStr))

	return strings.Join(parts, "\n")
}
