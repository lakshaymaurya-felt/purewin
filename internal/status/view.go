package status

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lakshaymaurya-felt/winmole/internal/core"
	"github.com/lakshaymaurya-felt/winmole/internal/ui"
)

// ─── Palette ─────────────────────────────────────────────────────────────────

var (
	clrGreen  = lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#4ade80"}
	clrYellow = lipgloss.AdaptiveColor{Light: "#ca8a04", Dark: "#facc15"}
	clrOrange = lipgloss.AdaptiveColor{Light: "#ea580c", Dark: "#fb923c"}
	clrRed    = lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#f87171"}
	clrCyan   = lipgloss.AdaptiveColor{Light: "#0891b2", Dark: "#22d3ee"}
	clrPink   = lipgloss.AdaptiveColor{Light: "#db2777", Dark: "#f472b6"}
)

// ─── Top-level renderer ─────────────────────────────────────────────────────

func (m StatusModel) renderView() string {
	w := m.Width
	if w < 50 {
		w = 50
	}

	var s strings.Builder
	s.WriteString(m.renderTabs(w))
	s.WriteString("\n")

	if m.Metrics == nil {
		s.WriteString(lipgloss.NewStyle().
			Foreground(ui.ColorMuted).
			Italic(true).
			Render("  Collecting metrics…"))
		return s.String()
	}

	switch m.Tab {
	case TabOverview:
		s.WriteString(m.renderOverview(w))
	case TabCPU:
		s.WriteString(m.renderCPU(w))
	case TabMemory:
		s.WriteString(m.renderMemory(w))
	case TabDisk:
		s.WriteString(m.renderDisk(w))
	case TabNetwork:
		s.WriteString(m.renderNetwork(w))
	case TabProcesses:
		s.WriteString(m.renderProcesses(w))
	}

	s.WriteString("\n")
	s.WriteString(m.renderStatusFooter())
	return s.String()
}

// ─── Tab bar ─────────────────────────────────────────────────────────────────

func (m StatusModel) renderTabs(w int) string {
	active := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.ColorPrimary).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ui.ColorPrimary).
		Padding(0, 2)

	inactive := lipgloss.NewStyle().
		Foreground(ui.ColorMuted).
		Padding(0, 2)

	var tabs []string
	for i, name := range TabNames {
		label := fmt.Sprintf("%d·%s", i+1, name)
		if Tab(i) == m.Tab {
			tabs = append(tabs, active.Render(label))
		} else {
			tabs = append(tabs, inactive.Render(label))
		}
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)
	divider := lipgloss.NewStyle().
		Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", w))

	return bar + "\n" + divider
}

// ─── Overview tab ────────────────────────────────────────────────────────────

