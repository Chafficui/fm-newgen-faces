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
	"fmnewgenfaces/internal/core/profile"
	"fmnewgenfaces/internal/i18n"
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
	a.win.Resize(fyne.NewSize(1000, 760))

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

	// 1 – a brand new profile with no pack set: cards 2/3 dimmed, the
	// friendly "choose the folder" message.
	empty, err := profile.Create(a.store, "Empty Profile", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	a.refreshProfileList()
	a.applyProfile(empty)
	a.evaluateSync()
	save("1-main-empty")

	// 2 – a fully set-up profile: pack + RTF set, checks OK.
	p := newTestProfile(t, a)
	a.refreshProfileList()
	a.applyProfile(p)
	a.evaluateSync()
	a.logf("Football Manager 2024: 2 file(s) installed, 0 already up to date")
	a.logf("using newgen export found in the face pack folder: %s", p.Settings.RTFPath)
	save("2-main-ready")

	// 3 – Card 1's Advanced disclosure opened.
	a.advancedDisclosure.setOpen(a, true)
	save("3-advanced-open")
	a.advancedDisclosure.setOpen(a, false)

	in, err := pipeline.Load(cloneSettings(a.current.Settings))
	if err != nil {
		t.Fatal(err)
	}
	plan := pipeline.Plan(in)

	// 4 – the preview dialog (what startRun(false) shows before confirming).
	widgets.ShowPreview(a.win, plan, i18n.T("gui.run.assign"), func() {}, func() {})
	save("4-preview")
	closeOverlays(a)

	// 5 – after executeRun: the inline result strip in Card 3.
	a.setInputs(in)
	a.runMu.Lock()
	a.running = true
	a.runMu.Unlock()
	a.executeRun(in, plan)
	waitUntil(t, 2*time.Second, func() bool { return !a.isRunning() })
	closeOverlays(a) // the summary dialog also opens; hide it to see the strip underneath
	save("5-running-or-result")

	// 6 – the review dialog, auto-loaded from the run's inputs.
	a.openReviewDialog()
	time.Sleep(300 * time.Millisecond)
	save("6-review-dialog")
	closeOverlays(a)

	// 7 – the unmapped-nations resolver.
	in2, err := pipeline.Load(cloneSettings(a.current.Settings))
	if err != nil {
		t.Fatal(err)
	}
	widgets.ShowUnmappedResolver(a.win, in2.RTF.Unmapped, ethnic.Names(), func(map[string]string) {}, func() {})
	save("7-unmapped")
	closeOverlays(a)

	// 8 – the first-run wizard.
	a.showWizard()
	save("8-wizard")
	closeOverlays(a)

	// 9 – the log section, expanded (an error logged this way auto-expands
	// it too — see TestErrorLogAutoExpandsLogSection — but the expand here
	// is forced directly so the screenshot isn't racing that goroutine hop).
	// The window is made taller first so the expanded log (below the fold
	// at the normal 760px height) is actually in frame.
	a.errorf("example error for the screenshot")
	a.logDisclosure.setOpen(a, true)
	a.win.Resize(fyne.NewSize(1000, 1000))
	save("9-log-expanded")
	a.toggleLog()
	a.win.Resize(fyne.NewSize(1000, 760))

	// 10 – the settings gear menu open (skipped if popups don't render in a
	// software capture).
	a.showSettingsMenu(a.profileSelect)
	save("10-settings-menu-open")
	closeOverlays(a)
}

func closeOverlays(a *App) {
	for _, o := range a.win.Canvas().Overlays().List() {
		a.win.Canvas().Overlays().Remove(o)
	}
}

func copyDir(t *testing.T, from, to string) {
	t.Helper()
	entries, err := os.ReadDir(from)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		src := filepath.Join(from, e.Name())
		dst := filepath.Join(to, e.Name())
		if e.IsDir() {
			os.MkdirAll(dst, 0o755)
			copyDir(t, src, dst)
			continue
		}
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
