package facepack

import (
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"fmnewgenfaces/internal/core/ethnic"
)

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestScan_AllGroupsPresentEvenWhenMissing(t *testing.T) {
	root := t.TempDir()
	// Only create African (lowercase, to test case-insensitivity).
	africanDir := filepath.Join(root, "african")
	mustMkdir(t, africanDir)
	mustWrite(t, filepath.Join(africanDir, "a.png"), "x")

	pack, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if len(pack.Folders) != len(ethnic.All) {
		t.Fatalf("expected %d folders, got %d", len(ethnic.All), len(pack.Folders))
	}
	for _, e := range ethnic.All {
		if _, ok := pack.Folders[e]; !ok {
			t.Errorf("missing Folder entry for %s", e)
		}
	}

	af := pack.Folders[ethnic.African]
	if !af.Exists {
		t.Fatalf("expected African folder to exist")
	}
	if af.Path != africanDir {
		t.Errorf("Path = %q, want real path %q", af.Path, africanDir)
	}
	if len(af.Images) != 1 || af.Images[0] != "a" {
		t.Errorf("Images = %v, want [a]", af.Images)
	}

	asian := pack.Folders[ethnic.Asian]
	if asian.Exists {
		t.Errorf("expected Asian folder to be missing")
	}
}

func TestScan_ImagesIgnoredAndNested(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Caucasian")
	mustMkdir(t, dir)

	mustWrite(t, filepath.Join(dir, "b.PNG"), "x")  // uppercase ext, counts
	mustWrite(t, filepath.Join(dir, "a.jpg"), "x")  // counts
	mustWrite(t, filepath.Join(dir, "c.jpeg"), "x") // counts
	mustWrite(t, filepath.Join(dir, ".DS_Store"), "x")
	mustWrite(t, filepath.Join(dir, "Thumbs.db"), "x")
	mustWrite(t, filepath.Join(dir, "readme.txt"), "x")
	mustMkdir(t, filepath.Join(dir, "subdir"))
	mustWrite(t, filepath.Join(dir, "subdir", "hidden.png"), "x")

	pack, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	f := pack.Folders[ethnic.Caucasian]
	wantImages := []string{"a", "b", "c"}
	if !sort.StringsAreSorted(f.Images) {
		t.Errorf("Images not sorted: %v", f.Images)
	}
	if len(f.Images) != len(wantImages) {
		t.Fatalf("Images = %v, want %v", f.Images, wantImages)
	}
	for i, w := range wantImages {
		if f.Images[i] != w {
			t.Errorf("Images[%d] = %q, want %q", i, f.Images[i], w)
		}
	}

	wantIgnored := map[string]bool{".DS_Store": true, "Thumbs.db": true, "readme.txt": true}
	if len(f.Ignored) != len(wantIgnored) {
		t.Fatalf("Ignored = %v, want 3 entries", f.Ignored)
	}
	for _, name := range f.Ignored {
		if !wantIgnored[name] {
			t.Errorf("unexpected ignored file %q", name)
		}
	}

	if f.NestedDirs != 1 {
		t.Errorf("NestedDirs = %d, want 1", f.NestedDirs)
	}
	// Images inside nested dirs must NOT be counted.
	for _, img := range f.Images {
		if img == "hidden" {
			t.Errorf("nested image should not be counted: %v", f.Images)
		}
	}
}

func TestPack_MissingEmptyIsCompleteWarningsCount(t *testing.T) {
	root := t.TempDir()

	// Create all 14 folders; make one empty, leave others with 1 image, and
	// one with a nested dir + ignored file (warnings, not missing).
	for _, e := range ethnic.All {
		dir := filepath.Join(root, string(e))
		mustMkdir(t, dir)
		if e == ethnic.Asian {
			continue // leave empty
		}
		mustWrite(t, filepath.Join(dir, "img1.png"), "x")
		if e == ethnic.Caucasian {
			mustMkdir(t, filepath.Join(dir, "nested"))
			mustWrite(t, filepath.Join(dir, "ignored.txt"), "x")
		}
	}

	pack, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	missing := pack.Missing()
	if len(missing) != 0 {
		t.Errorf("Missing() = %v, want none (all folders exist)", missing)
	}

	empty := pack.Empty()
	if len(empty) != 1 || empty[0] != ethnic.Asian {
		t.Errorf("Empty() = %v, want [Asian]", empty)
	}

	if pack.IsComplete() {
		t.Errorf("IsComplete() = true, want false (Asian is empty)")
	}

	if pack.Count(ethnic.African) != 1 {
		t.Errorf("Count(African) = %d, want 1", pack.Count(ethnic.African))
	}
	if pack.Count(ethnic.Asian) != 0 {
		t.Errorf("Count(Asian) = %d, want 0", pack.Count(ethnic.Asian))
	}

	warnings := pack.Warnings()
	if len(warnings) == 0 {
		t.Fatalf("expected warnings, got none")
	}
	var sawEmpty, sawNested, sawIgnored bool
	for _, w := range warnings {
		if containsAll(w, "Asian", "empty") {
			sawEmpty = true
		}
		if containsAll(w, "Caucasian", "nested") {
			sawNested = true
		}
		if containsAll(w, "Caucasian", "ignored") {
			sawIgnored = true
		}
		if containsAll(w, "missing") {
			t.Errorf("Warnings() must not report missing folders: %q", w)
		}
	}
	if !sawEmpty || !sawNested || !sawIgnored {
		t.Errorf("warnings missing expected entries: %v", warnings)
	}
}

