package analyze

import (
	"container/heap"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// DirEntry represents a file or directory in the scan tree.
type DirEntry struct {
	Path     string      `json:"path"`
	Name     string      `json:"name"`
	Size     int64       `json:"size"`
	IsDir    bool        `json:"is_dir"`
	Children []*DirEntry `json:"children,omitempty"`
	Parent   *DirEntry   `json:"-"`
	ModTime  time.Time   `json:"mod_time"`
	Scanned  bool        `json:"scanned"`
}

// IsOld returns true if the entry hasn't been modified in 6+ months.
func (e *DirEntry) IsOld() bool {
	return time.Since(e.ModTime) > 180*24*time.Hour
}

// Percentage returns the entry's size as a percentage of its parent's size.
func (e *DirEntry) Percentage(parentSize int64) float64 {
	if parentSize == 0 {
		return 0
	}
	return float64(e.Size) / float64(parentSize) * 100
}

// Scanner performs parallel recursive directory scanning.
type Scanner struct {
	sem          chan struct{}
	exclude      map[string]bool
	mu           sync.Mutex
	warnings     []string
	scannedCount atomic.Int64
}

// NewScanner creates a scanner with bounded concurrency.
// exclude is a list of directory names (case-insensitive) to skip.
func NewScanner(maxConcurrency int, exclude []string) *Scanner {
	if maxConcurrency <= 0 {
		maxConcurrency = 8
	}
	excMap := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		excMap[strings.ToLower(e)] = true
	}
	return &Scanner{
		sem:     make(chan struct{}, maxConcurrency),
		exclude: excMap,
	}
}

// Warnings returns any warnings accumulated during scanning.
func (s *Scanner) Warnings() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.warnings...)
}

// ScannedCount returns the number of entries scanned so far.
func (s *Scanner) ScannedCount() int64 {
	return s.scannedCount.Load()
}

func (s *Scanner) addWarning(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.warnings) < 500 {
		s.warnings = append(s.warnings, msg)
	}
}

// isReparsePoint returns true if the path is a Windows junction or symlink
// (FILE_ATTRIBUTE_REPARSE_POINT). Must be checked to avoid infinite recursion.
func isReparsePoint(path string) bool {
	pathp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attrs, err := syscall.GetFileAttributes(pathp)
	if err != nil {
		return false
	}
	const fileAttributeReparsePoint = 0x0400
	return attrs&fileAttributeReparsePoint != 0
}

