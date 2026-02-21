package analyze

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lakshaymaurya-felt/purewin/internal/core"
)

// searchTickMsg is sent after a debounce delay to trigger the actual search.
type searchTickMsg struct {
	query string
}

// searchDebounce is the delay before running SearchTree after a keystroke.
const searchDebounce = 150 * time.Millisecond

// ─── Messages ────────────────────────────────────────────────────────────────

type deleteResultMsg struct {
	path  string
	freed int64
	err   error
}

func deleteEntry(entry *DirEntry) tea.Cmd {
	return func() tea.Msg {
		freed, err := core.SafeDelete(entry.Path, false)
		return deleteResultMsg{path: entry.Path, freed: freed, err: err}
	}
}

// ─── Model ───────────────────────────────────────────────────────────────────

// AnalyzeModel is the bubbletea Model for the disk analyzer TUI.
type AnalyzeModel struct {
	root          *DirEntry
	current       *DirEntry   // directory being displayed
	cursor        int         // selected item index
	breadcrumb    []*DirEntry // navigation history stack
	width         int
	height        int
	offset        int  // viewport scroll offset
	largeOnly     bool // filter: show only >100MB
	confirmDelete bool // two-key delete: Backspace then Enter
	quitting      bool
	err           error
	maxDepth      int   // 0 = unlimited
	minSize       int64 // 0 = show all

	// Search state
	searching     bool           // true when in search mode
	searchQuery   string         // current search input
	searchResults []SearchResult // cached search results
	searchCursor  int            // cursor within search results
}

// NewAnalyzeModel creates an AnalyzeModel rooted at the given scan result.
func NewAnalyzeModel(root *DirEntry, maxDepth int, minSize int64) AnalyzeModel {
	return AnalyzeModel{
		root:     root,
		current:  root,
		width:    80,
		height:   24,
		maxDepth: maxDepth,
		minSize:  minSize,
	}
}

func (m AnalyzeModel) Init() tea.Cmd {
	return nil
}

func (m AnalyzeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Search mode input handling.
		if m.searching {
			switch msg.Type {
			case tea.KeyEscape:
				m.searching = false
				m.searchQuery = ""
				m.searchResults = nil
				m.searchCursor = 0
				return m, nil
			case tea.KeyEnter:
				if m.searchCursor >= 0 && m.searchCursor < len(m.searchResults) {
					m.navigateToEntry(m.searchResults[m.searchCursor].Entry)
					m.searching = false
					m.searchQuery = ""
					m.searchResults = nil
					m.searchCursor = 0
				}
				return m, nil
			case tea.KeyUp:
				if m.searchCursor > 0 {
					m.searchCursor--
				}
				return m, nil
			case tea.KeyDown:
				if m.searchCursor < len(m.searchResults)-1 {
					m.searchCursor++
				}
				return m, nil
			case tea.KeyBackspace:
				if len(m.searchQuery) > 0 {
					_, size := utf8.DecodeLastRuneInString(m.searchQuery)
					m.searchQuery = m.searchQuery[:len(m.searchQuery)-size]
					m.searchCursor = 0
					return m, tea.Tick(searchDebounce, func(t time.Time) tea.Msg {
						return searchTickMsg{query: m.searchQuery}
					})
				}
				return m, nil
			case tea.KeyRunes:
				m.searchQuery += string(msg.Runes)
				m.searchCursor = 0
				return m, tea.Tick(searchDebounce, func(t time.Time) tea.Msg {
					return searchTickMsg{query: m.searchQuery}
				})
			}
			return m, nil
		}

		// If awaiting delete confirmation, only Enter confirms.
		if m.confirmDelete {
			if msg.String() == "enter" {
				m.confirmDelete = false
				items := m.visibleItems()
				if m.cursor >= 0 && m.cursor < len(items) {
					return m, deleteEntry(items[m.cursor])
				}
			}
			m.confirmDelete = false
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			// In normal mode, esc also quits
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			}

		case "down", "j":
			items := m.visibleItems()
			if m.cursor < len(items)-1 {
				m.cursor++
				m.ensureVisible()
			}

		case "right", "l":
			// Drill into a directory.
			items := m.visibleItems()
			if m.cursor >= 0 && m.cursor < len(items) {
				entry := items[m.cursor]
				if entry.IsDir && len(entry.Children) > 0 {
					m.breadcrumb = append(m.breadcrumb, m.current)
					m.current = entry
					m.cursor = 0
					m.offset = 0
				}
			}

		case "enter":
			// Open file/folder location in Explorer.
			items := m.visibleItems()
			if m.cursor >= 0 && m.cursor < len(items) {
				openInExplorer(items[m.cursor].Path)
			}

		case "left", "h":
			// Go up to parent directory.
			if len(m.breadcrumb) > 0 {
				m.current = m.breadcrumb[len(m.breadcrumb)-1]
				m.breadcrumb = m.breadcrumb[:len(m.breadcrumb)-1]
				m.cursor = 0
				m.offset = 0
			}

		case "backspace":
			// First key of two-key delete confirmation.
			items := m.visibleItems()
			if m.cursor >= 0 && m.cursor < len(items) {
				m.confirmDelete = true
			}

		case "L":
			m.largeOnly = !m.largeOnly
			m.cursor = 0
			m.offset = 0

		case "/":
			m.searching = true
			m.searchQuery = ""
			m.searchResults = nil
			m.searchCursor = 0
			return m, nil
		}

		return m, nil

	case searchTickMsg:
		// Only execute search if query hasn't changed since the tick was scheduled (debounce).
		if m.searching && msg.query == m.searchQuery {
			m.searchResults = SearchTreeBounded(m.root, m.searchQuery, 50)
			// Clamp cursor — results may be shorter than the old list the user was navigating.
			if m.searchCursor >= len(m.searchResults) {
				m.searchCursor = 0
			}
		}
		return m, nil

	case deleteResultMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.removeEntry(msg.path)
		}
		return m, nil
	}

	return m, nil
}

