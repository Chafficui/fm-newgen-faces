package gui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"

	"fmnewgenfaces/gui/widgets"
	"fmnewgenfaces/internal/core/ethnic"
	"fmnewgenfaces/internal/core/pipeline"
)

// TestScreenshots renders the application with Fyne's software painter and
// writes one PNG per screen into $FM_NEWGEN_SCREENSHOTS. It is skipped unless
// that variable is set, so CI never runs it.
func TestScreenshots(t *testing.T) {
	out := os.Getenv("FM_NEWGEN_SCREENSHOTS")
	if out == "" {
		t.Skip("set FM_NEWGEN_SCREENSHOTS=<dir> to render screenshots")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}

	a := newTestApp(t)
	p := newTestProfile(t, a)
	a.refreshProfileList()
	a.applyProfile(p)
	a.win.Resize(fyne.NewSize(1200, 820))
	a.evaluateSync()
	a.logf("Football Manager 2024: 2 file(s) installed, 0 already up to date")
	a.logf("using newgen export found in the face pack folder: %s", p.Settings.RTFPath)

	save := func(name string) {
		t.Helper()
		time.Sleep(150 * time.Millisecond)
		img := a.win.Canvas().Capture()
		f, err := os.Create(filepath.Join(out, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}

	a.tabs.SelectIndex(0)
	save("1-setup")
	a.tabs.SelectIndex(1)
	save("2-settings")
	a.tabs.SelectIndex(2)
	save("3-run")

	in, err := pipeline.Load(cloneSettings(a.current.Settings))
	if err != nil {
		t.Fatal(err)
	}
	widgets.ShowUnmappedResolver(a.win, in.RTF.Unmapped, ethnic.Names(), func(map[string]string) {}, func() {})
	save("4-unmapped-nations")
	closeOverlays(a)

	plan := pipeline.Plan(in)
	widgets.ShowPreview(a.win, plan, "Assign faces", func() {}, func() {})
	save("5-preview")
	closeOverlays(a)

	rr, err := pipeline.Run(in, plan, a.backupBaseDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	widgets.ShowSummary(a.win, plan, rr.Result, rr.BackupPath, widgets.SummaryActions{
		OpenFolder: func() {}, ShowLog: func() {}, Undo: func() {}, Review: func() {},
	})
	save("6-summary")
	closeOverlays(a)

	a.evaluateSync()
	a.tabs.SelectIndex(3)
	a.loadCurrentMappings()
	time.Sleep(600 * time.Millisecond)
	save("7-review")

	a.tabs.SelectIndex(0)
	save("8-setup-after-run")

	a.showWizard()
	save("9-wizard")
	closeOverlays(a)
}

func closeOverlays(a *App) {
	for _, o := range a.win.Canvas().Overlays().List() {
		a.win.Canvas().Overlays().Remove(o)
	}
}
