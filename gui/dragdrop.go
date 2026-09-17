package gui

import (
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"

	"fmnewgenfaces/internal/core/facepack"
)

// setupDragDrop lets the user drop the RTF export or the face pack folder
// onto the window.
func (a *App) setupDragDrop() {
	a.win.SetOnDropped(func(_ fyne.Position, items []fyne.URI) {
		for _, u := range items {
			a.handleDrop(u)
		}
	})
}

// handleDrop is called on Fyne's event thread (SetOnDropped), so the
// filesystem checks (os.Stat, facepack.LooksLikePack) run in a goroutine;
// only the resulting UI/state changes go through doUI.
func (a *App) handleDrop(u fyne.URI) {
	if u == nil || u.Scheme() != "file" {
		return
	}
	path := u.Path()

	go func() {
		info, err := os.Stat(path)
		if err != nil {
			doUI(func() { a.warnf("dropped path not found: %s", path) })
			return
		}

		if !info.IsDir() {
			if strings.EqualFold(filepath.Ext(path), ".rtf") {
				doUI(func() {
					a.rtfRow.SetText(path)
					a.onRTFChanged(path)
				})
				return
			}
			doUI(func() { a.logf("dropped file ignored (not an .rtf export): %s", path) })
			return
		}

		if facepack.LooksLikePack(path, 10) {
			doUI(func() {
				a.packRow.SetText(path)
				if a.current != nil && a.current.Settings.ConfigXML == "" {
					a.configRow.SetText(filepath.Join(path, "config.xml"))
				}
				a.onPackDirChanged(path)
			})
			return
		}

		configCandidate := filepath.Join(path, "config.xml")
		if _, err := os.Stat(configCandidate); err == nil {
			doUI(func() {
				a.configRow.SetText(configCandidate)
				a.onConfigChanged(configCandidate)
			})
			return
		}

		doUI(func() { a.logf("dropped folder ignored (not a face pack, no config.xml): %s", path) })
	}()
}
