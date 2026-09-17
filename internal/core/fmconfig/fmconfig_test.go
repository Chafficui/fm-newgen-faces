package fmconfig

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fmnewgenfaces/internal/core/ethnic"
	"fmnewgenfaces/internal/core/fmversion"
)

var fm24 = fmversion.Version{Year: "2024", Label: "FM24", IDPrefix: "r-"}
var fm23 = fmversion.Version{Year: "2023", Label: "FM23", IDPrefix: ""}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestPortraitPath(t *testing.T) {
	if got := PortraitPath(fm24, "2000134233"); got != "graphics/pictures/person/r-2000134233/portrait" {
		t.Errorf("PortraitPath(fm24) = %q", got)
	}
	if got := PortraitPath(fm23, "2000134233"); got != "graphics/pictures/person/2000134233/portrait" {
		t.Errorf("PortraitPath(fm23) = %q", got)
	}
}

func TestParsePortrait(t *testing.T) {
	if uid, ok := ParsePortrait(fm24, "graphics/pictures/person/r-2000134233/portrait"); !ok || uid != "2000134233" {
		t.Errorf("ParsePortrait(fm24, r-path) = %q,%v", uid, ok)
	}
	if _, ok := ParsePortrait(fm24, "/graphics/pictures/person/r-2000134233/portrait"); !ok {
		t.Errorf("expected leading slash to be tolerated")
	}
	if _, ok := ParsePortrait(fm24, `graphics\pictures\person\r-2000134233\portrait`); !ok {
		t.Errorf("expected backslashes to be tolerated")
	}
	if _, ok := ParsePortrait(fm24, "graphics/pictures/person/2000134233/portrait"); ok {
		t.Errorf("fm24 (prefix r-) must not match a bare-digit path")
	}
	// Version with empty IDPrefix must NOT match an "r-" path.
	if _, ok := ParsePortrait(fm23, "graphics/pictures/person/r-2000134233/portrait"); ok {
		t.Errorf("fm23 (empty prefix) must not match an r- path")
	}
	if uid, ok := ParsePortrait(fm23, "graphics/pictures/person/2000134233/portrait"); !ok || uid != "2000134233" {
		t.Errorf("ParsePortrait(fm23) = %q,%v", uid, ok)
	}
	if _, ok := ParsePortrait(fm24, "graphics/pictures/kits/somekit.png"); ok {
		t.Errorf("non-portrait path should not match")
	}
}

func TestLoad_MissingFileError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.xml")
	_, err := Load(path, fm24)
	if err == nil || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Load missing file: err = %v, want fs.ErrNotExist", err)
	}
}

func TestLoadOrNew(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.xml")
	cfg, err := LoadOrNew(path, fm24)
	if err != nil {
		t.Fatalf("LoadOrNew: %v", err)
	}
	if cfg.Preload || cfg.Amap || cfg.Count() != 0 {
		t.Errorf("LoadOrNew on missing file should return empty config, got %+v", cfg)
	}
}

func TestLoad_MalformedXML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.xml")
	mustWrite(t, path, "<record><not-closed>")
	if _, err := Load(path, fm24); err == nil {
		t.Fatalf("expected error for malformed XML")
	}
}

