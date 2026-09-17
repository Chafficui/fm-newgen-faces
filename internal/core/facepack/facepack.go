// Package facepack scans a face pack directory (14 ethnic subfolders of
// portrait images) and hands out images to players.
package facepack

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fmnewgenfaces/internal/core/ethnic"
)

// ImageExtensions are accepted, case-insensitively. Anything else (Thumbs.db,
// .DS_Store, readme.txt, config.xml) is listed in Folder.Ignored.
var ImageExtensions = []string{".png", ".jpg", ".jpeg"}

// Folder is one ethnic subfolder.
type Folder struct {
	Ethnic ethnic.Ethnic
	Path   string
	Exists bool
	// Images are base names WITHOUT extension (what FM wants in config.xml), sorted.
	Images []string
	// Ignored are non-image file names that were skipped.
	Ignored []string
	// NestedDirs counts subdirectories (images inside them are NOT used; FM packs are flat).
	NestedDirs int
}

// Pack is the scan result. Folders always has an entry for every ethnic.All.
type Pack struct {
	Root        string
	Folders     map[ethnic.Ethnic]*Folder
	TotalImages int
}

// isImageExt reports whether ext (as returned by filepath.Ext, i.e. including
// the leading dot) is one of ImageExtensions, case-insensitively.
func isImageExt(ext string) bool {
	ext = strings.ToLower(ext)
	for _, e := range ImageExtensions {
		if ext == e {
			return true
		}
	}
	return false
}

func isImageFile(name string) bool {
	return isImageExt(filepath.Ext(name))
}

// Scan reads root. It returns an error only if root itself cannot be read;
// missing ethnic folders are reported via Folder.Exists=false.
func Scan(root string) (*Pack, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	// Case-insensitive lookup of subdirectory names.
	dirByLower := make(map[string]string, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			dirByLower[strings.ToLower(e.Name())] = e.Name()
		}
	}

	pack := &Pack{
		Root:    root,
		Folders: make(map[ethnic.Ethnic]*Folder, len(ethnic.All)),
	}

	for _, e := range ethnic.All {
		folder := &Folder{Ethnic: e}

		actualName, found := dirByLower[strings.ToLower(string(e))]
		if !found {
			folder.Path = filepath.Join(root, string(e))
			folder.Exists = false
			pack.Folders[e] = folder
			continue
		}

		folderPath := filepath.Join(root, actualName)
		folder.Path = folderPath
		folder.Exists = true

		subEntries, err := os.ReadDir(folderPath)
		if err != nil {
			// The folder exists but can't be listed (permissions, race).
			// Report it as an empty, existing folder rather than failing
			// the whole scan.
			pack.Folders[e] = folder
			continue
		}

		var images []string
		var ignored []string
		nested := 0
		for _, se := range subEntries {
			name := se.Name()
			if se.IsDir() {
				nested++
				continue
			}
			if isImageFile(name) {
				images = append(images, strings.TrimSuffix(name, filepath.Ext(name)))
			} else {
				ignored = append(ignored, name)
			}
		}
		sort.Strings(images)
		sort.Strings(ignored)

		folder.Images = images
		folder.Ignored = ignored
		folder.NestedDirs = nested

		pack.Folders[e] = folder
		pack.TotalImages += len(images)
	}

	return pack, nil
}

// Missing lists ethnic folders that do not exist, in ethnic.All order.
func (p *Pack) Missing() []ethnic.Ethnic {
	var out []ethnic.Ethnic
	for _, e := range ethnic.All {
		if f, ok := p.Folders[e]; !ok || !f.Exists {
			out = append(out, e)
		}
	}
	return out
}

// Empty lists folders that exist but hold zero images.
func (p *Pack) Empty() []ethnic.Ethnic {
	var out []ethnic.Ethnic
	for _, e := range ethnic.All {
		if f, ok := p.Folders[e]; ok && f.Exists && len(f.Images) == 0 {
			out = append(out, e)
		}
	}
	return out
}

// IsComplete is true when all 14 folders exist and each has at least one image.
func (p *Pack) IsComplete() bool {
	for _, e := range ethnic.All {
		f, ok := p.Folders[e]
		if !ok || !f.Exists || len(f.Images) == 0 {
			return false
		}
	}
	return true
}

// Warnings returns human-readable, non-fatal problems (nested dirs, ignored
// files, empty folders). Missing folders are NOT warnings; they are errors.
func (p *Pack) Warnings() []string {
	var out []string
	for _, e := range ethnic.All {
		f, ok := p.Folders[e]
		if !ok || !f.Exists {
			continue
		}
		if len(f.Images) == 0 {
			out = append(out, fmt.Sprintf("%s: folder is empty", e))
		}
		if f.NestedDirs > 0 {
			noun := "subdirectory"
			if f.NestedDirs != 1 {
				noun = "subdirectories"
			}
			out = append(out, fmt.Sprintf("%s: %d nested %s ignored", e, f.NestedDirs, noun))
		}
		if len(f.Ignored) > 0 {
			out = append(out, fmt.Sprintf("%s: %d non-image file(s) ignored (%s)", e, len(f.Ignored), strings.Join(f.Ignored, ", ")))
		}
	}
	return out
}

