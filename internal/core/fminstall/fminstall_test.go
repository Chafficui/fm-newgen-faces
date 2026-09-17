package fminstall

import (
	"os"
	"path/filepath"
	"testing"
)

func TestViewsFiltersDir(t *testing.T) {
	inst := Install{BasePath: filepath.Join("base", "Football Manager 2024")}
	if got, want := inst.ViewsDir(), filepath.Join(inst.BasePath, "views"); got != want {
		t.Errorf("ViewsDir() = %q, want %q", got, want)
	}
	if got, want := inst.FiltersDir(), filepath.Join(inst.BasePath, "filters"); got != want {
		t.Errorf("FiltersDir() = %q, want %q", got, want)
	}
}

func TestDetectCustomEnvVar(t *testing.T) {
	// Isolate from any real "Sports Interactive" folder on the host.
	isolatedHome := t.TempDir()
	t.Setenv("HOME", isolatedHome)
	t.Setenv("USERPROFILE", isolatedHome)

	tmp := t.TempDir()
	base := filepath.Join(tmp, "CustomBase")
	fm24 := filepath.Join(base, "Football Manager 2024")
	other := filepath.Join(base, "Not FM")
	if err := os.MkdirAll(fm24, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv(fmDirsEnvVar, base)

	installs := Detect()
	if len(installs) != 1 {
		t.Fatalf("Detect() returned %d installs, want 1: %+v", len(installs), installs)
	}
	if installs[0].BasePath != fm24 {
		t.Errorf("BasePath = %q, want %q", installs[0].BasePath, fm24)
	}
	if installs[0].Source != "custom" {
		t.Errorf("Source = %q, want custom", installs[0].Source)
	}
	if installs[0].Version.Year != "2024" {
		t.Errorf("Version.Year = %q, want 2024", installs[0].Version.Year)
	}
}

func TestDetectSortsNewestFirst(t *testing.T) {
	isolatedHome := t.TempDir()
	t.Setenv("HOME", isolatedHome)
	t.Setenv("USERPROFILE", isolatedHome)

	base := t.TempDir()
	for _, name := range []string{"Football Manager 2022", "Football Manager 2024", "Football Manager 2023"} {
		if err := os.MkdirAll(filepath.Join(base, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(fmDirsEnvVar, base)

	installs := Detect()
	if len(installs) != 3 {
		t.Fatalf("got %d installs, want 3", len(installs))
	}
	want := []string{"2024", "2023", "2022"}
	for i, w := range want {
		if installs[i].Version.Year != w {
			t.Errorf("installs[%d].Version.Year = %q, want %q", i, installs[i].Version.Year, w)
		}
	}
}

func TestDetectDedupesEnvAndRoots(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	siDir := filepath.Join(tmpHome, "Documents", "Sports Interactive")
	fm24 := filepath.Join(siDir, "Football Manager 2024")
	if err := os.MkdirAll(fm24, 0o755); err != nil {
		t.Fatal(err)
	}

	// Point the env var at the very same root; it must not appear twice.
	t.Setenv(fmDirsEnvVar, siDir)

	installs := Detect()
	if len(installs) != 1 {
		t.Fatalf("Detect() returned %d installs, want 1 (deduped): %+v", len(installs), installs)
	}
}

func TestFromPath(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "Sports Interactive", "Football Manager 2024")
	nested := filepath.Join(base, "graphics", "pictures", "person")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	inst, ok := FromPath(nested)
	if !ok {
		t.Fatal("FromPath() ok = false, want true")
	}
	if inst.BasePath != base {
		t.Errorf("BasePath = %q, want %q", inst.BasePath, base)
	}
	if inst.Version.Year != "2024" {
		t.Errorf("Version.Year = %q, want 2024", inst.Version.Year)
	}
}

func TestFromPathNotFound(t *testing.T) {
	tmp := t.TempDir()
	if _, ok := FromPath(tmp); ok {
		t.Fatal("FromPath() ok = true, want false for an unrelated path")
	}
}

func TestDistribute(t *testing.T) {
	tmp := t.TempDir()
	inst := Install{Name: "Football Manager 2024", BasePath: tmp}

	result, err := Distribute(inst)
	if err != nil {
		t.Fatalf("Distribute() error = %v", err)
	}
	if len(result.Copied) == 0 {
		t.Fatal("expected some files to be copied on first run")
	}
	if len(result.Skipped) != 0 {
		t.Fatalf("expected no skips on first run, got %v", result.Skipped)
	}

	// Second run: everything should already match, so nothing is copied.
	result2, err := Distribute(inst)
	if err != nil {
		t.Fatalf("Distribute() second call error = %v", err)
	}
	if len(result2.Copied) != 0 {
		t.Fatalf("expected no copies on second run, got %v", result2.Copied)
	}
	if len(result2.Skipped) != len(result.Copied) {
		t.Fatalf("expected %d skips on second run, got %d", len(result.Copied), len(result2.Skipped))
	}

	// Modify a distributed file: it must be re-copied.
	changed := result.Copied[0]
	if err := os.WriteFile(changed, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	result3, err := Distribute(inst)
	if err != nil {
		t.Fatalf("Distribute() third call error = %v", err)
	}
	found := false
	for _, c := range result3.Copied {
		if c == changed {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %q to be re-copied after being tampered with", changed)
	}
}

func TestRootsNoPanic(t *testing.T) {
	// Just exercise Roots() end-to-end; it must not panic and every entry it
	// returns must actually exist.
	for _, r := range Roots() {
		if info, err := os.Stat(r); err != nil || !info.IsDir() {
			t.Errorf("Roots() returned non-existent directory %q", r)
		}
	}
}