// View delegates to view.go renderView.
func (m AnalyzeModel) View() string {
	return m.renderView()
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func (m *AnalyzeModel) ensureVisible() {
	vh := m.viewportHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vh {
		m.offset = m.cursor - vh + 1
	}
}

func (m *AnalyzeModel) viewportHeight() int {
	h := m.height - 8 // header (4) + footer (3) + padding
	if h < 1 {
		h = 1
	}
	return h
}

// visibleItems returns the children of the current directory, optionally
// filtered to only entries ≥100 MiB.
func (m AnalyzeModel) visibleItems() []*DirEntry {
	if m.current == nil {
		return nil
	}

	// Calculate current depth from root.
	var currentDepth int
	if m.maxDepth > 0 {
		currentDepth = m.currentDepth()
	}

	var out []*DirEntry
	for _, c := range m.current.Children {
		// Filter by minimum size.
		if m.minSize > 0 && c.Size < m.minSize {
			continue
		}
		// Filter by size threshold (L key toggle).
		if m.largeOnly && c.Size < 100*1024*1024 {
			continue
		}
		// Filter by depth: hide directory children beyond maxDepth.
		if m.maxDepth > 0 && c.IsDir && currentDepth >= m.maxDepth {
			continue
		}
		out = append(out, c)
	}
	return out
}

// removeEntry deletes an entry from the current Children slice and
// recalculates the parent size.
func (m *AnalyzeModel) removeEntry(path string) {
	if m.current == nil {
		return
	}
	for i, c := range m.current.Children {
		if c.Path == path {
			m.current.Children = append(m.current.Children[:i], m.current.Children[i+1:]...)
			// Recalculate current directory size.
			var total int64
			for _, child := range m.current.Children {
				total += child.Size
			}
			m.current.Size = total
			if m.cursor >= len(m.current.Children) && m.cursor > 0 {
				m.cursor--
			}
			return
		}
	}
}

// currentDepth returns how many levels deep the current directory is from root.
func (m AnalyzeModel) currentDepth() int {
	return len(m.breadcrumb)
}

// navigateToEntry builds a breadcrumb trail to the given entry's parent and
// sets the cursor to the entry. Used when selecting a search result.
func (m *AnalyzeModel) navigateToEntry(entry *DirEntry) {
	// Build breadcrumb from root to entry's parent
	var trail []*DirEntry
	current := entry.Parent
	for current != nil && current != m.root {
		trail = append([]*DirEntry{current}, trail...)
		current = current.Parent
	}

	m.breadcrumb = trail
	if entry.Parent != nil {
		m.current = entry.Parent
	} else {
		m.current = m.root
	}

	// Find entry index in parent's children
	m.cursor = 0
	for i, child := range m.current.Children {
		if child == entry {
			m.cursor = i
			break
		}
	}
	m.offset = 0
	m.ensureVisible()
}

// openInExplorer opens the parent folder of a path with the item selected.
func openInExplorer(path string) {
	if runtime.GOOS == "windows" {
		dir := filepath.Dir(path)
		_ = exec.Command("explorer", "/select,", dir).Start()
	}
}
