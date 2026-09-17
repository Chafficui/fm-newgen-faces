// Package gui is the Fyne desktop application for FM NewGen Faces.
package gui

import (
	"log"
	"os"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
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
	defaultWindowWidth  = 1200
	defaultWindowHeight = 820
	autosaveDelay       = 500 * time.Millisecond
)

// App is the whole desktop application shell.
type App struct {
	fyneApp fyne.App
	win     fyne.Window

	store     *profile.Store
	state     profile.State
	current   *profile.Profile
	autosaver *profile.Autosaver

	logger  *log.Logger
	logFile *os.File
	logTee  *teeWriter
	logView *widgets.LogView

	inputs *pipeline.Inputs

	// loading suppresses onChanged/autosave handlers while applyProfile is
	// filling the UI from a profile that was just loaded.
	loading bool

	themeSetting string // "system", "light", "dark"

	// header
	profileSelect *widget.Select
	profiles      []*profile.Profile
	bannerSlot    *fyne.Container

	tabs *container.AppTabs

	// setup tab
	checklist     *widgets.Checklist
	packRow       *widgets.PathRow
	configRow     *widgets.PathRow
	rtfRow        *widgets.PathRow
	versionSelect *widget.Select
	packTable     *widgets.PackTable

	// settings tab
	preserveCheck  *widget.Check
	allowDupCheck  *widget.Check
	overrideEditor *widgets.OverrideEditor

	// run tab
	previewBtn    *widget.Button
	assignBtn     *widget.Button
	undoBtn       *widget.Button
	openFolderBtn *widget.Button
	progress      *widgets.ProgressPanel

	// review tab
	reviewTable *widgets.ReviewTable

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

// onClose persists window geometry/last profile and flushes pending saves.
func (a *App) onClose() {
	size := a.win.Canvas().Size()
	a.state.WindowWidth = size.Width
	a.state.WindowHeight = size.Height
	if a.current != nil {
		a.state.LastProfile = a.current.Slug
	}
	if err := a.store.SaveState(a.state); err != nil {
		a.errorf("saving app state: %v", err)
	}
	a.autosaver.Flush()
	a.stopWatcher()
	if a.logFile != nil {
		_ = a.logFile.Close()
	}
	a.win.Close()
}
