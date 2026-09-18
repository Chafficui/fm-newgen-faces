package gui

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const rtfWatchDebounce = 1 * time.Second

// rearmWatcher closes any existing watcher and watches the current
// profile's pack directory and the RTF's containing directory (whichever
// exist), so a new export dropped into either is picked up.
func (a *App) rearmWatcher() {
	a.watchMu.Lock()
	defer a.watchMu.Unlock()

	if a.watcher != nil {
		_ = a.watcher.Close()
		a.watcher = nil
	}
	if a.current == nil {
		return
	}

	dirs := map[string]bool{}
	if a.current.Settings.PackDir != "" {
		dirs[a.current.Settings.PackDir] = true
	}
	if a.current.Settings.RTFPath != "" {
		dirs[filepath.Dir(a.current.Settings.RTFPath)] = true
	}
	if len(dirs) == 0 {
		return
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		a.warnf("watcher: %v", err)
		return
	}

	watching := false
	for d := range dirs {
		if err := w.Add(d); err != nil {
			a.warnf("watching %s: %v", d, err)
			continue
		}
		watching = true
	}
	if !watching {
		_ = w.Close()
		return
	}

	a.watcher = w
	go a.watchLoop(w)
}

// watcherArmed reports whether the fsnotify watcher is currently active, so
// Card 2's hint line can note that new exports are picked up automatically.
func (a *App) watcherArmed() bool {
	a.watchMu.Lock()
	defer a.watchMu.Unlock()
	return a.watcher != nil
}

// stopWatcher closes the watcher, if any (called on shutdown).
func (a *App) stopWatcher() {
	a.watchMu.Lock()
	defer a.watchMu.Unlock()
	if a.watcher != nil {
		_ = a.watcher.Close()
		a.watcher = nil
	}
}

func (a *App) watchLoop(w *fsnotify.Watcher) {
	var mu sync.Mutex
	var timer *time.Timer

	debounced := func(path string) {
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(rtfWatchDebounce, func() {
			doUI(func() { a.onRTFCreatedOrChanged(path) })
		})
	}

	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if ev.Op&(fsnotify.Create|fsnotify.Write) == 0 {
				continue
			}
			if !strings.EqualFold(filepath.Ext(ev.Name), ".rtf") {
				continue
			}
			debounced(ev.Name)
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			a.warnf("watcher error: %v", err)
		}
	}
}

// onRTFCreatedOrChanged runs on the UI thread (via fyne.Do). It fills in the
// RTF path when it was empty or pointed at a now-missing file, then
// re-evaluates.
func (a *App) onRTFCreatedOrChanged(path string) {
	a.logf("new newgen export detected: %s", path)
	if a.current == nil {
		return
	}

	cur := a.current.Settings.RTFPath
	needsUpdate := cur == ""
	if !needsUpdate {
		if _, err := os.Stat(cur); err != nil {
			needsUpdate = true
		}
	}
	if needsUpdate {
		a.loading = true
		a.rtfRow.SetText(path)
		a.loading = false
		a.current.Settings.RTFPath = path
		a.autosaver.Trigger(a.current)
	}
	a.evaluate()
}