func (m StatusModel) renderOverview(w int) string {
	met := m.Metrics
	score := HealthScore(met)

	// Health score card.
	scoreColor := clrGreen
	scoreLabel := "EXCELLENT"
	switch {
	case score < 50:
		scoreColor = clrRed
		scoreLabel = "CRITICAL"
	case score < 70:
		scoreColor = clrOrange
		scoreLabel = "FAIR"
	case score < 90:
		scoreColor = clrYellow
		scoreLabel = "GOOD"
	}

	scoreBox := lipgloss.NewStyle().
		Bold(true).
		Foreground(scoreColor).
		Render(fmt.Sprintf("  %d  %s", score, scoreLabel))

	// Hardware info card.
	hw := met.Hardware
	ramStr := core.FormatSize(int64(hw.RAMTotal))
	hwLines := []string{
		fmt.Sprintf("  Computer   %s", hw.Hostname),
		fmt.Sprintf("  OS         %s %s", hw.OS, hw.OSVersion),
		fmt.Sprintf("  CPU        %s", hw.CPUModel),
		fmt.Sprintf("  Cores      %d", hw.CPUCores),
		fmt.Sprintf("  RAM        %s", ramStr),
		fmt.Sprintf("  Arch       %s", hw.Architecture),
	}
	if met.GPU.Name != "" {
		hwLines = append(hwLines,
			fmt.Sprintf("  GPU        %s", met.GPU.Name))
	}
	if met.Battery.HasBattery {
		batt := fmt.Sprintf("%d%%", met.Battery.Charge)
		if met.Battery.IsCharging {
			batt += " ⚡ charging"
		}
		hwLines = append(hwLines,
			fmt.Sprintf("  Battery    %s", batt))
	}

	hwCard := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorSecondary).
		Padding(0, 1).
		Render(strings.Join(hwLines, "\n"))

	// Quick summary bars.
	barW := 24
	if w > 100 {
		barW = 32
	}
	cpuBar := colorBar(met.CPU.TotalPercent, barW)
	memBar := colorBar(met.Memory.UsedPercent, barW)

	var diskSummary string
	if len(met.Disk.Partitions) > 0 {
		p := met.Disk.Partitions[0]
		diskSummary = fmt.Sprintf("  DSK  %s  %5.1f%%  %s / %s  (%s)",
			colorBar(p.UsedPercent, barW), p.UsedPercent,
			core.FormatSize(int64(p.Used)), core.FormatSize(int64(p.Total)), p.Path)
	}

	netDown := formatSpeed(met.Network.RecvSpeed)
	netUp := formatSpeed(met.Network.SendSpeed)

	summary := strings.Join([]string{
		fmt.Sprintf("  CPU  %s  %5.1f%%", cpuBar, met.CPU.TotalPercent),
		fmt.Sprintf("  MEM  %s  %5.1f%%  %s / %s",
			memBar, met.Memory.UsedPercent,
			core.FormatSize(int64(met.Memory.Used)),
			core.FormatSize(int64(met.Memory.Total))),
		diskSummary,
		fmt.Sprintf("  NET  ↓ %s  ↑ %s", netDown, netUp),
	}, "\n")

	summaryCard := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorMuted).
		Padding(0, 1).
		Render(summary)

	return lipgloss.JoinVertical(lipgloss.Left, "", scoreBox, "", hwCard, "", summaryCard)
}

// ─── CPU tab ─────────────────────────────────────────────────────────────────

func (m StatusModel) renderCPU(w int) string {
	met := m.Metrics
	barW := 40
	if w > 110 {
		barW = 56
	}

	var lines []string
	lines = append(lines, "")
	lines = append(lines,
		fmt.Sprintf("  Total  %s  %5.1f%%", colorBar(met.CPU.TotalPercent, barW), met.CPU.TotalPercent))
	lines = append(lines, "")

	// Sparkline history.
	if len(m.CPUHistory) > 1 {
		spark := sparklineF64(m.CPUHistory, 30)
		lines = append(lines,
			lipgloss.NewStyle().Foreground(clrCyan).Render("  History  "+spark))
		lines = append(lines, "")
	}

	// Per-core bars.
	for i, pct := range met.CPU.PerCore {
		coreBar := colorBar(pct, barW-10)
		lines = append(lines,
			fmt.Sprintf("  Core %-2d  %s  %5.1f%%", i, coreBar, pct))
	}

	return strings.Join(lines, "\n")
}

// ─── Memory tab ──────────────────────────────────────────────────────────────

