package widgets

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"fmnewgenfaces/internal/core/assign"
)

const maxSkippedShown = 20

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

		scroll := container.NewVScroll(list)
		scroll.SetMinSize(fyne.NewSize(0, 150))
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
	d.Resize(fyne.NewSize(640, 540))
	d.Show()
}
