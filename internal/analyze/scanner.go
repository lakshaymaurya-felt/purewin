package analyze

import (
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