// longPath adds the \\?\ prefix for paths exceeding MAX_PATH on Windows.
func longPath(path string) string {
	if len(path) >= 260 && !strings.HasPrefix(path, `\\?\`) {
		return `\\?\` + filepath.Clean(path)
	}
	return path
}

// Scan performs a parallel recursive scan of the given root path.
func (s *Scanner) Scan(rootPath string) (*DirEntry, error) {
	rootPath = filepath.Clean(rootPath)

	info, err := os.Lstat(longPath(rootPath))
	if err != nil {
		return nil, err
	}

	root := &DirEntry{
		Path:    rootPath,
		Name:    info.Name(),
		IsDir:   info.IsDir(),
		ModTime: info.ModTime(),
	}

	if !info.IsDir() {
		root.Size = info.Size()
		root.Scanned = true
		return root, nil
	}

	s.scanDir(root)
	s.calculateSizes(root)
	root.Scanned = true

	return root, nil
}

// scanDir recursively scans a directory, using the semaphore only during I/O
// to prevent deadlocks from nested goroutine semaphore acquisition.
func (s *Scanner) scanDir(entry *DirEntry) {
	dirPath := longPath(entry.Path)

	// Hold semaphore only during the ReadDir I/O.
	s.sem <- struct{}{}
	entries, err := os.ReadDir(dirPath)
	<-s.sem

	if err != nil {
		s.addWarning("cannot read " + entry.Path + ": " + err.Error())
		return
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, e := range entries {
		childPath := filepath.Join(entry.Path, e.Name())
		s.scannedCount.Add(1)

		// Skip excluded directories.
		if e.IsDir() && s.exclude[strings.ToLower(e.Name())] {
			continue
		}

		// NEVER follow junction points / reparse points — infinite recursion risk.
		if e.IsDir() && isReparsePoint(childPath) {
			s.addWarning("skipping junction/reparse: " + childPath)
			continue
		}

		info, err := e.Info()
		if err != nil {
			// Permission denied or other error — skip, don't fail.
			s.addWarning("cannot stat " + childPath + ": " + err.Error())
			continue
		}

		child := &DirEntry{
			Path:    childPath,
			Name:    e.Name(),
			IsDir:   e.IsDir(),
			Parent:  entry,
			ModTime: info.ModTime(),
		}

		if !e.IsDir() {
			child.Size = info.Size()
			child.Scanned = true
		} else {
			wg.Add(1)
			go func(dir *DirEntry) {
				defer wg.Done()
				s.scanDir(dir)
				dir.Scanned = true
			}(child)
		}

		mu.Lock()
		entry.Children = append(entry.Children, child)
		mu.Unlock()
	}

	wg.Wait()
}

// calculateSizes walks the tree bottom-up, summing sizes from children,
// then sorts each level by size descending.
func (s *Scanner) calculateSizes(entry *DirEntry) {
	if !entry.IsDir {
		return
	}

	var total int64
	for _, child := range entry.Children {
		s.calculateSizes(child)
		total += child.Size
	}
	entry.Size = total

	// Sort children by size descending after all sizes are known.
	sort.Slice(entry.Children, func(i, j int) bool {
		return entry.Children[i].Size > entry.Children[j].Size
	})
}

// ─── Fuzzy Search ────────────────────────────────────────────────────────────

// fuzzyMatch returns true if all characters in pattern appear in str in order (case-insensitive).
// Also returns a score — higher is better (consecutive matches, start-of-word bonuses).
// Operates on runes to correctly handle multi-byte UTF-8 characters.
func fuzzyMatch(str, pattern string) (bool, int) {
	// Case-insensitive
	strRunes := []rune(strings.ToLower(str))
	patRunes := []rune(strings.ToLower(pattern))

	if len(patRunes) == 0 {
		return true, 0
	}
	if len(patRunes) > len(strRunes) {
		return false, 0
	}

	// Check if all pattern runes exist in order
	pIdx := 0
	score := 0
	prevMatch := false
	for i := 0; i < len(strRunes) && pIdx < len(patRunes); i++ {
		if strRunes[i] == patRunes[pIdx] {
			score += 1
			if prevMatch {
				score += 2 // consecutive bonus
			}
			if i == 0 || strRunes[i-1] == '/' || strRunes[i-1] == '\\' || strRunes[i-1] == '.' || strRunes[i-1] == '_' || strRunes[i-1] == '-' {
				score += 3 // start-of-word bonus
			}
			pIdx++
			prevMatch = true
		} else {
			prevMatch = false
		}
	}

	return pIdx == len(patRunes), score
}

// SearchResult holds a matched entry and its score.
type SearchResult struct {
	Entry *DirEntry
	Score int
}

// ─── Min-heap for bounded search results ────────────────────────────────────

// searchHeap implements a min-heap on SearchResult by (Score, Size).
// The smallest element is at the top so we can evict the worst result efficiently.
type searchHeap []SearchResult

func (h searchHeap) Len() int { return len(h) }

// Less: min-heap — lowest score first; tie-break by smallest size first.
func (h searchHeap) Less(i, j int) bool {
	if h[i].Score != h[j].Score {
		return h[i].Score < h[j].Score
	}
	return h[i].Entry.Size < h[j].Entry.Size
}

func (h searchHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *searchHeap) Push(x interface{}) {
	*h = append(*h, x.(SearchResult))
}

func (h *searchHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// SearchTreeBounded performs fuzzy search across the entire tree using a min-heap
// to keep memory bounded at O(maxResults). Returns matches sorted by score desc.
func SearchTreeBounded(root *DirEntry, query string, maxResults int) []SearchResult {
	if query == "" || root == nil || maxResults <= 0 {
		return nil
	}

	h := &searchHeap{}
	heap.Init(h)

	var search func(entry *DirEntry)
	search = func(entry *DirEntry) {
		if matched, score := fuzzyMatch(entry.Name, query); matched {
			if h.Len() < maxResults {
				heap.Push(h, SearchResult{Entry: entry, Score: score})
			} else if score > (*h)[0].Score || (score == (*h)[0].Score && entry.Size > (*h)[0].Entry.Size) {
				// Better than worst in heap — replace it.
				(*h)[0] = SearchResult{Entry: entry, Score: score}
				heap.Fix(h, 0)
			}
		}
		for _, child := range entry.Children {
			search(child)
		}
	}

	// Search children (not the root itself)
	for _, child := range root.Children {
		search(child)
	}

	// Drain heap into sorted slice (score desc, size desc).
	// Min-heap pops in ascending order; filling from the back yields descending.
	results := make([]SearchResult, h.Len())
	for i := len(results) - 1; i >= 0; i-- {
		results[i] = heap.Pop(h).(SearchResult)
	}

	return results
}

// SearchTree is a convenience alias for SearchTreeBounded.
func SearchTree(root *DirEntry, query string, maxResults int) []SearchResult {
	return SearchTreeBounded(root, query, maxResults)
}
