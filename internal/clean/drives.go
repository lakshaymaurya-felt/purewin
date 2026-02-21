package clean

import (
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/lakshaymaurya-felt/purewin/pkg/whitelist"
)

// ─── Multi-Drive Scanning ────────────────────────────────────────────────────

// driveLetters returns all mounted drive letters (e.g., "C:", "D:", "E:")
// by probing A-Z. Skips the system drive since it's already covered by
// the standard cleanup targets.
func nonSystemDrives() []string {
	sysDrive := strings.ToUpper(os.Getenv("SYSTEMDRIVE"))
	if sysDrive == "" {
		sysDrive = "C:"
	}

	var drives []string
	for c := 'A'; c <= 'Z'; c++ {
		drive := string(c) + ":"
		if strings.EqualFold(drive, sysDrive) {
			continue // Skip system drive — already scanned by standard targets.
		}

		// Check if the drive root exists and is accessible.
		root := drive + `\`
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}

		drives = append(drives, drive)
	}

	return drives
}

// commonTempDirs are directory names commonly used for temporary files
// on secondary drives. These are safe to clean.
var commonTempDirs = []string{
	"Temp",
	"tmp",
	"temp",
	"$RECYCLE.BIN", // per-drive recycle bin (system-managed, files inside are safe)
}

// commonJunkPatterns are file glob patterns for junk files found on any drive.
var commonJunkPatterns = []string{
	"*.tmp",
	"*.temp",
	"~$*",       // Office temp files
	"Thumbs.db", // Windows thumbnail cache
	"desktop.ini",
}

// ScanNonSystemDrives discovers all non-system drives and scans them for
// temp files, junk files, and common cache directories.
func ScanNonSystemDrives(wl *whitelist.Whitelist) []CleanItem {
	drives := nonSystemDrives()
	if len(drives) == 0 {
		return nil
	}

	var items []CleanItem

	for _, drive := range drives {
		root := drive + `\`
		driveLetter := drive[:1]

		// 1. Scan common temp directories on this drive.
		for _, tempDir := range commonTempDirs {
			dir := filepath.Join(root, tempDir)
			info, err := os.Stat(dir)
			if err != nil || !info.IsDir() {
				continue
			}

			// Skip $RECYCLE.BIN — it's handled by the Shell API already.
			if strings.EqualFold(tempDir, "$RECYCLE.BIN") {
				continue
			}

			if wl != nil && wl.IsWhitelisted(dir) {
				continue
			}

			dirItems := scanDirectory(dir, "user", driveLetter+": Temp files", wl)
			items = append(items, dirItems...)
		}

		// 2. Scan root-level junk files on this drive.
		for _, pattern := range commonJunkPatterns {
			matches, err := filepath.Glob(filepath.Join(root, pattern))
			if err != nil {
				continue
			}
			for _, match := range matches {
				if wl != nil && wl.IsWhitelisted(match) {
					continue
				}
				info, err := os.Stat(match)
				if err != nil || info.IsDir() {
					continue
				}
				items = append(items, CleanItem{
					Path:        match,
					Size:        info.Size(),
					Category:    "user",
					Description: driveLetter + ": Junk files",
				})
			}
		}

		// 3. Scan Windows.old on non-system drives (rare but possible).
		winOld := filepath.Join(root, "Windows.old")
		if info, err := os.Stat(winOld); err == nil && info.IsDir() {
			dirItems := scanDirectory(winOld, "system", driveLetter+": Windows.old", wl)
			items = append(items, dirItems...)
		}

		// 4. Scan nested temp directories (common on data drives).
		//    e.g., D:\Users\*\AppData\Local\Temp
		userDirs := filepath.Join(root, "Users")
		if info, err := os.Stat(userDirs); err == nil && info.IsDir() {
			// Find all user temp directories on this drive.
			tempPattern := filepath.Join(userDirs, "*", "AppData", "Local", "Temp")
			matches, err := filepath.Glob(tempPattern)
			if err == nil {
				for _, tempDir := range matches {
					if wl != nil && wl.IsWhitelisted(tempDir) {
						continue
					}
					dirItems := scanDirectory(tempDir, "user", driveLetter+": User temp", wl)
					items = append(items, dirItems...)
				}
			}
		}
	}

	return items
}

// ScanDriveJunkFiles scans a specific drive for common junk files
// recursively in the top 2 directory levels (not deep — too slow).
func ScanDriveJunkFiles(drive string, wl *whitelist.Whitelist) []CleanItem {
	root := drive + `\`
	driveLetter := drive[:1]

	if !utf8.ValidString(driveLetter) {
		return nil
	}

	var items []CleanItem

	// Only scan top-level entries to avoid very slow deep scans.
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		name := entry.Name()

		// Skip system/protected directories.
		lower := strings.ToLower(name)
		if lower == "windows" || lower == "program files" ||
			lower == "program files (x86)" || lower == "programdata" ||
			lower == "$recycle.bin" || lower == "system volume information" ||
			lower == "recovery" || lower == "boot" || lower == "efi" {
			continue
		}

		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(root, name)

		// Check for temp/cache subdirectories inside top-level folders.
		for _, sub := range []string{"Temp", "temp", "tmp", "cache", "Cache"} {
			subPath := filepath.Join(dirPath, sub)
			if info, err := os.Stat(subPath); err == nil && info.IsDir() {
				if wl != nil && wl.IsWhitelisted(subPath) {
					continue
				}
				dirItems := scanDirectory(subPath, "user", driveLetter+": "+name+" temp", wl)
				items = append(items, dirItems...)
			}
		}
	}

	return items
}
