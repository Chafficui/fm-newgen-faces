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
	"fmnewgenfaces/internal/core/rtf"
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
		// evaluate() schedules a debounced timer that outlives this test if
		// nothing waits for it (e.g. after a successful executeRun); left
		// running, its doUI callback can fire during a later test's
		// newApp()/buildLayout(), which touches Fyne's global
		// text-measurement cache without going through doUI's mutex. Cancel
		// it so no test leaks a background UI mutation into the next one.
		a.evalMu.Lock()
		if a.evalTimer != nil {
			a.evalTimer.Stop()
		}
		a.evalMu.Unlock()
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

	// Exercise the same non-dialog logic startRun/continueRun use, without
	// going through the preview/unmapped-resolver dialogs.
	settings := cloneSettings(a.current.Settings)
	in, err := pipeline.Load(settings)
	if err != nil {
		t.Fatalf("pipeline.Load: %v", err)
	}
	a.setInputs(in)
	plan := pipeline.Plan(in)

	// executeRun no longer manages a.running itself (startRun/continueRun
	// own that for the whole preview/run flow), so the test sets it the
	// same way startRun would before calling in.
	a.runMu.Lock()
	a.running = true
	a.runMu.Unlock()

	a.executeRun(in, plan)

	waitUntil(t, 2*time.Second, func() bool { return !a.isRunning() })

	if _, err := os.Stat(a.current.Settings.ConfigXML); err != nil {
		t.Fatalf("expected config.xml to exist after executeRun: %v", err)
	}
}

// TestExecuteRunIgnoresAppInputs is finding 3(a): executeRun must use the
// *pipeline.Inputs its plan was built from, threaded in explicitly, and
// never re-read a.inputs — which may have been replaced (e.g. by
// applyProfile switching profiles) while a preview built from the original
// inputs is still open.
func TestExecuteRunIgnoresAppInputs(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)

	a.refreshProfileList()
	a.applyProfile(p)

	if _, err := os.Stat(a.current.Settings.ConfigXML); err == nil {
		t.Fatalf("config.xml should not exist yet")
	}

	settings := cloneSettings(a.current.Settings)
	in, err := pipeline.Load(settings)
	if err != nil {
		t.Fatalf("pipeline.Load: %v", err)
	}
	plan := pipeline.Plan(in)

	// Simulate a.inputs being cleared out from under this in-flight
	// plan/inputs pair, e.g. by applyProfile.
	a.setInputs(nil)

	a.runMu.Lock()
	a.running = true
	a.runMu.Unlock()

	a.executeRun(in, plan)

	waitUntil(t, 2*time.Second, func() bool { return !a.isRunning() })

	if _, err := os.Stat(a.current.Settings.ConfigXML); err != nil {
		t.Fatalf("expected config.xml to exist after executeRun despite a.inputs being nil: %v", err)
	}
}

// TestStartRunNoopWhileRunning is finding 3(b): startRun must return early,
// without touching a.inputs, when a run/preview flow is already in
// progress.
func TestStartRunNoopWhileRunning(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	a.refreshProfileList()
	a.applyProfile(p)

	a.runMu.Lock()
	a.running = true
	a.runMu.Unlock()

	a.startRun(true) // dryRun Preview; must be a no-op while already running

	if a.getInputs() != nil {
		t.Fatalf("startRun should not have loaded inputs while a run was already in progress")
	}
	if !a.isRunning() {
		t.Fatalf("startRun must not clear a.running when it declines to start")
	}
}

// TestReviewRerollErrorsWhenNoInputs is finding 2's error path: reviewReroll
// must return an error, not panic, when no inputs are loaded.
func TestReviewRerollErrorsWhenNoInputs(t *testing.T) {
	a := newTestApp(t)

	if a.getInputs() != nil {
		t.Fatalf("expected no inputs loaded on a fresh app")
	}

	_, err := a.reviewReroll(rtf.Player{ID: "2000000001", Name: "Uno"})
	if err == nil {
		t.Fatalf("expected reviewReroll to error out when no inputs are loaded")
	}
}
