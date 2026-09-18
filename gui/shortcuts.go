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

	add(fyne.KeyR, func() {
		if a.isRunning() {
			return
		}
		a.startRun(false)
	})
	add(fyne.KeyO, a.actionBrowsePack)
	add(fyne.KeyL, a.toggleLog)
	add(fyne.KeyQ, a.onClose)
}
