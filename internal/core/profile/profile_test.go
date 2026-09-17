package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultSettingsFreshMap(t *testing.T) {
	a := DefaultSettings()
	b := DefaultSettings()
	a.Overrides["FRA"] = "x"
	if _, ok := b.Overrides["FRA"]; ok {
		t.Fatal("DefaultSettings() Overrides map is shared across calls")
	}
	if !a.Preserve || !a.AllowDuplicates {
		t.Fatal("DefaultSettings() should default Preserve and AllowDuplicates to true")
	}
	if a.FMVersion == "" {
		t.Fatal("DefaultSettings() FMVersion should default to the newest known version")
	}
}

func TestValidateName(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{"  My Profile  ", false},
		{"", true},
		{"   ", true},
		{string(make([]rune, 65)), true}, // too long (zero runes, still 65 long)
		{"ok\x00name", true},
	}
	for _, c := range cases {
		_, err := ValidateName(c.name)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidateName(%q) err = %v, wantErr %v", c.name, err, c.wantErr)
		}
	}

	trimmed, err := ValidateName("  Hello  ")
	if err != nil || trimmed != "Hello" {
		t.Errorf("ValidateName trim: got %q, %v", trimmed, err)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"My Profile":      "my-profile",
		"  Über Cool!!  ": "ber-cool",
		"":                "profile",
		"already-slug":    "already-slug",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCreateAndSlugCollisions(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	p1, err := Create(s, "My Profile", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p1.Slug != "my-profile" {
		t.Errorf("slug = %q, want my-profile", p1.Slug)
	}

	p2, err := Create(s, "My  Profile!!", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p2.Slug != "my-profile-2" {
		t.Errorf("slug = %q, want my-profile-2", p2.Slug)
	}

	p3, err := Create(s, "my profile", "", nil)
	if err == nil {
		t.Fatalf("expected duplicate-name error, got profile %+v", p3)
	}
	var nameErr *NameError
	if _, ok := err.(*NameError); !ok {
		_ = nameErr
		t.Errorf("expected *NameError, got %T: %v", err, err)
	}
}

func TestCreateWithBaseSettings(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	base := DefaultSettings()
	base.PackDir = "/tmp/pack"
	base.Overrides["FRA"] = "eu"

	p, err := Create(s, "Based", "", &base)
	if err != nil {
		t.Fatal(err)
	}
	if p.Settings.PackDir != "/tmp/pack" {
		t.Errorf("PackDir = %q, want /tmp/pack", p.Settings.PackDir)
	}
	// mutating the returned profile must not affect the store's copy
	p.Settings.Overrides["ESP"] = "changed"
	stored, _ := s.Get(p.Slug)
	if _, ok := stored.Settings.Overrides["ESP"]; ok {
		t.Fatal("Create/Get did not clone Overrides map")
	}
}

func TestSaveIsAtomicAndPersists(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Create(s, "Persisted", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Re-open the store from disk and check the profile round-trips.
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s2.Get(p.Slug)
	if !ok {
		t.Fatal("profile not found after reopening store")
	}
	if got.Name != "Persisted" {
		t.Errorf("Name = %q, want Persisted", got.Name)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == "" && e.Name() != ".migrated" {
			t.Errorf("leftover temp file in store dir: %s", e.Name())
		}
	}
}

func TestRenameAndDelete(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Create(s, "Old Name", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Rename(p.Slug, "New Name"); err != nil {
		t.Fatal(err)
	}
	got, ok := s.Get(p.Slug)
	if !ok || got.Name != "New Name" {
		t.Fatalf("Rename did not persist: %+v ok=%v", got, ok)
	}
	if got.Slug != p.Slug {
		t.Errorf("slug changed on rename: %q -> %q", p.Slug, got.Slug)
	}

	if err := s.Delete(p.Slug); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get(p.Slug); ok {
		t.Fatal("profile still present after Delete")
	}
	if _, err := os.Stat(filepath.Join(dir, p.Slug+".json")); !os.IsNotExist(err) {
		t.Fatal("profile file still exists on disk after Delete")
	}
}

func TestRenameToDuplicateFails(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Create(s, "Alpha", "", nil); err != nil {
		t.Fatal(err)
	}
	beta, err := Create(s, "Beta", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Rename(beta.Slug, "alpha"); err == nil {
		t.Fatal("expected error renaming to a case-insensitive duplicate")
	}
}

func TestListSortedAndCloned(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Create(s, "Zeta", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(s, "Alpha", "", nil); err != nil {
		t.Fatal(err)
	}
	list := s.List()
	if len(list) != 2 || list[0].Name != "Alpha" || list[1].Name != "Zeta" {
		t.Fatalf("List() not sorted: %+v", list)
	}
	list[0].Name = "Mutated"
	list2 := s.List()
	if list2[0].Name == "Mutated" {
		t.Fatal("List() did not return clones")
	}
}

func TestFindByGamePath(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	gamePath := filepath.Join(dir, "FM24")
	if _, err := Create(s, "Bound", gamePath, nil); err != nil {
		t.Fatal(err)
	}
	got, ok := s.FindByGamePath(gamePath)
	if !ok || got.Name != "Bound" {
		t.Fatalf("FindByGamePath failed: %+v ok=%v", got, ok)
	}
	if _, ok := s.FindByGamePath(filepath.Join(dir, "Nope")); ok {
		t.Fatal("FindByGamePath matched an unrelated path")
	}
}

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	st := s.LoadState()
	if st.LastProfile != "" || st.Theme != "" {
		t.Fatalf("expected zero State, got %+v", st)
	}

	st.LastProfile = "my-profile"
	st.Theme = "dark"
	st.WindowWidth = 1024
	if err := s.SaveState(st); err != nil {
		t.Fatal(err)
	}

	got := s.LoadState()
	if got.LastProfile != "my-profile" || got.Theme != "dark" || got.WindowWidth != 1024 {
		t.Fatalf("state did not round-trip: %+v", got)
	}
}

func TestMigrateLegacy(t *testing.T) {
	legacyDir := t.TempDir()
	legacyProfilesDir := filepath.Join(legacyDir, "profiles")
	if err := os.MkdirAll(legacyProfilesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	full := `{
		"name": "Full Profile",
		"game_path": "/games/fm24",
		"config": {
			"preserve": false,
			"xml_path": "/games/fm24/config.xml",
			"rtf_path": "/games/fm24/newgen.rtf",
			"img_path": "/games/fm24/graphics",
			"fm_version": "2024",
			"allow_duplicate": false,
			"mapping_override": {"fra": "european"}
		},
		"created_at": "2023-01-01T00:00:00Z",
		"updated_at": "2023-02-01T00:00:00Z"
	}`
	sparse := `{
		"name": "Sparse Profile",
		"game_path": "",
		"config": {},
		"created_at": "2023-01-01T00:00:00Z",
		"updated_at": "2023-01-01T00:00:00Z"
	}`
	if err := os.WriteFile(filepath.Join(legacyProfilesDir, "full.json"), []byte(full), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyProfilesDir, "sparse.json"), []byte(sparse), 0o644); err != nil {
		t.Fatal(err)
	}

	toDir := t.TempDir()
	n, err := MigrateLegacy(legacyDir, toDir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("migrated %d profiles, want 2", n)
	}

	s, err := Open(toDir)
	if err != nil {
		t.Fatal(err)
	}
	// Open() would try migrating again, but the marker written by Open (not
	// present yet since we called MigrateLegacy directly) means a second
	// migration pass runs; that's fine, it must be idempotent w.r.t. slugs.
	list := s.List()
	names := map[string]*Profile{}
	for _, p := range list {
		names[p.Name] = p
	}

	fp, ok := names["Full Profile"]
	if !ok {
		t.Fatalf("Full Profile not migrated, got %+v", list)
	}
	if fp.GamePath != "/games/fm24" {
		t.Errorf("GamePath = %q", fp.GamePath)
	}
	if fp.Settings.Preserve {
		t.Error("Preserve should be false (migrated from legacy)")
	}
	if fp.Settings.AllowDuplicates {
		t.Error("AllowDuplicates should be false (migrated from legacy)")
	}
	if fp.Settings.FMVersion != "2024" {
		t.Errorf("FMVersion = %q, want 2024", fp.Settings.FMVersion)
	}
	if fp.Settings.PackDir != "/games/fm24/graphics" {
		t.Errorf("PackDir = %q", fp.Settings.PackDir)
	}
	if got := fp.Settings.Overrides["FRA"]; got != "european" {
		t.Errorf("Overrides[FRA] = %q, want european (upper-cased key)", got)
	}

	sp, ok := names["Sparse Profile"]
	if !ok {
		t.Fatalf("Sparse Profile not migrated, got %+v", list)
	}
	def := DefaultSettings()
	if sp.Settings.Preserve != def.Preserve || sp.Settings.AllowDuplicates != def.AllowDuplicates {
		t.Errorf("sparse profile should fall back to DefaultSettings: %+v", sp.Settings)
	}
	if sp.Settings.FMVersion != def.FMVersion {
		t.Errorf("FMVersion = %q, want default %q", sp.Settings.FMVersion, def.FMVersion)
	}
}

func TestMigrateLegacyNoLegacyDir(t *testing.T) {
	n, err := MigrateLegacy(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected 0 migrated, got %d", n)
	}
}

func TestOpenMigratesOnceViaMarker(t *testing.T) {
	// Simulate an already-migrated store: writing the marker should stop
	// Open from importing legacy profiles again even if we can't easily
	// override LegacyDirs() in this test, so we verify the marker mechanics
	// directly against a directory that has one.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, migratedMarker), []byte("done"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.List()) != 0 {
		t.Fatalf("expected no profiles, got %+v", s.List())
	}
}

func TestAutosaverDebounces(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Create(s, "Debounced", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, p.Slug+".json")

	baseline, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	baselineModTime := baseline.ModTime()

	const delay = 60 * time.Millisecond
	as := NewAutosaver(s, delay, func(err error) {
		t.Errorf("autosave error: %v", err)
	})

	// Fire 5 rapid triggers, spaced well inside the debounce window, and
	// track every distinct mtime the file passes through.
	seen := map[time.Time]bool{baselineModTime: true}
	for i := 0; i < 5; i++ {
		clone := p.Clone()
		clone.Settings.PackDir = filepath.Join("pack", string(rune('a'+i)))
		as.Trigger(clone)
		time.Sleep(delay / 10)
		if info, err := os.Stat(path); err == nil {
			seen[info.ModTime()] = true
		}
	}

	// None of the 5 triggers should have written yet: they were all inside
	// one debounce window.
	if len(seen) != 1 {
		t.Fatalf("file was written %d times during the debounce window, want 0 (still only the baseline mtime)", len(seen)-1)
	}

	// Now let the single debounced write happen.
	time.Sleep(delay * 3)

	got, ok := s.Get(p.Slug)
	if !ok {
		t.Fatal("profile missing after autosave")
	}
	wantSuffix := filepath.Join("pack", "e")
	if got.Settings.PackDir != wantSuffix {
		t.Errorf("PackDir = %q, want %q (only the last trigger should have been written)", got.Settings.PackDir, wantSuffix)
	}

	// Flush with nothing pending should be a no-op.
	as.Flush()
}

func TestAutosaverFlush(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Create(s, "Flushed", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	as := NewAutosaver(s, time.Hour, nil) // long delay: only Flush should write
	clone := p.Clone()
	clone.Settings.PackDir = "flushed-value"
	as.Trigger(clone)
	as.Flush()

	got, ok := s.Get(p.Slug)
	if !ok || got.Settings.PackDir != "flushed-value" {
		t.Fatalf("Flush did not persist synchronously: %+v ok=%v", got, ok)
	}
}

func TestProfileJSONShape(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Create(s, "Shape", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, p.Slug+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"slug", "name", "game_path", "settings", "created_at", "updated_at"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing key %q in saved JSON", key)
		}
	}
}
