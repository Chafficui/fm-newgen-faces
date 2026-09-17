package widgets

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// wizardHandles exposes the interactive pieces of a built wizard, mainly so
// tests can drive it without a full dialog round-trip.
type wizardHandles struct {
	dialog    dialog.Dialog
	indicator *widget.Label
	errLabel  *widget.Label
	backBtn   *widget.Button
	nextBtn   *widget.Button
	cancelBtn *widget.Button
}

func buildWizard(win fyne.Window, title string, steps []WizardStep, onDone func(), onCancel func()) *wizardHandles {
	step := 0

	indicator := widget.NewLabel("")
	stepTitle := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	errLabel := widget.NewLabel("")
	errLabel.Wrapping = fyne.TextWrapWord
	errLabel.Hide()

	body := container.NewStack()

	h := &wizardHandles{
		indicator: indicator,
		errLabel:  errLabel,
		backBtn:   widget.NewButtonWithIcon(T("widgets.wizard.back"), theme.NavigateBackIcon(), nil),
		nextBtn:   widget.NewButtonWithIcon(T("widgets.wizard.next"), theme.NavigateNextIcon(), nil),
		cancelBtn: widget.NewButton(T("widgets.wizard.cancel"), nil),
	}

	render := func() {
		s := steps[step]
		indicator.SetText(T("widgets.wizard.step", step+1, len(steps)))
		stepTitle.SetText(s.Title)
		body.Objects = []fyne.CanvasObject{s.Content}
		body.Refresh()
		errLabel.SetText("")
		errLabel.Hide()

		h.backBtn.Disable()
		if step > 0 {
			h.backBtn.Enable()
		}
		if step == len(steps)-1 {
			h.nextBtn.SetText(T("widgets.wizard.finish"))
			h.nextBtn.SetIcon(theme.ConfirmIcon())
		} else {
			h.nextBtn.SetText(T("widgets.wizard.next"))
			h.nextBtn.SetIcon(theme.NavigateNextIcon())
		}
	}

	h.backBtn.OnTapped = func() {
		if step > 0 {
			step--
			render()
		}
	}

	h.nextBtn.OnTapped = func() {
		s := steps[step]
		if s.Validate != nil {
			if err := s.Validate(); err != nil {
				errLabel.Importance = widget.DangerImportance
				errLabel.SetText(err.Error())
				errLabel.Show()
				return
			}
		}
		if step == len(steps)-1 {
			if h.dialog != nil {
				h.dialog.Hide()
			}
			if onDone != nil {
				onDone()
			}
			return
		}
		step++
		render()
	}

	h.cancelBtn.OnTapped = func() {
		if h.dialog != nil {
			h.dialog.Hide()
		}
		if onCancel != nil {
			onCancel()
		}
	}

	header := container.NewVBox(indicator, stepTitle, widget.NewSeparator())
	footer := container.NewVBox(errLabel, container.NewBorder(nil, nil, h.cancelBtn, container.NewHBox(h.backBtn, h.nextBtn)))
	content := container.NewBorder(header, footer, nil, nil, body)

	render()

	custom := dialog.NewCustomWithoutButtons(title, content, win)
	custom.Resize(fyne.NewSize(560, 420))
	h.dialog = custom
	return h
}

func showWizard(win fyne.Window, title string, steps []WizardStep, onDone func(), onCancel func()) {
	if len(steps) == 0 {
		return
	}
	buildWizard(win, title, steps, onDone, onCancel).dialog.Show()
}
