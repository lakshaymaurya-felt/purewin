package clean

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// ─── Shell32 Syscalls ────────────────────────────────────────────────────────

var (
	modShell32          = syscall.NewLazyDLL("shell32.dll")
	procEmptyRecycleBin = modShell32.NewProc("SHEmptyRecycleBinW")
	procQueryRecycleBin = modShell32.NewProc("SHQueryRecycleBinW")
)

const (
	sherbNoConfirmation = 0x00000001
	sherbNoProgressUI   = 0x00000002
	sherbNoSound        = 0x00000004
)

// shQueryRBInfo mirrors the Windows SHQUERYRBINFO struct.
// Go's natural alignment adds padding after cbSize on AMD64,
// matching the C struct layout on both 32-bit and 64-bit.
type shQueryRBInfo struct {
	cbSize      uint32
	i64Size     int64
	i64NumItems int64
}

// ─── User Cache Scanning ─────────────────────────────────────────────────────

// ScanUserCaches scans user temporary file directories (%TEMP% and
// %LOCALAPPDATA%\Temp), deduplicating if they resolve to the same path.
func ScanUserCaches() []CleanItem {
	dirs := []string{
		os.ExpandEnv("$TEMP"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Temp"),
	}

	// Deduplicate — %TEMP% often points to %LOCALAPPDATA%\Temp.
	seen := make(map[string]bool)
	var unique []string
	for _, d := range dirs {
		cleaned := filepath.Clean(d)
		key := strings.ToLower(cleaned)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, cleaned)
		}
	}

	var items []CleanItem
	for _, dir := range unique {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		dirItems := scanDirectory(dir, "user", "User temporary files", nil)
		items = append(items, dirItems...)
	}

	return items
}

// ScanThumbnailCache scans for Windows Explorer thumbnail cache files
// (thumbcache_*.db) in the Explorer directory.
func ScanThumbnailCache() []CleanItem {
	explorerDir := filepath.Join(os.Getenv("LOCALAPPDATA"),
		"Microsoft", "Windows", "Explorer")

	pattern := filepath.Join(explorerDir, "thumbcache_*.db")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil
	}

	var items []CleanItem
	for _, path := range matches {
		info, statErr := os.Stat(path)
		if statErr != nil {
			continue
		}
		items = append(items, CleanItem{
			Path:        path,
			Size:        info.Size(),
			Category:    "user",
			Description: "Thumbnail cache",
		})
	}

	return items
}

// ─── Recycle Bin ──────────────────────────────────────────────────────────────

// ScanRecycleBin calculates the total size of items in the Windows Recycle
// Bin across all drives using the SHQueryRecycleBinW Shell API.
func ScanRecycleBin() (int64, error) {
	var info shQueryRBInfo
	info.cbSize = uint32(unsafe.Sizeof(info))

	ret, _, _ := procQueryRecycleBin.Call(
		0, // NULL = query all drives
		uintptr(unsafe.Pointer(&info)),
	)
	if ret != 0 {
		return 0, fmt.Errorf("SHQueryRecycleBinW failed: HRESULT 0x%08x", uint32(ret))
	}

	return info.i64Size, nil
}

// EmptyRecycleBin empties the Windows Recycle Bin on all drives via the
// SHEmptyRecycleBinW Shell API. In dryRun mode, no action is taken.
func EmptyRecycleBin(dryRun bool) error {
	if dryRun {
		return nil
	}

	flags := uintptr(sherbNoConfirmation | sherbNoProgressUI | sherbNoSound)
	ret, _, _ := procEmptyRecycleBin.Call(0, 0, flags)

	hr := uint32(ret)
	// S_OK (0) = success, E_UNEXPECTED (0x8000FFFF) = bin already empty.
	if hr != 0 && hr != 0x8000FFFF {
		return fmt.Errorf("SHEmptyRecycleBinW failed: HRESULT 0x%08x", hr)
	}

	return nil
}