// Count returns the number of images for e (0 if missing).
func (p *Pack) Count(e ethnic.Ethnic) int {
	f, ok := p.Folders[e]
	if !ok || !f.Exists {
		return 0
	}
	return len(f.Images)
}

// LooksLikePack reports whether dir contains at least minFolders ethnic
// subfolders (case-insensitive names). Used for auto-detection.
func LooksLikePack(dir string, minFolders int) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, ok := ethnic.Parse(e.Name()); ok {
			count++
		}
	}
	return count >= minFolders
}

// Find searches dir and its subdirectories up to maxDepth for the first
// directory that LooksLikePack (10 folders). Used to prefill the pack path
// from an FM graphics directory.
func Find(dir string, maxDepth int) (string, bool) {
	return find(dir, maxDepth)
}

func find(dir string, depthLeft int) (string, bool) {
	if LooksLikePack(dir, 10) {
		return dir, true
	}
	if depthLeft <= 0 {
		return "", false
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}

	names := make([]string, 0, len(entries))
	byName := make(map[string]os.DirEntry, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
		byName[e.Name()] = e
	}
	sort.Strings(names)

	for _, name := range names {
		e := byName[name]
		// Skip anything that isn't a real, non-symlinked directory. A
		// DirEntry's Type() reflects Lstat information (it does not follow
		// symlinks), so a symlink-to-directory reports ModeSymlink here, not
		// ModeDir, and is correctly skipped.
		if e.Type()&os.ModeSymlink != 0 {
			continue
		}
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(dir, name)
		if found, ok := find(sub, depthLeft-1); ok {
			return found, true
		}
	}
	return "", false
}

// ImageRef identifies one image inside a pack.
type ImageRef struct {
	Ethnic ethnic.Ethnic
	Name   string // base name without extension
}

// ParseRef extracts an ImageRef from a config.xml "from" path such as
// "faces/African/abc123" or "African/abc123" (any slash style). ok is false
// when the second-to-last segment is not an ethnic folder name.
func ParseRef(from string) (ref ImageRef, ok bool) {
	clean := strings.ReplaceAll(from, "\\", "/")
	clean = strings.Trim(clean, "/")
	if clean == "" {
		return ImageRef{}, false
	}
	parts := strings.Split(clean, "/")
	if len(parts) < 2 {
		return ImageRef{}, false
	}

	name := parts[len(parts)-1]
	folder := parts[len(parts)-2]

	e, found := ethnic.Parse(folder)
	if !found {
		return ImageRef{}, false
	}

	if isImageExt(filepath.Ext(name)) {
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}

	return ImageRef{Ethnic: e, Name: name}, true
}

// Pool draws images per ethnic group.
type Pool struct {
	// unexported
	pool map[ethnic.Ethnic][]string
	rng  *rand.Rand
}

// NewPool copies image lists from p. rng may be nil (a time-seeded source is used).
func NewPool(p *Pack, rng *rand.Rand) *Pool {
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	pool := make(map[ethnic.Ethnic][]string, len(ethnic.All))
	for _, e := range ethnic.All {
		var images []string
		if f, ok := p.Folders[e]; ok && f.Exists && len(f.Images) > 0 {
			images = make([]string, len(f.Images))
			copy(images, f.Images)
		}
		pool[e] = images
	}
	return &Pool{pool: pool, rng: rng}
}

// Exclude removes already-used images so they cannot be drawn again
// (used with "no duplicates" + preserved mappings). Unknown refs are ignored.
func (pool *Pool) Exclude(used []ImageRef) {
	toRemove := make(map[ethnic.Ethnic]map[string]bool)
	for _, u := range used {
		set, ok := toRemove[u.Ethnic]
		if !ok {
			set = make(map[string]bool)
			toRemove[u.Ethnic] = set
		}
		set[u.Name] = true
	}
	for e, names := range toRemove {
		list, ok := pool.pool[e]
		if !ok || len(list) == 0 {
			continue
		}
		filtered := make([]string, 0, len(list))
		for _, img := range list {
			if !names[img] {
				filtered = append(filtered, img)
			}
		}
		pool.pool[e] = filtered
	}
}

// Available returns how many images remain for e.
func (pool *Pool) Available(e ethnic.Ethnic) int {
	return len(pool.pool[e])
}

// ErrExhausted is returned by Draw when no image is left for the group.
type ErrExhausted struct{ Ethnic ethnic.Ethnic }

func (e *ErrExhausted) Error() string { return "no images left for " + string(e.Ethnic) }

// Draw returns a uniformly random image for e (every index reachable). When
// consume is true the image is removed from the pool.
func (pool *Pool) Draw(e ethnic.Ethnic, consume bool) (string, error) {
	list := pool.pool[e]
	if len(list) == 0 {
		return "", &ErrExhausted{Ethnic: e}
	}
	idx := pool.rng.Intn(len(list))
	img := list[idx]

	if consume {
		newList := make([]string, 0, len(list)-1)
		newList = append(newList, list[:idx]...)
		newList = append(newList, list[idx+1:]...)
		pool.pool[e] = newList
	}

	return img, nil
}
