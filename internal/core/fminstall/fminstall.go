// Package fminstall finds Football Manager user-data folders
// (".../Sports Interactive/Football Manager 2024") on Windows, macOS and
// Linux (native, Steam Proton, Heroic/Epic) and installs the bundled view
// and filter files into them.
package fminstall

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"fmnewgenfaces/assets"
	"fmnewgenfaces/internal/core/fmversion"
)

// Install is one FM user-data directory.
type Install struct {
	// Name is the folder name, e.g. "Football Manager 2024".
	Name string
	// BasePath is ".../Sports Interactive/Football Manager 2024".
	BasePath string
	// GraphicsDir is BasePath/graphics (may not exist yet).
	GraphicsDir string
	Version     fmversion.Version
	// Source is where it was found: "documents", "onedrive", "proton", "heroic", "custom".
	Source string
}

// ViewsDir / FiltersDir are BasePath/views and BasePath/filters.
func (i Install) ViewsDir() string   { return filepath.Join(i.BasePath, "views") }
func (i Install) FiltersDir() string { return filepath.Join(i.BasePath, "filters") }

// sportsInteractiveDirName is the folder that always sits directly above an
// FM user-data folder.
const sportsInteractiveDirName = "Sports Interactive"

// fmDirsEnvVar lists extra base ("Sports Interactive") directories to scan,
// separated by os.PathListSeparator. It exists mainly to make Detect
// testable without touching the real filesystem layout.
const fmDirsEnvVar = "FM_NEWGEN_FACES_FM_DIRS"

// Roots returns the candidate "Sports Interactive" parent directories for the
// current OS that exist on disk. It never walks game libraries; Proton
// prefixes are probed by their known layout
// (compatdata/<appid>/pfx/drive_c/users/steamuser/Documents/Sports Interactive).
func Roots() []string {
	var candidates []string

	home, homeErr := os.UserHomeDir()

	switch runtime.GOOS {
	case "windows":
		if homeErr == nil && home != "" {
			candidates = append(candidates,
				filepath.Join(home, "Documents", sportsInteractiveDirName),
				filepath.Join(home, "OneDrive", "Documents", sportsInteractiveDirName),
			)
		}
		if oneDrive := os.Getenv("OneDrive"); oneDrive != "" {
			candidates = append(candidates, filepath.Join(oneDrive, "Documents", sportsInteractiveDirName))
		}
	case "darwin":
		if homeErr == nil && home != "" {
			candidates = append(candidates, filepath.Join(home, "Documents", sportsInteractiveDirName))
		}
	default: // linux and other unix-likes
		if homeErr == nil && home != "" {
			candidates = append(candidates, filepath.Join(home, "Documents", sportsInteractiveDirName))
			candidates = append(candidates, globCompatdata(filepath.Join(home, ".steam", "steam", "steamapps", "compatdata"))...)
			candidates = append(candidates, globCompatdata(filepath.Join(home, ".local", "share", "Steam", "steamapps", "compatdata"))...)
			candidates = append(candidates, globCompatdata(filepath.Join(home, ".steam", "debian-installation", "steamapps", "compatdata"))...)
			candidates = append(candidates, globHeroic(filepath.Join(home, "Games", "Heroic", "Prefixes", "default"))...)
		}
	}

	seen := make(map[string]bool, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		clean := filepath.Clean(c)
		if seen[clean] {
			continue
		}
		if info, err := os.Stat(clean); err == nil && info.IsDir() {
			seen[clean] = true
			out = append(out, clean)
		}
	}
	return out
}

func globCompatdata(compatdata string) []string {
	pattern := filepath.Join(compatdata, "*", "pfx", "drive_c", "users", "steamuser", "Documents", sportsInteractiveDirName)
	matches, _ := filepath.Glob(pattern)
	return matches
}

func globHeroic(prefixesDefault string) []string {
	pattern := filepath.Join(prefixesDefault, "*", "drive_c", "users", "*", "Documents", sportsInteractiveDirName)
	matches, _ := filepath.Glob(pattern)
	return matches
}

// sourceForRoot classifies a "Sports Interactive" root directory for the
// Source field.
func sourceForRoot(root string) string {
	lower := strings.ToLower(root)
	switch {
	case strings.Contains(lower, "onedrive"):
		return "onedrive"
	case strings.Contains(lower, "compatdata"):
		return "proton"
	case strings.Contains(lower, "heroic"):
		return "heroic"
	default:
		return "documents"
	}
}

