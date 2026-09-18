package gui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"fmnewgenfaces/gui/widgets"
	"fmnewgenfaces/internal/i18n"
)

// statusLineRow renders one checklist row as a standalone inline status
// line (icon, detail text, the row's own fix-it action if any, plus any
// extra trailing buttons) directly under the field it describes, in place
// of the old separate checklist block.
func statusLineRow(row widgets.ChecklistRow, extra ...fyne.CanvasObject) fyne.CanvasObject {
	icon := widgets.StatusIcon(row.Status)
	detail := widget.NewLabel(row.Detail)
	detail.Wrapping = fyne.TextWrapWord

	var trailing []fyne.CanvasObject
	if row.Action != "" && row.OnAction != nil {
		action := row.OnAction
		trailing = append(trailing, widget.NewButton(row.Action, func() { action() }))
	}
	trailing = append(trailing, extra...)

	var trailingObj fyne.CanvasObject
	if len(trailing) > 0 {
		trailingObj = container.NewHBox(trailing...)
	}
	return container.NewBorder(nil, nil, container.NewPadded(icon), trailingObj, detail)
}

// findRow returns the row with the given Key, if any.
func findRow(rows []widgets.ChecklistRow, key string) (widgets.ChecklistRow, bool) {
	for _, r := range rows {
		if r.Key == key {
			return r, true
		}
	}
	return widgets.ChecklistRow{}, false
}

// errorReason joins the Title/Detail of every StatusError row, for the
// label shown under the disabled primary button.
func errorReason(rows []widgets.ChecklistRow) (string, bool) {
	var reasons []string
	for _, r := range rows {
		if r.Status == widgets.StatusError {
			reasons = append(reasons, r.Title+": "+r.Detail)
		}
	}
	if len(reasons) == 0 {
		return "", false
	}
	return strings.Join(reasons, "; "), true
}

// disclosure is a minimal collapsible section: a toggle header button plus
// a content pane hidden until expanded. It stands in for widget.Accordion
// because every expand/collapse must also call refreshPage(): Fyne does not
// itself notice that a descendant deep in the tree now needs more (or
// less) height, so nothing would reposition the cards/sections below it.
type disclosure struct {
	btn     *widget.Button
	content fyne.CanvasObject
	open    bool
}

// newDisclosure builds a collapsed section titled label; content starts
// hidden. Call .root() for the object to place in the layout.
func (a *App) newDisclosure(label string, content fyne.CanvasObject) *disclosure {
	content.Hide()
	d := &disclosure{content: content}
	d.btn = widget.NewButtonWithIcon(label, theme.MenuExpandIcon(), func() { d.setOpen(a, !d.open) })
	return d
}

func (d *disclosure) root() fyne.CanvasObject {
	return container.NewVBox(d.btn, d.content)
}

func (d *disclosure) setOpen(a *App, open bool) {
	d.open = open
	if open {
		d.content.Show()
		d.btn.SetIcon(theme.MenuDropDownIcon())
	} else {
		d.content.Hide()
		d.btn.SetIcon(theme.MenuExpandIcon())
	}
	a.refreshPage()
}

// refreshPage re-runs layout from the window's root content down. Needed
// after anything that changes how tall a nested section is (a status line
// appearing, a disclosure opening, the result strip replacing the progress
// panel, …): Fyne only recomputes a container's own children on Refresh,
// it never walks back up to resize ancestors on its own, so without this
// the content below the change stays frozen at its old position and the
// new content is drawn right over it.
func (a *App) refreshPage() {
	if a.win == nil {
		return
	}
	if c := a.win.Content(); c != nil {
		c.Refresh()
	}
}

func (a *App) setStatusBox(box *fyne.Container, obj fyne.CanvasObject) {
	if box == nil {
		return
	}
	box.Objects = []fyne.CanvasObject{obj}
	box.Refresh()
}

// applyChecklistToCards is runEvaluate's UI-thread step: it turns the five
// computeChecklist rows into the inline status lines under each field,
// dims cards 2/3 until the face pack is valid, and enables/disables the
// primary/undo buttons. Must run on the UI thread (via doUI).
func (a *App) applyChecklistToCards(rows []widgets.ChecklistRow, hasPack, hasBackup bool, packDir string) {
	defer a.refreshPage()

	packRow, _ := findRow(rows, "pack")
	configRow, _ := findRow(rows, "config")
	versionRow, _ := findRow(rows, "version")
	installRow, _ := findRow(rows, "install")
	rtfRow, _ := findRow(rows, "rtf")

	packDisplay := packRow
	if packDir == "" {
		packDisplay.Detail = i18n.T("gui.card1.pack_choose_folder")
	}
	var extra []fyne.CanvasObject
	if hasPack {
		extra = append(extra, widget.NewButton(i18n.T("gui.card1.details"), func() { a.showPackDetails() }))
	}
	a.setStatusBox(a.packStatusBox, statusLineRow(packDisplay, extra...))
	a.setStatusBox(a.configStatusBox, statusLineRow(configRow))
	a.setStatusBox(a.versionStatusBox, statusLineRow(versionRow))
	a.setStatusBox(a.installStatusBox, statusLineRow(installRow))
	a.setStatusBox(a.rtfStatusBox, statusLineRow(rtfRow))

	a.setRTFHint(a.watcherArmed())

	packValid := packRow.Status != widgets.StatusError
	a.setCard23Enabled(packValid)

	if a.isRunning() {
		// setBusy owns primary/undo enablement while a run/preview is in
		// flight; don't fight it with a debounced evaluate() landing mid-run.
		return
	}

	if !packValid {
		a.primaryReasonLabel.SetText(packDisplay.Detail)
		a.primaryReasonLabel.Show()
		return
	}

	if reason, blocked := errorReason(rows); blocked {
		a.primaryBtn.Disable()
		a.primaryReasonLabel.SetText(reason)
		a.primaryReasonLabel.Show()
	} else {
		a.primaryBtn.Enable()
		a.primaryReasonLabel.Hide()
	}

	if hasBackup {
		a.undoBtn.Enable()
	} else {
		a.undoBtn.Disable()
	}
}

// setCard23Enabled dims Card 2 (newgen export) and Card 3 (assign faces)
// until the face pack is valid: there is nothing useful to do with an RTF
// export or run options before that.
func (a *App) setCard23Enabled(enabled bool) {
	if enabled {
		a.rtfRow.Enable()
		a.preserveCheck.Enable()
		a.allowDupCheck.Enable()
		a.overridesBtn.Enable()
		return
	}
	a.rtfRow.Disable()
	a.preserveCheck.Disable()
	a.allowDupCheck.Disable()
	a.overridesBtn.Disable()
	a.primaryBtn.Disable()
	a.undoBtn.Disable()
}

// showPackDetails opens the full per-folder breakdown (the old always-
// visible pack table) in a dialog, on demand.
func (a *App) showPackDetails() {
	d := dialog.NewCustom(i18n.T("gui.card1.details_title"), i18n.T("common.close"), a.packTable, a.win)
	d.Resize(fyne.NewSize(640, 420))
	d.Show()
}

// toggleLog is Ctrl+L: open the bottom log section if it is collapsed,
// collapse it otherwise.
func (a *App) toggleLog() {
	if a.logDisclosure == nil {
		return
	}
	a.logDisclosure.setOpen(a, !a.logDisclosure.open)
}
