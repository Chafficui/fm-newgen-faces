package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// setupShortcuts wires the window-wide keyboard shortcuts.
func (a *App) setupShortcuts() {
	canvas := a.win.Canvas()

	add := func(key fyne.KeyName, handler func()) {
		canvas.AddShortcut(&desktop.CustomShortcut{KeyName: key, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) {
			handler()
		})
	}

	add(fyne.KeyR, func() { a.startRun(false) })
	add(fyne.KeyP, func() { a.startRun(true) })
	add(fyne.KeyO, a.actionBrowsePack)
	add(fyne.Key1, func() { a.tabs.SelectIndex(0) })
	add(fyne.Key2, func() { a.tabs.SelectIndex(1) })
	add(fyne.Key3, func() { a.tabs.SelectIndex(2) })
	add(fyne.Key4, func() { a.tabs.SelectIndex(3) })
	add(fyne.KeyQ, a.onClose)
}