func (m StatusModel) renderMemory(w int) string {
	met := m.Metrics
	barW := 40
	if w > 110 {
		barW = 56
	}

	var lines []string
	lines = append(lines, "")
	lines = append(lines,
		fmt.Sprintf("  Used       %s  %5.1f%%",
			colorBar(met.Memory.UsedPercent, barW), met.Memory.UsedPercent))
	lines = append(lines, "")
	lines = append(lines,
		fmt.Sprintf("  Total      %s", core.FormatSize(int64(met.Memory.Total))))
	lines = append(lines,
		fmt.Sprintf("  Used       %s", core.FormatSize(int64(met.Memory.Used))))
	lines = append(lines,
		fmt.Sprintf("  Available  %s", core.FormatSize(int64(met.Memory.Available))))
	lines = append(lines,
		fmt.Sprintf("  Free       %s", core.FormatSize(int64(met.Memory.Free))))

	if met.Memory.SwapTotal > 0 {
		lines = append(lines, "")
		lines = append(lines,
			fmt.Sprintf("  Swap       %s  %5.1f%%",
				colorBar(met.Memory.SwapPercent, barW), met.Memory.SwapPercent))
		lines = append(lines,
			fmt.Sprintf("  Swap Used  %s / %s",
				core.FormatSize(int64(met.Memory.SwapUsed)),
				core.FormatSize(int64(met.Memory.SwapTotal))))
	}

	return strings.Join(lines, "\n")
}

// ─── Disk tab ────────────────────────────────────────────────────────────────

func (m StatusModel) renderDisk(w int) string {
	met := m.Metrics
	barW := 36
	if w > 110 {
		barW = 48
	}

	var lines []string
	lines = append(lines, "")

	for _, p := range met.Disk.Partitions {
		lines = append(lines,
			fmt.Sprintf("  %-4s %s  %5.1f%%  %s / %s",
				p.Path, colorBar(p.UsedPercent, barW), p.UsedPercent,
				core.FormatSize(int64(p.Used)),
				core.FormatSize(int64(p.Total))))
	}

	lines = append(lines, "")
	lines = append(lines,
		fmt.Sprintf("  Read   %s   Write  %s",
			core.FormatSize(int64(met.Disk.ReadBytes)),
			core.FormatSize(int64(met.Disk.WriteBytes))))

	return strings.Join(lines, "\n")
}

// ─── Network tab ─────────────────────────────────────────────────────────────

func (m StatusModel) renderNetwork(w int) string {
	met := m.Metrics

	var lines []string
	lines = append(lines, "")

	downStyle := lipgloss.NewStyle().Foreground(clrCyan)
	upStyle := lipgloss.NewStyle().Foreground(clrPink)

	lines = append(lines,
		fmt.Sprintf("  %s Download  %s",
			downStyle.Render("↓"), formatSpeed(met.Network.RecvSpeed)))
	lines = append(lines,
		fmt.Sprintf("  %s Upload    %s",
			upStyle.Render("↑"), formatSpeed(met.Network.SendSpeed)))

	lines = append(lines, "")
	lines = append(lines,
		fmt.Sprintf("  Total Recv  %s", core.FormatSize(int64(met.Network.BytesRecv))))
	lines = append(lines,
		fmt.Sprintf("  Total Sent  %s", core.FormatSize(int64(met.Network.BytesSent))))

	// Sparklines.
	if len(m.NetRecvHistory) > 1 {
		lines = append(lines, "")
		lines = append(lines,
			downStyle.Render("  ↓ ")+sparklineU64(m.NetRecvHistory, 30))
		lines = append(lines,
			upStyle.Render("  ↑ ")+sparklineU64(m.NetSendHistory, 30))
	}

	return strings.Join(lines, "\n")
}

// ─── Processes tab ───────────────────────────────────────────────────────────