// TestRoundTrip_PreservesForeignRecords proves that a file with 3 real-player
// records, 2 newgen records and 1 record using the other prefix style saves
// with all 6 intact, and only the 2 newgen ones are editable.
func TestRoundTrip_PreservesForeignRecords(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.xml")

	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<record>
	<boolean id="preload" value="true"/>
	<boolean id="amap" value="false"/>
	<list id="maps">
		<record from="players/real1" to="graphics/pictures/person/1000000001/portrait"/>
		<record from="players/real2" to="graphics/pictures/person/1000000002/portrait"/>
		<record from="players/real3" to="graphics/pictures/person/1000000003/portrait"/>
		<record from="faces/African/aaa" to="graphics/pictures/person/r-2000000001/portrait"/>
		<record from="faces/Asian/bbb" to="graphics/pictures/person/r-2000000002/portrait"/>
		<record from="other/prefix/style" to="graphics/pictures/person/other-2000000003/portrait"/>
	</list>
</record>`
	mustWrite(t, path, xmlContent)

	cfg, err := Load(path, fm24)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !cfg.Preload || cfg.Amap {
		t.Fatalf("Preload/Amap not preserved: preload=%v amap=%v", cfg.Preload, cfg.Amap)
	}

	if cfg.Count() != 2 {
		t.Fatalf("Count() = %d, want 2 (only records matching fm24's r- prefix)", cfg.Count())
	}
	if from, ok := cfg.Get("2000000001"); !ok || from != "faces/African/aaa" {
		t.Errorf("Get(2000000001) = %q,%v", from, ok)
	}
	if from, ok := cfg.Get("2000000002"); !ok || from != "faces/Asian/bbb" {
		t.Errorf("Get(2000000002) = %q,%v", from, ok)
	}

	passthrough := cfg.Passthrough()
	if len(passthrough) != 4 {
		t.Fatalf("Passthrough() len = %d, want 4, got %+v", len(passthrough), passthrough)
	}

	// Edit one newgen mapping to prove it's the editable one.
	cfg.Set("2000000001", "faces/African/zzz")

	backupPath, err := cfg.Save(SaveOptions{})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if backupPath != "" {
		t.Errorf("no backup should be made without BackupDir, got %q", backupPath)
	}

	// Reload and verify all 6 records survived, with the edit intact.
	reloaded, err := Load(path, fm24)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Count() != 2 {
		t.Fatalf("reloaded Count() = %d, want 2", reloaded.Count())
	}
	if from, _ := reloaded.Get("2000000001"); from != "faces/African/zzz" {
		t.Errorf("edit not preserved: got %q", from)
	}
	if from, _ := reloaded.Get("2000000002"); from != "faces/Asian/bbb" {
		t.Errorf("unedited newgen changed: got %q", from)
	}
	if len(reloaded.Passthrough()) != 4 {
		t.Fatalf("reloaded Passthrough() len = %d, want 4", len(reloaded.Passthrough()))
	}

	total := reloaded.Count() + len(reloaded.Passthrough())
	if total != 6 {
		t.Fatalf("total records after round trip = %d, want 6", total)
	}

	// Verify the raw file content still has all 6 <record> elements.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	content := string(data)
	if !strings.HasPrefix(content, `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Errorf("missing XML declaration header")
	}
	if got := strings.Count(content, "<record from="); got != 6 {
		t.Errorf("saved file has %d <record> elements, want 6:\n%s", got, content)
	}
	for _, want := range []string{"players/real1", "players/real2", "players/real3", "other/prefix/style", "faces/African/zzz", "faces/Asian/bbb"} {
		if !strings.Contains(content, want) {
			t.Errorf("saved file missing %q:\n%s", want, content)
		}
	}
}

func TestSave_OutputFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.xml")

	cfg := New(path, fm24)
	cfg.Preload = true
	cfg.Set("2000000001", "faces/African/aaa")
	cfg.Set("2000000002", "faces/Asian/bbb")

	if _, err := cfg.Save(SaveOptions{}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)

	wantPrefix := `<?xml version="1.0" encoding="UTF-8"?>` + "\n<record>\n"
	if !strings.HasPrefix(content, wantPrefix) {
		t.Fatalf("unexpected header:\n%s", content)
	}
	if !strings.Contains(content, "\t<boolean id=\"preload\" value=\"true\"/>\n") {
		t.Errorf("preload boolean missing/wrong:\n%s", content)
	}
	if !strings.Contains(content, "\t<boolean id=\"amap\" value=\"false\"/>\n") {
		t.Errorf("amap boolean missing/wrong:\n%s", content)
	}
	if !strings.Contains(content, "\t<list id=\"maps\">\n") {
		t.Errorf("list open tag missing/wrong:\n%s", content)
	}
	if !strings.Contains(content, "\t\t<record from=\"faces/African/aaa\" to=\"graphics/pictures/person/r-2000000001/portrait\"/>\n") {
		t.Errorf("expected tab-indented record line:\n%s", content)
	}
	// First-seen order: 2000000001 before 2000000002.
	idx1 := strings.Index(content, "2000000001")
	idx2 := strings.Index(content, "2000000002")
	if idx1 == -1 || idx2 == -1 || idx1 > idx2 {
		t.Errorf("expected first-seen order in output:\n%s", content)
	}
	if !strings.HasSuffix(strings.TrimRight(content, "\n"), "</record>") {
		t.Errorf("expected file to end with </record>:\n%s", content)
	}
}

func TestSave_AtomicWrite_NoLeftoverTmp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.xml")
	cfg := New(path, fm24)
	cfg.Set("1", "faces/African/a")

	if _, err := cfg.Save(SaveOptions{}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf(".tmp file should not remain after Save, stat err = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected config.xml to exist: %v", err)
	}
}

