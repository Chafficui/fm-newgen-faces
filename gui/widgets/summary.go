package widgets

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"fmnewgenfaces/internal/core/assign"
)

const maxSkippedShown = 20

// Dialog sizing: fixed width, height fit to content between a floor (the
// no-skipped-players case) and a ceiling (a long skipped list scrolls
// instead of growing the dialog past this).
const (
	summaryWidth     = 620
	summaryMinHeight = 380
	summaryMaxHeight = 540
)

// skippedScrollHeight sizes the skipped-players scroll area to the n lines
// it holds (n is already capped at maxSkippedShown), up to a height that
// still leaves room for the rest of the dialog under summaryMaxHeight.
func skippedScrollHeight(n int) float32 {
	const lineHeight float32 = 24
	const capHeight float32 = 200
	h := float32(n) * lineHeight
	if h > capHeight {
		h = capHeight
	}
	if h < lineHeight {
		h = lineHeight
	}
	return h
}

func showSummary(win fyne.Window, plan *assign.Plan, res *assign.Result, backupPath string, actions SummaryActions) {
	if res == nil {
		return
	}

	counts := widget.NewLabel(T("widgets.summary.counts", len(res.Assigned), res.Preserved, len(res.Skipped), res.Unmapped))
	counts.Wrapping = fyne.TextWrapWord

	content := container.NewVBox(counts)

	if len(res.Skipped) > 0 {
		content.Add(widget.NewLabelWithStyle(T("widgets.summary.skipped_header"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))

		list := container.NewVBox()
		n := len(res.Skipped)
		if n > maxSkippedShown {
			n = maxSkippedShown
		}
		for _, s := range res.Skipped[:n] {
			line := widget.NewLabel(T("widgets.summary.skipped_line", s.Player.Name, s.Player.ID, s.Reason))
			line.Wrapping = fyne.TextWrapWord
			list.Add(line)
		}
		if len(res.Skipped) > maxSkippedShown {
			list.Add(widget.NewLabel(T("widgets.summary.skipped_more", len(res.Skipped)-maxSkippedShown)))
		}

		// Size the skipped list to how many lines it actually has, capped so
		// a long list scrolls instead of pushing the dialog past its max
		// height (see the Resize call below).
		scroll := container.NewVScroll(list)
		scroll.SetMinSize(fyne.NewSize(0, skippedScrollHeight(n)))
		content.Add(scroll)
	}

	if backupPath != "" {
		backup := widget.NewLabel(T("widgets.summary.backup", backupPath))
		backup.Wrapping = fyne.TextWrapWord
		content.Add(backup)
	}

	content.Add(widget.NewSeparator())
	content.Add(widget.NewLabelWithStyle(T("widgets.summary.next_steps_header"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	nextSteps := widget.NewLabel(T("widgets.summary.next_steps_body"))
	nextSteps.Wrapping = fyne.TextWrapWord
	content.Add(nextSteps)

	var buttons []fyne.CanvasObject
	if actions.OpenFolder != nil {
		fn := actions.OpenFolder
		buttons = append(buttons, widget.NewButton(T("widgets.summary.btn_open_folder"), func() { fn() }))
	}
	if actions.ShowLog != nil {
		fn := actions.ShowLog
		buttons = append(buttons, widget.NewButton(T("widgets.summary.btn_show_log"), func() { fn() }))
	}
	if actions.Review != nil {
		fn := actions.Review
		buttons = append(buttons, widget.NewButton(T("widgets.summary.btn_review"), func() { fn() }))
	}
	if actions.Undo != nil {
		fn := actions.Undo
		buttons = append(buttons, widget.NewButton(T("widgets.summary.btn_undo"), func() { fn() }))
	}
	if len(buttons) > 0 {
		content.Add(container.NewHBox(buttons...))
	}

	d := dialog.NewCustom(T("widgets.summary.title"), T("widgets.summary.close"), content, win)

	// Size to content (fixed width, height clamped between the no-skipped
	// floor and a ceiling that scrolls a long skipped list) instead of a
	// flat box that leaves empty space when there's little to show.
	size := d.MinSize()
	h := size.Height
	if h < summaryMinHeight {
		h = summaryMinHeight
	}
	if h > summaryMaxHeight {
		h = summaryMaxHeight
	}
	w := size.Width
	if w < summaryWidth {
		w = summaryWidth
	}
	d.Resize(fyne.NewSize(w, h))
	d.Show()
}