func (m StatusModel) renderProcesses(w int) string {
	met := m.Metrics
	barW := 24
	if w > 100 {
		barW = 32
	}

	var lines []string
	lines = append(lines, "")
	lines = append(lines,
		lipgloss.NewStyle().Bold(true).Foreground(ui.ColorSecondary).
			Render("  Top processes by CPU"))
	lines = append(lines, "")

	nameW := 22
	if w > 100 {
		nameW = 30
	}

	header := fmt.Sprintf("  %-6s %-*s %s  %6s  %6s", "PID", nameW, "Name", strings.Repeat(" ", barW), "CPU%", "Mem%")
	lines = append(lines, lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(header))
	lines = append(lines, lipgloss.NewStyle().Foreground(ui.ColorMuted).Render("  "+strings.Repeat("─", w-4)))

	for _, p := range met.TopProcs {
		name := p.Name
		if len(name) > nameW {
			name = name[:nameW-1] + "…"
		}
		cpuClamp := p.CPUPct
		if cpuClamp > 100 {
			cpuClamp = 100
		}
		bar := colorBar(cpuClamp, barW)
		lines = append(lines,
			fmt.Sprintf("  %-6d %-*s %s  %5.1f%%  %5.1f%%",
				p.PID, nameW, name, bar, p.CPUPct, p.MemPct))
	}

	if len(met.TopProcs) == 0 {
		lines = append(lines,
			lipgloss.NewStyle().Foreground(ui.ColorMuted).Italic(true).
				Render("  (no process data yet)"))
	}

	return strings.Join(lines, "\n")
}

// ─── Footer ──────────────────────────────────────────────────────────────────

func (m StatusModel) renderStatusFooter() string {
	hints := "  Tab/Shift-Tab switch  " + ui.IconPipe + "  1-6 jump  " + ui.IconPipe + "  q quit"
	footer := lipgloss.NewStyle().
		Foreground(ui.ColorMuted).
		Italic(true).
		Render(hints)

	if m.Err != nil {
		errStr := lipgloss.NewStyle().
			Foreground(ui.ColorError).
			Render("  " + ui.IconError + " " + m.Err.Error())
		return errStr + "\n" + footer
	}
	return footer
}

// ─── Drawing primitives ─────────────────────────────────────────────────────

// colorBar renders a ████░░░░ bar colored by severity.
func colorBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}

	barColor := clrGreen
	switch {
	case pct >= 90:
		barColor = clrRed
	case pct >= 75:
		barColor = clrOrange
	case pct >= 50:
		barColor = clrYellow
	}

	fStr := lipgloss.NewStyle().Foreground(barColor).Render(strings.Repeat("█", filled))
	eStr := lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(strings.Repeat("░", width-filled))
	return fStr + eStr
}

// sparklineF64 renders a mini chart from float64 data using block chars.
func sparklineF64(data []float64, width int) string {
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	var maxVal float64
	for _, v := range data {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal == 0 {
		maxVal = 1
	}

	d := data
	if len(d) > width {
		d = d[len(d)-width:]
	}

	var b strings.Builder
	for _, v := range d {
		idx := int(v / maxVal * 7)
		if idx > 7 {
			idx = 7
		}
		if idx < 0 {
			idx = 0
		}
		b.WriteRune(blocks[idx])
	}
	// Pad remaining width.
	for i := len(d); i < width; i++ {
		b.WriteRune(blocks[0])
	}
	return lipgloss.NewStyle().Foreground(clrCyan).Render(b.String())
}

// sparklineU64 renders a mini chart from uint64 data.
func sparklineU64(data []uint64, width int) string {
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	var maxVal uint64
	for _, v := range data {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal == 0 {
		maxVal = 1
	}

	d := data
	if len(d) > width {
		d = d[len(d)-width:]
	}

	var b strings.Builder
	for _, v := range d {
		idx := int(float64(v) / float64(maxVal) * 7)
		if idx > 7 {
			idx = 7
		}
		b.WriteRune(blocks[idx])
	}
	for i := len(d); i < width; i++ {
		b.WriteRune(blocks[0])
	}
	return lipgloss.NewStyle().Foreground(clrCyan).Render(b.String())
}

// formatSpeed returns a human-readable bytes/sec string.
func formatSpeed(bps uint64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case bps >= gb:
		return fmt.Sprintf("%.1f GB/s", float64(bps)/float64(gb))
	case bps >= mb:
		return fmt.Sprintf("%.1f MB/s", float64(bps)/float64(mb))
	case bps >= kb:
		return fmt.Sprintf("%.1f KB/s", float64(bps)/float64(kb))
	default:
		return fmt.Sprintf("%d B/s", bps)
	}
}