func TestImagePath(t *testing.T) {
	base := t.TempDir()
	configDir := filepath.Join(base, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.xml")

	// Case 1: pack root == config dir.
	got, err := ImagePath(configPath, configDir, ethnic.African, "img")
	if err != nil {
		t.Fatalf("ImagePath: %v", err)
	}
	if got != "African/img" {
		t.Errorf("case1 = %q, want African/img", got)
	}

	// Case 2: pack under config dir.
	packUnder := filepath.Join(configDir, "faces")
	got, err = ImagePath(configPath, packUnder, ethnic.African, "img")
	if err != nil {
		t.Fatalf("ImagePath: %v", err)
	}
	if got != "faces/African/img" {
		t.Errorf("case2 = %q, want faces/African/img", got)
	}

	// Case 3: pack beside config dir.
	packBeside := filepath.Join(base, "pack")
	got, err = ImagePath(configPath, packBeside, ethnic.African, "img")
	if err != nil {
		t.Fatalf("ImagePath: %v", err)
	}
	if got != "../pack/African/img" {
		t.Errorf("case3 = %q, want ../pack/African/img", got)
	}
}

func TestGenerate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.xml")

	if err := Generate(path); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), `<list id="maps">`) {
		t.Errorf("generated content unexpected:\n%s", data)
	}

	// A second call must not overwrite an existing file.
	mustWrite(t, path, "custom content")
	if err := Generate(path); err != nil {
		t.Fatalf("Generate (existing): %v", err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "custom content" {
		t.Errorf("Generate overwrote an existing file")
	}
}

func TestBackupDirFor(t *testing.T) {
	base := t.TempDir()
	a := BackupDirFor(base, "/tmp/x/config.xml")
	b := BackupDirFor(base, "/tmp/x/config.xml")
	if a != b {
		t.Errorf("BackupDirFor not stable: %q vs %q", a, b)
	}
	c := BackupDirFor(base, "/tmp/y/config.xml")
	if a == c {
		t.Errorf("BackupDirFor should differ for different config paths")
	}
	if !strings.HasPrefix(a, base+string(filepath.Separator)) {
		t.Errorf("BackupDirFor(%q) not under base %q", a, base)
	}
}

func TestSave_BackupsAndPrune(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.xml")
	backupBase := t.TempDir()
	backupDir := BackupDirFor(backupBase, path)

	// Deterministic, strictly increasing timestamps without sleeping.
	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.Local)
	tick := 0
	origNow := nowFunc
	nowFunc = func() time.Time {
		t := base.Add(time.Duration(tick) * time.Second)
		tick++
		return t
	}
	defer func() { nowFunc = origNow }()

	cfg := New(path, fm24)
	cfg.Set("1", "faces/African/a")

	// First save: file doesn't exist yet, no backup should be made.
	bp, err := cfg.Save(SaveOptions{BackupDir: backupDir, KeepBackups: 2})
	if err != nil {
		t.Fatalf("Save 1: %v", err)
	}
	if bp != "" {
		t.Errorf("expected no backup on first save (no prior file), got %q", bp)
	}

	// Subsequent saves back up the previous version.
	for i := 0; i < 3; i++ {
		cfg.Set("1", "faces/African/edit")
		bp, err := cfg.Save(SaveOptions{BackupDir: backupDir, KeepBackups: 2})
		if err != nil {
			t.Fatalf("Save %d: %v", i+2, err)
		}
		if bp == "" {
			t.Fatalf("expected a backup path on save %d", i+2)
		}
	}

	backups, err := ListBackups(backupDir)
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) != 2 {
		t.Fatalf("ListBackups len = %d, want 2 (pruned to KeepBackups)", len(backups))
	}
	if !backups[0].Time.After(backups[1].Time) {
		t.Errorf("ListBackups not newest-first: %v then %v", backups[0].Time, backups[1].Time)
	}
	for _, b := range backups {
		if b.Size == 0 {
			t.Errorf("backup %q has zero size", b.Path)
		}
	}
}

func TestRestore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.xml")
	mustWrite(t, path, "current content")

	backupDir := filepath.Join(dir, "backups")
	mustWrite(t, filepath.Join(backupDir, "config-20240101-120000.xml"), "old content")

	if err := Restore(filepath.Join(backupDir, "config-20240101-120000.xml"), path); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "old content" {
		t.Errorf("Restore did not replace file content: got %q", data)
	}
	if _, err := os.Stat(path + ".tmp"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Restore left a .tmp file")
	}
}

func TestAssignedImagesAndDelete(t *testing.T) {
	cfg := New(filepath.Join(t.TempDir(), "config.xml"), fm24)
	cfg.Set("a", "faces/African/1")
	cfg.Set("b", "faces/Asian/2")

	imgs := cfg.AssignedImages()
	if len(imgs) != 2 || imgs[0] != "faces/African/1" || imgs[1] != "faces/Asian/2" {
		t.Errorf("AssignedImages() = %v", imgs)
	}

	cfg.Delete("a")
	if cfg.Has("a") {
		t.Errorf("Has(a) true after Delete")
	}
	if cfg.Count() != 1 {
		t.Errorf("Count() = %d, want 1", cfg.Count())
	}
	imgs = cfg.AssignedImages()
	if len(imgs) != 1 || imgs[0] != "faces/Asian/2" {
		t.Errorf("AssignedImages() after delete = %v", imgs)
	}
}
