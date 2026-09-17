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

func (a *App) handleDrop(u fyne.URI) {
	if u == nil || u.Scheme() != "file" {
		return
	}
	path := u.Path()
	info, err := os.Stat(path)
	if err != nil {
		a.warnf("dropped path not found: %s", path)
		return
	}

	if !info.IsDir() {
		if strings.EqualFold(filepath.Ext(path), ".rtf") {
			a.rtfRow.SetText(path)
			a.onRTFChanged(path)
			return
		}
		a.logf("dropped file ignored (not an .rtf export): %s", path)
		return
	}

	if facepack.LooksLikePack(path, 10) {
		a.packRow.SetText(path)
		if a.current != nil && a.current.Settings.ConfigXML == "" {
			a.configRow.SetText(filepath.Join(path, "config.xml"))
		}
		a.onPackDirChanged(path)
		return
	}

	configCandidate := filepath.Join(path, "config.xml")
	if _, err := os.Stat(configCandidate); err == nil {
		a.configRow.SetText(configCandidate)
		a.onConfigChanged(configCandidate)
		return
	}

	a.logf("dropped folder ignored (not a face pack, no config.xml): %s", path)
}
