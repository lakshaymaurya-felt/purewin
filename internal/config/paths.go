package config

import (
	"os"
	"path/filepath"

	"github.com/lakshaymaurya-felt/purewin/internal/envutil"
)

// CleanTarget represents a category of files that can be cleaned.
type CleanTarget struct {
	// Name is the unique identifier for this target.
	Name string

	// Paths is the list of filesystem paths to clean.
	Paths []string

	// Description is a human-readable description.
	Description string

	// RequiresAdmin indicates whether elevated privileges are needed.
	RequiresAdmin bool

	// Category groups related targets (e.g., "user", "system", "browser", "dev").
	Category string

	// RiskLevel is one of "low", "medium", "high".
	RiskLevel string
}

// expand resolves environment variables in a path, supporting both
// Windows %VAR% and Unix $VAR / ${VAR} syntax.
func expand(path string) string {
	return envutil.ExpandWindowsEnv(path)
}

// userProfile returns the user profile directory.
func userProfile() string {
	return os.Getenv("USERPROFILE")
}

// localAppData returns the local app data directory.
func localAppData() string {
	return os.Getenv("LOCALAPPDATA")
}

// appData returns the roaming app data directory.
func appData() string {
	return os.Getenv("APPDATA")
}

// winDir returns the Windows directory (e.g., C:\Windows).
// Falls back to C:\Windows only if %WINDIR% is not set.
func winDir() string {
	if w := os.Getenv("WINDIR"); w != "" {
		return w
	}
	return `C:\Windows`
}

// programData returns the ProgramData directory (e.g., C:\ProgramData).
// Falls back to C:\ProgramData only if %PROGRAMDATA% is not set.
func programData() string {
	if p := os.Getenv("PROGRAMDATA"); p != "" {
		return p
	}
	return `C:\ProgramData`
}