// Detect lists installs under Roots(), deduplicated, newest version first.
// It is bounded (no recursive walks) and safe to call at startup.
func Detect() []Install {
	var found []Install
	for _, root := range Roots() {
		found = append(found, scanRoot(root, sourceForRoot(root))...)
	}

	if extra := os.Getenv(fmDirsEnvVar); extra != "" {
		for _, base := range strings.Split(extra, string(os.PathListSeparator)) {
			base = strings.TrimSpace(base)
			if base == "" {
				continue
			}
			found = append(found, scanRoot(base, "custom")...)
		}
	}

	dedup := make(map[string]Install, len(found))
	order := make([]string, 0, len(found))
	for _, inst := range found {
		key := filepath.Clean(inst.BasePath)
		if _, exists := dedup[key]; !exists {
			order = append(order, key)
		}
		dedup[key] = inst
	}

	out := make([]Install, 0, len(order))
	for _, key := range order {
		out = append(out, dedup[key])
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Version.Year > out[j].Version.Year
	})
	return out
}

// scanRoot lists the immediate children of root whose name contains
// "Football Manager" (case-insensitive) and builds an Install for each.
func scanRoot(root, source string) []Install {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []Install
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !strings.Contains(strings.ToLower(e.Name()), "football manager") {
			continue
		}
		basePath := filepath.Join(root, e.Name())
		ver, _ := fmversion.FromPath(basePath)
		out = append(out, Install{
			Name:        e.Name(),
			BasePath:    basePath,
			GraphicsDir: filepath.Join(basePath, "graphics"),
			Version:     ver,
			Source:      source,
		})
	}
	return out
}

// FromPath walks up from any path inside an FM user-data folder (e.g. a face
// pack under graphics/) and returns the Install.
func FromPath(p string) (Install, bool) {
	if p == "" {
		return Install{}, false
	}
	current := filepath.Clean(p)
	if abs, err := filepath.Abs(current); err == nil {
		current = abs
	}

	for {
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		if strings.EqualFold(filepath.Base(parent), sportsInteractiveDirName) {
			name := filepath.Base(current)
			_, looksLikeVersion := fmversion.FromPath(name)
			if strings.Contains(strings.ToLower(name), "football manager") || looksLikeVersion {
				ver, _ := fmversion.FromPath(current)
				return Install{
					Name:        name,
					BasePath:    current,
					GraphicsDir: filepath.Join(current, "graphics"),
					Version:     ver,
					Source:      sourceForRoot(parent),
				}, true
			}
		}
		current = parent
	}
	return Install{}, false
}

// DistributeResult reports what Distribute did.
type DistributeResult struct {
	Copied  []string // destination paths written
	Skipped []string // destinations already identical
}

// distributeSpecs pairs an embedded directory with its destination resolver.
type distributeSpec struct {
	embedDir string
	destDir  func(Install) string
}

var distributeSpecs = []distributeSpec{
	{embedDir: "views", destDir: Install.ViewsDir},
	{embedDir: "filters", destDir: Install.FiltersDir},
}

// Distribute writes the embedded views/*.fmf and filters/*.fmf into the
// install, creating the folders. Identical files are skipped, so it is safe
// to call on every start.
func Distribute(inst Install) (DistributeResult, error) {
	var result DistributeResult

	for _, spec := range distributeSpecs {
		destDir := spec.destDir(inst)
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return result, err
		}

		entries, err := assets.FS.ReadDir(spec.embedDir)
		if err != nil {
			return result, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".fmf") {
				continue
			}
			data, err := assets.FS.ReadFile(path.Join(spec.embedDir, e.Name()))
			if err != nil {
				return result, err
			}
			dest := filepath.Join(destDir, e.Name())
			if existing, err := os.ReadFile(dest); err == nil && bytes.Equal(existing, data) {
				result.Skipped = append(result.Skipped, dest)
				continue
			}
			if err := os.WriteFile(dest, data, 0o644); err != nil {
				return result, err
			}
			result.Copied = append(result.Copied, dest)
		}
	}

	return result, nil
}

// ViewName / FilterName are the names the user must pick inside FM.
const (
	ViewName   = "SCRIPT FACES player search"
	FilterName = "is newgen search filter"
)
