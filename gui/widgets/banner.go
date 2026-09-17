package widgets

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func newUpdateBanner(version string, onOpen func(), onSkip func()) fyne.CanvasObject {
	bg := canvas.NewRectangle(theme.Color(theme.ColorNamePrimary))

	msg := widget.NewLabel(T("widgets.banner.message", version))
	msg.Wrapping = fyne.TextWrapWord

	var bar *fyne.Container

	skip := widget.NewButton(T("widgets.banner.skip"), func() {
		if onSkip != nil {
			onSkip()
		}
		bar.Hide()
	})
	download := widget.NewButtonWithIcon(T("widgets.banner.download"), theme.DownloadIcon(), func() {
		if onOpen != nil {
			onOpen()
		}
	})
	download.Importance = widget.HighImportance

	row := container.NewBorder(nil, nil, nil, container.NewHBox(download, skip), msg)
	bar = container.NewStack(bg, container.NewPadded(row))
	return bar
}
