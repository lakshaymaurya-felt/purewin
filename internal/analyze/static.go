package analyze

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lakshaymaurya-felt/purewin/internal/core"
)

// PrintStaticTree prints a plain-text tree view of the disk analysis results.
// Used as a fallback when VT processing is unavailable and the interactive
// bubbletea TUI cannot render. Respects depth and minSize filters.
func PrintStaticTree(root *DirEntry, maxDepth int, minSize int64) {
	if root == nil {
		fmt.Println("  No data to display.")
		return
	}

	fmt.Printf("  Disk usage: %s\n", root.Path)
	fmt.Printf("  Total size: %s\n", core.FormatSize(root.Size))
	fmt.Println("  " + strings.Repeat("-", 58))
	fmt.Println()

	printEntry(root, "", true, 0, maxDepth, minSize)

	fmt.Println()
	fmt.Println("  " + strings.Repeat("-", 58))
	fmt.Printf("  Total: %s\n", core.FormatSize(root.Size))
}

// printEntry recursively prints a directory entry in tree format.
// Uses ASCII connectors (+-- \-- |) for maximum compatibility with
// all Windows consoles, including legacy code pages.
func printEntry(entry *DirEntry, prefix string, isLast bool, depth int, maxDepth int, minSize int64) {
	if entry == nil {
		return
	}

	// Apply depth limit (0 = unlimited).
	if maxDepth > 0 && depth > maxDepth {
		return
	}

	// Apply size filter.
	if minSize > 0 && entry.Size < minSize {
		return
	}

	// ASCII tree connector characters for maximum compatibility.
	connector := "+-- "
	childPrefix := "|   "
	if isLast {
		connector = "\\-- "
		childPrefix = "    "
	}

	// Root has no connector.
	if depth == 0 {
		connector = ""
		childPrefix = ""
	}

	// Format the line.
	sizeStr := core.FormatSize(entry.Size)
	dirMarker := ""
	if entry.IsDir {
		dirMarker = "/"
	}

	fmt.Printf("  %s%s%s%s  %s\n", prefix, connector, entry.Name, dirMarker, sizeStr)

	// Print children sorted by size (largest first).
	if entry.IsDir && len(entry.Children) > 0 {
		// Sort children by size descending.
		children := make([]*DirEntry, len(entry.Children))
		copy(children, entry.Children)
		sort.Slice(children, func(i, j int) bool {
			return children[i].Size > children[j].Size
		})

		// Limit to top 20 entries per level to keep output manageable.
		maxShow := 20
		if len(children) > maxShow {
			shown := children[:maxShow]
			for i, child := range shown {
				isChildLast := i == len(shown)-1
				printEntry(child, prefix+childPrefix, isChildLast, depth+1, maxDepth, minSize)
			}
			remaining := len(children) - maxShow
			fmt.Printf("  %s%s... and %d more entries\n", prefix+childPrefix, "\\-- ", remaining)
		} else {
			for i, child := range children {
				isChildLast := i == len(children)-1
				printEntry(child, prefix+childPrefix, isChildLast, depth+1, maxDepth, minSize)
			}
		}
	}
}