func TestPack_MissingFoldersAreErrorsNotWarnings(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, string(ethnic.African)))
	mustWrite(t, filepath.Join(root, string(ethnic.African), "a.png"), "x")

	pack, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	missing := pack.Missing()
	if len(missing) != len(ethnic.All)-1 {
		t.Fatalf("Missing() len = %d, want %d", len(missing), len(ethnic.All)-1)
	}
	for _, w := range pack.Warnings() {
		if containsAll(w, "Asian") {
			t.Errorf("Warnings() must not mention missing folders: %q", w)
		}
	}
}

func TestScan_ErrorsOnUnreadableRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := Scan(root); err == nil {
		t.Fatalf("expected error scanning missing root")
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func buildTestPack(t *testing.T, images map[ethnic.Ethnic][]string) *Pack {
	t.Helper()
	root := t.TempDir()
	pack := &Pack{Root: root, Folders: make(map[ethnic.Ethnic]*Folder)}
	for _, e := range ethnic.All {
		imgs := images[e]
		f := &Folder{Ethnic: e, Path: filepath.Join(root, string(e))}
		if imgs != nil {
			f.Exists = true
			f.Images = append([]string(nil), imgs...)
			sort.Strings(f.Images)
		}
		pack.Folders[e] = f
		pack.TotalImages += len(f.Images)
	}
	return pack
}

func TestPoolDraw_EveryImageReachable(t *testing.T) {
	pack := buildTestPack(t, map[ethnic.Ethnic][]string{
		ethnic.African: {"a", "b", "c"},
	})
	pool := NewPool(pack, rand.New(rand.NewSource(1)))

	seen := make(map[string]bool)
	for i := 0; i < 500; i++ {
		img, err := pool.Draw(ethnic.African, false)
		if err != nil {
			t.Fatalf("Draw: %v", err)
		}
		seen[img] = true
	}
	for _, want := range []string{"a", "b", "c"} {
		if !seen[want] {
			t.Errorf("image %q was never drawn in 500 tries (v1 bug: len-1 never picks last)", want)
		}
	}
}

func TestPoolDraw_ConsumeRemovesImage(t *testing.T) {
	pack := buildTestPack(t, map[ethnic.Ethnic][]string{
		ethnic.African: {"a", "b"},
	})
	pool := NewPool(pack, rand.New(rand.NewSource(2)))

	if pool.Available(ethnic.African) != 2 {
		t.Fatalf("Available = %d, want 2", pool.Available(ethnic.African))
	}
	first, err := pool.Draw(ethnic.African, true)
	if err != nil {
		t.Fatalf("Draw: %v", err)
	}
	if pool.Available(ethnic.African) != 1 {
		t.Fatalf("Available after consume = %d, want 1", pool.Available(ethnic.African))
	}
	second, err := pool.Draw(ethnic.African, true)
	if err != nil {
		t.Fatalf("Draw: %v", err)
	}
	if first == second {
		t.Fatalf("expected distinct images, got %q twice", first)
	}
	if pool.Available(ethnic.African) != 0 {
		t.Fatalf("Available after consuming all = %d, want 0", pool.Available(ethnic.African))
	}
	if _, err := pool.Draw(ethnic.African, true); err == nil {
		t.Fatalf("expected ErrExhausted")
	} else if _, ok := err.(*ErrExhausted); !ok {
		t.Fatalf("expected *ErrExhausted, got %T", err)
	}
}

func TestPoolExclude(t *testing.T) {
	pack := buildTestPack(t, map[ethnic.Ethnic][]string{
		ethnic.African: {"a", "b", "c"},
		ethnic.Asian:   {"x", "y"},
	})
	pool := NewPool(pack, rand.New(rand.NewSource(3)))

	pool.Exclude([]ImageRef{
		{Ethnic: ethnic.African, Name: "b"},
		{Ethnic: ethnic.Asian, Name: "zzz"}, // unknown, ignored
	})

	if pool.Available(ethnic.African) != 2 {
		t.Fatalf("Available(African) = %d, want 2", pool.Available(ethnic.African))
	}
	if pool.Available(ethnic.Asian) != 2 {
		t.Fatalf("Available(Asian) = %d, want 2 (unknown exclude ignored)", pool.Available(ethnic.Asian))
	}
	for i := 0; i < 50; i++ {
		img, err := pool.Draw(ethnic.African, false)
		if err != nil {
			t.Fatalf("Draw: %v", err)
		}
		if img == "b" {
			t.Fatalf("excluded image %q was drawn", img)
		}
	}
}

func TestParseRef(t *testing.T) {
	cases := []struct {
		in      string
		wantRef ImageRef
		wantOK  bool
	}{
		{"faces/African/abc123", ImageRef{ethnic.African, "abc123"}, true},
		{"African/abc123", ImageRef{ethnic.African, "abc123"}, true},
		{`faces\African\abc123`, ImageRef{ethnic.African, "abc123"}, true},
		{"faces/african/abc123", ImageRef{ethnic.African, "abc123"}, true}, // case-insensitive
		{"faces/African/abc123.png", ImageRef{ethnic.African, "abc123"}, true},
		{"notethnic/abc123", ImageRef{}, false},
		{"abc123", ImageRef{}, false},
		{"", ImageRef{}, false},
	}
	for _, c := range cases {
		ref, ok := ParseRef(c.in)
		if ok != c.wantOK {
			t.Errorf("ParseRef(%q) ok = %v, want %v", c.in, ok, c.wantOK)
			continue
		}
		if ok && ref != c.wantRef {
			t.Errorf("ParseRef(%q) = %+v, want %+v", c.in, ref, c.wantRef)
		}
	}
}

func makePackFolders(t *testing.T, root string, names []string) {
	t.Helper()
	for _, n := range names {
		mustMkdir(t, filepath.Join(root, n))
	}
}

func TestLooksLikePack(t *testing.T) {
	root := t.TempDir()
	// 10 ethnic-looking folders (case varied) + 2 decoys.
	names := []string{
		"african", "Asian", "CAUCASIAN", "Central European", "EECA",
		"Italmed", "MENA", "MESA", "SAMed", "Scandinavian",
		"randomfolder", "another",
	}
	makePackFolders(t, root, names)

	if !LooksLikePack(root, 10) {
		t.Errorf("expected LooksLikePack(root, 10) = true")
	}
	if LooksLikePack(root, 11) {
		t.Errorf("expected LooksLikePack(root, 11) = false (only 10 ethnic folders)")
	}

	empty := t.TempDir()
	if LooksLikePack(empty, 10) {
		t.Errorf("expected LooksLikePack(empty, 10) = false")
	}

	if LooksLikePack(filepath.Join(root, "does-not-exist"), 1) {
		t.Errorf("expected LooksLikePack on missing dir = false")
	}
}

func fullPackFolderNames() []string {
	out := make([]string, len(ethnic.All))
	for i, e := range ethnic.All {
		out[i] = string(e)
	}
	return out
}

func TestFind_BoundedDepth(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	mustMkdir(t, nested)
	makePackFolders(t, nested, fullPackFolderNames())

	// root itself: depth 0
	// root/a: depth 1
	// root/a/b: depth 2 (the pack)
	if _, ok := Find(root, 1); ok {
		t.Errorf("Find with maxDepth=1 should not reach depth-2 pack")
	}
	got, ok := Find(root, 2)
	if !ok {
		t.Fatalf("Find with maxDepth=2 should find the pack")
	}
	if got != nested {
		t.Errorf("Find() = %q, want %q", got, nested)
	}
}

func TestFind_DoesNotFollowSymlinks(t *testing.T) {
	root := t.TempDir()

	// Real pack lives outside the search tree.
	realPack := t.TempDir()
	makePackFolders(t, realPack, fullPackFolderNames())

	link := filepath.Join(root, "link")
	if err := os.Symlink(realPack, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	if _, ok := Find(root, 5); ok {
		t.Errorf("Find must not follow symlinks into a pack directory")
	}
}

func TestFind_FindsPackAtRoot(t *testing.T) {
	root := t.TempDir()
	makePackFolders(t, root, fullPackFolderNames())

	got, ok := Find(root, 0)
	if !ok || got != root {
		t.Fatalf("Find(root,0) = (%q,%v), want (%q,true)", got, ok, root)
	}
}
