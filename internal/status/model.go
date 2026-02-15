package status

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ─── Tab enumeration ─────────────────────────────────────────────────────────

// Tab identifies one of the dashboard sections.
type Tab int

const (
	TabOverview Tab = iota
	TabCPU
	TabMemory
	TabDisk
	TabNetwork
	TabProcesses
)

// TabNames is the display label for each tab.
var TabNames = []string{"Overview", "CPU", "Memory", "Disk", "Network", "Processes"}

// ─── Messages ────────────────────────────────────────────────────────────────

type tickMsg time.Time

type metricsMsg struct {
	metrics *SystemMetrics
	err     error
}

// ─── Model ───────────────────────────────────────────────────────────────────

// StatusModel is the bubbletea Model for the system health dashboard.
type StatusModel struct {
	Metrics         *SystemMetrics
	prevNet         *NetworkMetrics
	Tab             Tab
	Width           int
	Height          int
	refreshInterval time.Duration
	quitting        bool
	Err             error

	// Sparkline ring buffers (last 60 readings).
	NetSendHistory []uint64
	NetRecvHistory []uint64
	CPUHistory     []float64
	MemHistory     []float64
}

// NewStatusModel creates a StatusModel with the given refresh cadence.
func NewStatusModel(refreshInterval time.Duration) StatusModel {
	if refreshInterval <= 0 {
		refreshInterval = time.Second
	}
	return StatusModel{
		Width:           80,
		Height:          24,
		refreshInterval: refreshInterval,
	}
}

func (m StatusModel) doTick() tea.Cmd {
	return tea.Tick(m.refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m StatusModel) collectMetrics() tea.Cmd {
	prevNet := m.prevNet
	interval := m.refreshInterval
	return func() tea.Msg {
		metrics, err := CollectMetrics(prevNet, interval)
		return metricsMsg{metrics: metrics, err: err}
	}
}

// ─── tea.Model interface ─────────────────────────────────────────────────────

func (m StatusModel) Init() tea.Cmd {
	// Immediately start collecting; the first metricsMsg will trigger the tick
	// loop, keeping collection and display strictly sequential.
	return m.collectMetrics()
}

func (m StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "tab":
			m.Tab = (m.Tab + 1) % Tab(len(TabNames))
		case "shift+tab":
			if m.Tab == 0 {
				m.Tab = Tab(len(TabNames) - 1)
			} else {
				m.Tab--
			}
		case "1":
			m.Tab = TabOverview
		case "2":
			m.Tab = TabCPU
		case "3":
			m.Tab = TabMemory
		case "4":
			m.Tab = TabDisk
		case "5":
			m.Tab = TabNetwork
		case "6":
			m.Tab = TabProcesses
		}
		return m, nil

	case tickMsg:
		return m, m.collectMetrics()

	case metricsMsg:
		if msg.err != nil {
			m.Err = msg.err
			return m, m.doTick()
		}
		m.Metrics = msg.metrics
		m.prevNet = &msg.metrics.Network

		// Append to sparkline histories (cap at 60).
		m.CPUHistory = appendF64(m.CPUHistory, msg.metrics.CPU.TotalPercent, 60)
		m.MemHistory = appendF64(m.MemHistory, msg.metrics.Memory.UsedPercent, 60)
		m.NetSendHistory = appendU64(m.NetSendHistory, msg.metrics.Network.SendSpeed, 60)
		m.NetRecvHistory = appendU64(m.NetRecvHistory, msg.metrics.Network.RecvSpeed, 60)

		return m, m.doTick()
	}

	return m, nil
}

func (m StatusModel) View() string {
	if m.quitting {
		return ""
	}
	return m.renderView()
}

// ─── History helpers ─────────────────────────────────────────────────────────

func appendF64(h []float64, v float64, maxLen int) []float64 {
	h = append(h, v)
	if len(h) > maxLen {
		h = h[1:]
	}
	return h
}

func appendU64(h []uint64, v uint64, maxLen int) []uint64 {
	h = append(h, v)
	if len(h) > maxLen {
		h = h[1:]
	}
	return h
}
