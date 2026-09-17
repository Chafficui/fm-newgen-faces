package widgets

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const pathRowDebounce = 300 * time.Millisecond

func newPathRow(label, placeholder string, onBrowse func() string, onOpen func(), onChanged func(string)) (fyne.CanvasObject, *PathRow) {
	entry := widget.NewEntry()
	entry.SetPlaceHolder(placeholder)

	pr := &PathRow{
		entry:     entry,
		debounce:  pathRowDebounce,
		onChanged: onChanged,
	}
	pr.SetText = pr.setText
	pr.Text = pr.text

	entry.OnChanged = func(text string) {
		if pr.guard {
			return
		}
		if pr.timer != nil {
			pr.timer.Stop()
		}
		pr.timer = time.AfterFunc(pr.debounce, func() {
			fyne.Do(func() {
				if pr.onChanged != nil {
					pr.onChanged(text)
				}
			})
		})
	}

	buttons := []fyne.CanvasObject{}
	if onBrowse != nil {
		browseBtn := widget.NewButtonWithIcon(T("widgets.pathrow.browse"), theme.FolderOpenIcon(), func() {
			path := onBrowse()
			if path == "" {
				return
			}
			pr.setText(path)
			if pr.onChanged != nil {
				pr.onChanged(path)
			}
		})
		buttons = append(buttons, browseBtn)
	}
	if onOpen != nil {
		openBtn := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), onOpen)
		buttons = append(buttons, openBtn)
	}

	lbl := widget.NewLabel(label)
	row := container.NewBorder(nil, nil, lbl, container.NewHBox(buttons...), entry)
	return row, pr
}

func (pr *PathRow) setText(text string) {
	pr.guard = true
	pr.entry.SetText(text)
	pr.guard = false
}

func (pr *PathRow) text() string {
	return pr.entry.Text
}
