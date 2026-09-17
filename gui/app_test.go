package gui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"fmnewgenfaces/gui/widgets"
	"fmnewgenfaces/internal/core/ethnic"
	"fmnewgenfaces/internal/core/pipeline"
	"fmnewgenfaces/internal/core/profile"
)

// rtfSample uses the same column format as
// internal/core/pipeline/pipeline_test.go's fixture, but with only cleanly
// resolvable rows (no unknown nations, duplicates or malformed lines) so a
// freshly-created profile checks out as fully OK.
const rtfSample = "| UID       | Nat | 2nd Nat | Name | | | |\r\n" +
	"| 2000000001| ESP |         | Uno  | 1 | 9 | 0 |\r\n" +
	"| 2000000002| GER | RSA     | Dos  | 1 | 16| 3 |\r\n"

// writeTestPack creates the 14 ethnic subfolders under root, each with
// perGroup dummy PNGs.
func writeTestPack(t *testing.T, root string, perGroup int) {
	t.Helper()
	for _, e := range ethnic.All {
		dir := filepath.Join(root, string(e))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < perGroup; i++ {
			name := filepath.Join(dir, string(e[:2])+string(rune('a'+i))+".png")
			if err := os.WriteFile(name, []byte("png"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

func rowByKey(rows []widgets.ChecklistRow, key string) (widgets.ChecklistRow, bool) {
	for _, r := range rows {
		if r.Key == key {
			return r, true
		}
	}
	return widgets.ChecklistRow{}, false
}

func newTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	fyneApp := test.NewApp()
	a := newApp(dir, fyneApp)
	t.Cleanup(func() {
		a.stopWatcher()
	})
	return a
}

func newTestProfile(t *testing.T, a *App) *profile.Profile {
	t.Helper()
	root := t.TempDir()
	pack := filepath.Join(root, "faces")
	writeTestPack(t, pack, 2)
	rtfPath := filepath.Join(pack, "newgen.rtf")
	if err := os.WriteFile(rtfPath, []byte(rtfSample), 0o644); err != nil {
		t.Fatal(err)
	}

	settings := profile.DefaultSettings()
	settings.PackDir = pack
	settings.ConfigXML = filepath.Join(pack, "config.xml")
	settings.RTFPath = rtfPath
	settings.FMVersion = "2024"
	settings.Preserve = true
	settings.AllowDuplicates = true

	p, err := profile.Create(a.store, "Test Profile", "", &settings)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEvaluateSyncProducesExpectedChecklist(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)

	a.refreshProfileList()
	a.applyProfile(p)
	a.evaluateSync()

	rows, pack := a.computeChecklist(cloneSettings(a.current.Settings))
	if pack == nil {
		t.Fatalf("expected a scanned pack")
	}

	packRow, ok := rowByKey(rows, "pack")
	if !ok || packRow.Status != widgets.StatusOK {
		t.Errorf("expected pack row OK, got %+v (found=%v)", packRow, ok)
	}

	rtfRow, ok := rowByKey(rows, "rtf")
	if !ok || rtfRow.Status != widgets.StatusOK {
		t.Errorf("expected rtf row OK, got %+v (found=%v)", rtfRow, ok)
	}

	configRow, ok := rowByKey(rows, "config")
	if !ok || configRow.Status != widgets.StatusWarn {
		t.Errorf("expected config row Warn (config.xml does not exist yet), got %+v (found=%v)", configRow, ok)
	}
}

func TestStartRunHelpersWriteConfig(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)

	a.refreshProfileList()
	a.applyProfile(p)

	if _, err := os.Stat(a.current.Settings.ConfigXML); err == nil {
		t.Fatalf("config.xml should not exist yet")
	}

	// Exercise the same non-dialog logic startRun uses, without going
	// through the preview/unmapped-resolver dialogs.
	settings := cloneSettings(a.current.Settings)
	in, err := pipeline.Load(settings)
	if err != nil {
		t.Fatalf("pipeline.Load: %v", err)
	}
	a.inputs = in
	plan := pipeline.Plan(in)

	a.executeRun(plan)

	waitUntil(t, 2*time.Second, func() bool {
		a.runMu.Lock()
		defer a.runMu.Unlock()
		return !a.running
	})

	if _, err := os.Stat(a.current.Settings.ConfigXML); err != nil {
		t.Fatalf("expected config.xml to exist after executeRun: %v", err)
	}
}
