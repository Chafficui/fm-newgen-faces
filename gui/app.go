// Package gui is the Fyne desktop application for FM NewGen Faces.
package gui

import (
	"log"
	"os"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"

	"github.com/fsnotify/fsnotify"

	"fmnewgenfaces/assets"
	"fmnewgenfaces/gui/widgets"
	"fmnewgenfaces/internal/brand"
	"fmnewgenfaces/internal/core/pipeline"
	"fmnewgenfaces/internal/core/profile"
	"fmnewgenfaces/internal/i18n"
)

const (
	defaultWindowWidth  = 1000
	defaultWindowHeight = 760
	autosaveDelay       = 500 * time.Millisecond
)

// App is the whole desktop application shell.
type App struct {
	fyneApp fyne.App
	win     fyne.Window

	store *profile.Store

	// state is the persisted app-wide settings (window geometry, last
	// profile, theme/language, update-check bookkeeping). It is read and
	// written both from the UI thread (onClose, setTheme, setLanguage, the
	// wizard) and from the startup/update-check goroutine (runUpdateCheck),
	// so every access goes through getState/updateState rather than the
	// field directly.
	stateMu sync.Mutex
	state   profile.State

	current   *profile.Profile
	autosaver *profile.Autosaver

	logger  *log.Logger
	logFile *os.File
	logTee  *teeWriter
	logView *widgets.LogView

	// inputs is loaded on a background goroutine (startRun, openReviewDialog)
	// and cleared on the UI thread (applyProfile) while widgets.ReviewTable's
	// own reroll goroutine reads it via reviewReroll, so every access goes
	// through inputsMu/getInputs/setInputs rather than the field directly.
	inputsMu sync.Mutex
	inputs   *pipeline.Inputs

	// loading suppresses onChanged/autosave handlers while applyProfile is
	// filling the UI from a profile that was just loaded.
	loading bool

	themeSetting string // "system", "light", "dark"

	// header
	profileSelect  *widget.Select
	profileMenuBtn *widget.Button
	profiles       []*profile.Profile
	bannerSlot     *fyne.Container

	// card 1 – face pack
	packRow            *widgets.PathRow
	packStatusBox      *fyne.Container
	configRow          *widgets.PathRow
	configStatusBox    *fyne.Container
	versionSelect      *widget.Select
	versionStatusBox   *fyne.Container
	installStatusBox   *fyne.Container
	packTable          *widgets.PackTable
	advancedDisclosure *disclosure

	// card 2 – newgen export
	rtfRow       *widgets.PathRow
	rtfStatusBox *fyne.Container
	rtfHintLabel *widget.Label

	// card 3 – assign faces
	preserveCheck       *widget.Check
	allowDupCheck       *widget.Check // label "Avoid duplicate images"; inverted vs Settings.AllowDuplicates
	overrideEditor      *widgets.OverrideEditor
	overridesBtn        *widget.Button
	primaryBtn          *widget.Button
	undoBtn             *widget.Button
	primaryReasonLabel  *widget.Label
	progress            *widgets.ProgressPanel
	resultStrip         *fyne.Container
	resultLabel         *widget.Label
	resultReviewBtn     *widget.Button
	resultOpenFolderBtn *widget.Button

	// review dialog
	reviewTable *widgets.ReviewTable

	// log section (bottom of the page)
	logDisclosure *disclosure

	// evaluate() debounce/serialisation
	evalMu    sync.Mutex
	evalTimer *time.Timer
	evalGen   uint64

	// startRun() serialisation
	runMu   sync.Mutex
	running bool

	// fsnotify watcher on the pack dir / RTF dir
	watchMu sync.Mutex
	watcher *fsnotify.Watcher
}

// Run starts the desktop application and blocks until the window is closed.
func Run() {
	dir, err := profile.DefaultDir()
	if err != nil {
		log.Fatalf("fm-newgen-faces: %v", err)
	}
	fyneApp := app.NewWithID(brand.AppID)
	a := newApp(dir, fyneApp)
	a.start()
}

