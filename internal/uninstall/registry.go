package uninstall

import (
	"regexp"
	"sort"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// InstalledApp represents an application found in the Windows registry.
type InstalledApp struct {
	Name                 string
	Version              string
	Publisher            string
	InstallDate          string
	EstimatedSize        int64
	UninstallString      string
	QuietUninstallString string
	InstallLocation      string
	BundleID             string
	IsSystemComponent    bool
}

// ─── Registry Sources ────────────────────────────────────────────────────────

// registrySource describes one registry hive + path to scan.
type registrySource struct {
	root registry.Key
	path string
}

// uninstallSources are the three standard locations for installed programs.
var uninstallSources = []registrySource{
	{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
}

// kbPattern matches Windows update identifiers like KB1234567.
var kbPattern = regexp.MustCompile(`(?i)\bKB\d{6,}\b`)

// ─── Public API ──────────────────────────────────────────────────────────────

// GetInstalledApps reads installed applications from the Windows registry.
// If showAll is true, system components and Windows updates are included.
func GetInstalledApps(showAll bool) ([]InstalledApp, error) {
	seen := make(map[string]bool)
	var apps []InstalledApp

	for _, src := range uninstallSources {
		found, err := readAppsFromKey(src.root, src.path)
		if err != nil {
			// Registry path may not exist (e.g., WOW6432Node on 32-bit);
			// skip silently.
			continue
		}

		for _, app := range found {
			// Deduplicate by name + version.
			key := strings.ToLower(app.Name + "|" + app.Version)
			if seen[key] {
				continue
			}
			seen[key] = true

			// Filter unless showAll is set.
			if !showAll {
				if app.Name == "" {
					continue
				}
				if app.IsSystemComponent {
					continue
				}
				if kbPattern.MatchString(app.Name) {
					continue
				}
			}

			apps = append(apps, app)
		}
	}

	// Sort by size descending — largest first.
	sort.Slice(apps, func(i, j int) bool {
		return apps[i].EstimatedSize > apps[j].EstimatedSize
	})

	return apps, nil
}

// ─── Registry Helpers ────────────────────────────────────────────────────────

// readAppsFromKey enumerates subkeys under the given registry path and
// reads application metadata from each.
func readAppsFromKey(root registry.Key, path string) ([]InstalledApp, error) {
	key, err := registry.OpenKey(root, path, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	defer key.Close()

	subkeys, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return nil, err
	}

	var apps []InstalledApp
	for _, name := range subkeys {
		app, readErr := readAppFromSubKey(root, path+`\`+name)
		if readErr != nil {
			continue
		}
		if app.Name == "" {
			continue
		}
		apps = append(apps, app)
	}

	return apps, nil
}

// readAppFromSubKey reads a single application's metadata from a registry key.
func readAppFromSubKey(root registry.Key, path string) (InstalledApp, error) {
	key, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return InstalledApp{}, err
	}
	defer key.Close()

	app := InstalledApp{
		Name:                 readStringValue(key, "DisplayName"),
		Version:              readStringValue(key, "DisplayVersion"),
		Publisher:            readStringValue(key, "Publisher"),
		InstallDate:          readStringValue(key, "InstallDate"),
		UninstallString:      readStringValue(key, "UninstallString"),
		QuietUninstallString: readStringValue(key, "QuietUninstallString"),
		InstallLocation:      readStringValue(key, "InstallLocation"),
		BundleID:             readStringValue(key, "BundleCachePath"),
	}

	// EstimatedSize is stored in KB as a DWORD.
	if size, _, sizeErr := key.GetIntegerValue("EstimatedSize"); sizeErr == nil {
		app.EstimatedSize = int64(size) * 1024 // Convert KB → bytes.
	}

	// SystemComponent is a DWORD (1 = system).
	if sc, _, scErr := key.GetIntegerValue("SystemComponent"); scErr == nil {
		app.IsSystemComponent = sc == 1
	}

	return app, nil
}

// readStringValue safely reads a string value from a registry key.
// Returns an empty string on any error.
func readStringValue(key registry.Key, name string) string {
	val, _, err := key.GetStringValue(name)
	if err != nil {
		return ""
	}
	return val
}