// systemDrive returns the system drive letter with backslash (e.g., C:\).
// Falls back to C:\ only if %SYSTEMDRIVE% is not set.
func systemDrive() string {
	if d := os.Getenv("SYSTEMDRIVE"); d != "" {
		return d + `\`
	}
	return `C:\`
}

// programFiles returns the Program Files directory.
func programFiles() string {
	if p := os.Getenv("PROGRAMFILES"); p != "" {
		return p
	}
	return `C:\Program Files`
}

// programFilesX86 returns the Program Files (x86) directory.
func programFilesX86() string {
	if p := os.Getenv("PROGRAMFILES(X86)"); p != "" {
		return p
	}
	return `C:\Program Files (x86)`
}

// GetCleanTargets returns all available cleanup targets with paths expanded.
func GetCleanTargets() []CleanTarget {
	home := userProfile()
	local := localAppData()
	roaming := appData()

	return []CleanTarget{
		// ── User Temp ───────────────────────────────────────────
		{
			Name:          "UserTemp",
			Paths:         []string{expand("$TEMP"), filepath.Join(local, "Temp")},
			Description:   "User temporary files",
			RequiresAdmin: false,
			Category:      "user",
			RiskLevel:     "low",
		},

		// ── System Temp ─────────────────────────────────────────
		{
			Name:          "SystemTemp",
			Paths:         []string{filepath.Join(winDir(), "Temp")},
			Description:   "System temporary files",
			RequiresAdmin: true,
			Category:      "system",
			RiskLevel:     "low",
		},

		// ── Browser Caches ──────────────────────────────────────
		{
			Name: "ChromeCache",
			Paths: []string{
				filepath.Join(local, "Google", "Chrome", "User Data", "Default", "Cache"),
				filepath.Join(local, "Google", "Chrome", "User Data", "Default", "Code Cache"),
				filepath.Join(local, "Google", "Chrome", "User Data", "Default", "GPUCache"),
				filepath.Join(local, "Google", "Chrome", "User Data", "Default", "Service Worker", "CacheStorage"),
			},
			Description:   "Google Chrome browser cache",
			RequiresAdmin: false,
			Category:      "browser",
			RiskLevel:     "low",
		},
		{
			Name: "EdgeCache",
			Paths: []string{
				filepath.Join(local, "Microsoft", "Edge", "User Data", "Default", "Cache"),
				filepath.Join(local, "Microsoft", "Edge", "User Data", "Default", "Code Cache"),
				filepath.Join(local, "Microsoft", "Edge", "User Data", "Default", "GPUCache"),
				filepath.Join(local, "Microsoft", "Edge", "User Data", "Default", "Service Worker", "CacheStorage"),
			},
			Description:   "Microsoft Edge browser cache",
			RequiresAdmin: false,
			Category:      "browser",
			RiskLevel:     "low",
		},
		{
			Name: "FirefoxCache",
			Paths: []string{
				filepath.Join(local, "Mozilla", "Firefox", "Profiles", "*", "cache2"),
				filepath.Join(local, "Mozilla", "Firefox", "Profiles", "*", "startupCache"),
				filepath.Join(local, "Mozilla", "Firefox", "Profiles", "*", "thumbnails"),
			},
			Description:   "Mozilla Firefox browser cache (cache2 within profiles)",
			RequiresAdmin: false,
			Category:      "browser",
			RiskLevel:     "low",
		},
		{
			Name: "BraveCache",
			Paths: []string{
				filepath.Join(local, "BraveSoftware", "Brave-Browser", "User Data", "Default", "Cache"),
				filepath.Join(local, "BraveSoftware", "Brave-Browser", "User Data", "Default", "Code Cache"),
				filepath.Join(local, "BraveSoftware", "Brave-Browser", "User Data", "Default", "GPUCache"),
			},
			Description:   "Brave browser cache",
			RequiresAdmin: false,
			Category:      "browser",
			RiskLevel:     "low",
		},

		// ── Developer Caches ────────────────────────────────────
		{
			Name:          "NpmCache",
			Paths:         []string{filepath.Join(roaming, "npm-cache")},
			Description:   "npm package manager cache",
			RequiresAdmin: false,
			Category:      "dev",
			RiskLevel:     "low",
		},
		{
			Name:          "PipCache",
			Paths:         []string{filepath.Join(local, "pip", "Cache")},
			Description:   "Python pip package cache",
			RequiresAdmin: false,
			Category:      "dev",
			RiskLevel:     "low",
		},
		{
			Name:          "CargoCache",
			Paths:         []string{filepath.Join(home, ".cargo", "registry", "cache")},
			Description:   "Rust cargo registry cache",
			RequiresAdmin: false,
			Category:      "dev",
			RiskLevel:     "low",
		},
		{
			Name:          "GradleCache",
			Paths:         []string{filepath.Join(home, ".gradle", "caches")},
			Description:   "Gradle build cache",
			RequiresAdmin: false,
			Category:      "dev",
			RiskLevel:     "low",
		},
		{
			Name:          "NuGetCache",
			Paths:         []string{filepath.Join(home, ".nuget", "packages")},
			Description:   "NuGet package cache",
			RequiresAdmin: false,
			Category:      "dev",
			RiskLevel:     "medium",
		},
		{
			Name: "GoModCache",
			Paths: []string{
				filepath.Join(home, "go", "pkg", "mod", "cache"),
			},
			Description:   "Go module download cache",
			RequiresAdmin: false,
			Category:      "dev",
			RiskLevel:     "low",
		},

		// ── IDE Caches ──────────────────────────────────────────
		{
			Name: "VSCodeCache",
			Paths: []string{
				filepath.Join(roaming, "Code", "Cache"),
				filepath.Join(roaming, "Code", "CachedData"),
				filepath.Join(roaming, "Code", "CachedExtensions"),
				filepath.Join(roaming, "Code", "CachedExtensionVSIXs"),
				filepath.Join(roaming, "Code", "logs"),
			},
			Description:   "Visual Studio Code cache and logs",
			RequiresAdmin: false,
			Category:      "dev",
			RiskLevel:     "low",
		},
		{
			Name: "JetBrainsCache",
			Paths: []string{
				filepath.Join(local, "JetBrains", "*", "caches"),
				filepath.Join(local, "JetBrains", "*", "log"),
				filepath.Join(local, "JetBrains", "*", "tmp"),
			},
			Description:   "JetBrains IDE caches (IntelliJ, GoLand, etc.)",
			RequiresAdmin: false,
			Category:      "dev",
			RiskLevel:     "medium",
		},
		{
			Name: "VisualStudioCache",
			Paths: []string{
				filepath.Join(local, "Microsoft", "VisualStudio", "Packages"),
				filepath.Join(local, "Microsoft", "VisualStudio", "ComponentModelCache"),
			},
			Description:   "Visual Studio component and package cache",
			RequiresAdmin: false,
			Category:      "dev",
			RiskLevel:     "medium",
		},

		// ── System Caches ───────────────────────────────────────
		{
			Name:          "WindowsUpdateCache",
			Paths:         []string{filepath.Join(winDir(), "SoftwareDistribution", "Download")},
			Description:   "Windows Update download cache",
			RequiresAdmin: true,
			Category:      "system",
			RiskLevel:     "medium",
		},
		{
			Name:          "CBSLogs",
			Paths:         []string{filepath.Join(winDir(), "Logs", "CBS")},
			Description:   "Component-Based Servicing logs",
			RequiresAdmin: true,
			Category:      "system",
			RiskLevel:     "low",
		},
		{
			Name:          "DISMLogs",
			Paths:         []string{filepath.Join(winDir(), "Logs", "DISM")},
			Description:   "DISM operation logs",
			RequiresAdmin: true,
			Category:      "system",
			RiskLevel:     "low",
		},
		{
			Name: "WERReports",
			Paths: []string{
				filepath.Join(local, "Microsoft", "Windows", "WER", "ReportArchive"),
				filepath.Join(local, "Microsoft", "Windows", "WER", "ReportQueue"),
				filepath.Join(programData(), "Microsoft", "Windows", "WER", "ReportArchive"),
				filepath.Join(programData(), "Microsoft", "Windows", "WER", "ReportQueue"),
			},
			Description:   "Windows Error Reporting crash dumps and reports",
			RequiresAdmin: false,
			Category:      "system",
			RiskLevel:     "low",
		},
		{
			Name:          "DeliveryOptimization",
			Paths:         []string{filepath.Join(winDir(), "SoftwareDistribution", "DeliveryOptimization")},
			Description:   "Delivery Optimization peer-to-peer update cache",
			RequiresAdmin: true,
			Category:      "system",
			RiskLevel:     "low",
		},
		{
			Name:          "FontCache",
			Paths:         []string{filepath.Join(winDir(), "ServiceProfiles", "LocalService", "AppData", "Local", "FontCache")},
			Description:   "Windows font cache (rebuilds automatically)",
			RequiresAdmin: true,
			Category:      "system",
			RiskLevel:     "medium",
		},

		// ── Thumbnails ──────────────────────────────────────────
		{
			Name: "Thumbnails",
			Paths: []string{
				filepath.Join(local, "Microsoft", "Windows", "Explorer"),
			},
			Description:   "Windows Explorer thumbnail cache (thumbcache_*.db)",
			RequiresAdmin: false,
			Category:      "user",
			RiskLevel:     "low",
		},

		// ── Memory Dumps ────────────────────────────────────────
		{
			Name: "MemoryDumps",
			Paths: []string{
				filepath.Join(winDir(), "MEMORY.DMP"),
				filepath.Join(winDir(), "Minidump"),
			},
			Description:   "Kernel and minidump crash files",
			RequiresAdmin: true,
			Category:      "system",
			RiskLevel:     "low",
		},

		// ── Windows.old ─────────────────────────────────────────
		{
			Name:          "WindowsOld",
			Paths:         []string{filepath.Join(systemDrive(), "Windows.old")},
			Description:   "Previous Windows installation (requires extra confirmation)",
			RequiresAdmin: true,
			Category:      "system",
			RiskLevel:     "high",
		},

		// ── Recycle Bin ─────────────────────────────────────────
		{
			Name:          "RecycleBin",
			Paths:         []string{}, // Cleaned via Shell API, no direct path
			Description:   "Windows Recycle Bin (emptied via system API)",
			RequiresAdmin: false,
			Category:      "user",
			RiskLevel:     "medium",
		},
	}
}

// GetTargetsByCategory returns clean targets filtered by category.
func GetTargetsByCategory(category string) []CleanTarget {
	var result []CleanTarget
	for _, t := range GetCleanTargets() {
		if t.Category == category {
			result = append(result, t)
		}
	}
	return result
}

// GetNeverDeletePaths returns paths that must NEVER be deleted under any
// circumstances. This list uses environment variables to support Windows
// installations on any drive letter (not just C:).
func GetNeverDeletePaths() []string {
	w := winDir()
	sd := systemDrive()
	return []string{
		w,
		filepath.Join(w, "System32"),
		filepath.Join(w, "SysWOW64"),
		filepath.Join(w, "WinSxS"),
		filepath.Join(w, "assembly"),
		filepath.Join(w, "System32", "config"),
		filepath.Join(sd, "Boot"),
		filepath.Join(sd, "bootmgr"),
		filepath.Join(sd, "EFI"),
		programFiles(),
		programFilesX86(),
		filepath.Join(sd, "Users"),
		programData(),
		filepath.Join(sd, "Recovery"),
		filepath.Join(w, "Installer"),
		filepath.Join(w, "servicing"),
		filepath.Join(w, "Prefetch"),
	}
}