// newApp wires up the store, logging, i18n and the window/UI, without
// showing the window or starting background work. It is used by Run and by
// tests.
func newApp(configDir string, fyneApp fyne.App) *App {
	store, err := profile.Open(configDir)
	if err != nil {
		log.Fatalf("fm-newgen-faces: opening profile store: %v", err)
	}
	state := store.LoadState()

	i18n.Init(state.Language)
	widgets.T = i18n.T

	logger, logFile, tee, err := openLogger(store.Dir())
	if err != nil {
		log.Printf("fm-newgen-faces: opening log file: %v", err)
	}

	a := &App{
		fyneApp:      fyneApp,
		store:        store,
		state:        state,
		logger:       logger,
		logFile:      logFile,
		logTee:       tee,
		themeSetting: state.Theme,
	}
	a.autosaver = profile.NewAutosaver(store, autosaveDelay, func(err error) {
		a.errorf("autosave failed: %v", err)
	})

	fyneApp.Settings().SetTheme(newTheme(a.themeSetting))

	a.win = fyneApp.NewWindow(brand.AppName)
	a.win.SetIcon(fyne.NewStaticResource("icon.png", assets.Icon()))

	width := state.WindowWidth
	height := state.WindowHeight
	if width <= 0 {
		width = defaultWindowWidth
	}
	if height <= 0 {
		height = defaultWindowHeight
	}
	a.win.Resize(fyne.NewSize(width, height))

	a.buildLayout()
	a.setupShortcuts()
	a.setupDragDrop()

	a.win.SetCloseIntercept(func() {
		a.onClose()
	})

	return a
}

// start shows the window immediately, kicks off startup work in the
// background, then blocks running the Fyne event loop until the window
// closes.
func (a *App) start() {
	a.win.Show()
	go a.runStartup()
	a.fyneApp.Run()
}

// doUI serialises every background-goroutine UI mutation onto fyne.Do.
// fyne's production driver already runs Do callbacks one at a time on the
// main thread, but the test driver runs them inline on the calling
// goroutine with no such serialisation; without this mutex, two of our own
// goroutines calling fyne.Do around the same time can race on shared widget
// state under `go test -race`. Locking here costs nothing in production and
// makes background UI updates deterministic in tests.
var uiMu sync.Mutex

func doUI(fn func()) {
	uiMu.Lock()
	defer uiMu.Unlock()
	fyne.Do(fn)
}

// getState returns a copy of the app state, safe to call from any goroutine.
func (a *App) getState() profile.State {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return a.state
}

// updateState applies fn to the app state under the lock and returns the
// resulting copy (typically passed straight to store.SaveState). Safe to
// call from any goroutine.
func (a *App) updateState(fn func(*profile.State)) profile.State {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	fn(&a.state)
	return a.state
}

// getInputs returns the currently loaded pipeline inputs (nil if none),
// safe to call from any goroutine.
func (a *App) getInputs() *pipeline.Inputs {
	a.inputsMu.Lock()
	defer a.inputsMu.Unlock()
	return a.inputs
}

// setInputs replaces the currently loaded pipeline inputs, safe to call
// from any goroutine.
func (a *App) setInputs(in *pipeline.Inputs) {
	a.inputsMu.Lock()
	a.inputs = in
	a.inputsMu.Unlock()
}

// onClose persists window geometry/last profile and flushes pending saves.
func (a *App) onClose() {
	size := a.win.Canvas().Size()
	state := a.updateState(func(s *profile.State) {
		s.WindowWidth = size.Width
		s.WindowHeight = size.Height
		if a.current != nil {
			s.LastProfile = a.current.Slug
		}
	})
	if err := a.store.SaveState(state); err != nil {
		a.errorf("saving app state: %v", err)
	}
	a.autosaver.Flush()
	a.stopWatcher()
	if a.logFile != nil {
		_ = a.logFile.Close()
	}
	a.win.Close()
}
