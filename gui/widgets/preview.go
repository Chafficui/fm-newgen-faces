package widgets

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"fmnewgenfaces/internal/core/assign"
	"fmnewgenfaces/internal/core/ethnic"
)

type previewRow struct {
	group, needed, available, shortfall string
}

func previewHeaders() []string {
	return []string{
		T("widgets.preview.col_group"),
		T("widgets.preview.col_needed"),
		T("widgets.preview.col_available"),
		T("widgets.preview.col_shortfall"),
	}
}

func showPreview(win fyne.Window, plan *assign.Plan, confirmLabel string, onConfirm func(), onClose func()) {
	if plan == nil {
		return
	}

	summary := widget.NewLabel(T("widgets.preview.summary", len(plan.New), len(plan.Preserved), len(plan.Unmapped)))
	summary.Wrapping = fyne.TextWrapWord

	var rows []previewRow
	totalShortfall := 0
	for _, e := range ethnic.All {
		d := plan.PerEthnic[e]
		if d == nil {
			continue
		}
		sf := d.Shortfall(plan.Options.AllowDuplicates)
		totalShortfall += sf
		rows = append(rows, previewRow{
			group:     string(e),
			needed:    fmt.Sprintf("%d", d.Needed),
			available: fmt.Sprintf("%d", d.Available),
			shortfall: fmt.Sprintf("%d", sf),
		})
	}

	table := widget.NewTable(
		func() (int, int) { return len(rows), 4 },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			lbl := o.(*widget.Label)
			if id.Row < 0 || id.Row >= len(rows) {
				lbl.SetText("")
				return
			}
			r := rows[id.Row]
			switch id.Col {
			case 0:
				lbl.SetText(r.group)
			case 1:
				lbl.SetText(r.needed)
			case 2:
				lbl.SetText(r.available)
			case 3:
				lbl.SetText(r.shortfall)
			}
		},
	)
	table.ShowHeaderRow = true
	table.CreateHeader = func() fyne.CanvasObject {
		return widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	table.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		headers := previewHeaders()
		lbl := o.(*widget.Label)
		if id.Col >= 0 && id.Col < len(headers) {
			lbl.SetText(headers[id.Col])
		}
	}
	table.SetColumnWidth(0, 160)
	table.SetColumnWidth(1, 100)
	table.SetColumnWidth(2, 100)
	table.SetColumnWidth(3, 100)

	// Cap the table to at most previewMaxRows rows worth of height instead
	// of letting it stretch to fill whatever the dialog is given: a
	// two-row result would otherwise sit above a large empty table area.
	// Put it in a fixed-size GridWrap (rather than leaving it as a Border
	// center) so it keeps this height even inside a container that would
	// normally stretch it.
	tableWrap := container.NewGridWrap(fyne.NewSize(previewTableWidth, previewTableHeight(len(rows))), table)

	var items []fyne.CanvasObject
	items = append(items, summary, widget.NewSeparator(), tableWrap)

	if totalShortfall > 0 {
		warn := widget.NewLabel(T("widgets.preview.warn_shortfall", totalShortfall))
		warn.Wrapping = fyne.TextWrapWord
		items = append(items, container.NewHBox(widget.NewIcon(theme.NewColoredResource(theme.WarningIcon(), theme.ColorNameWarning)), warn))
	}
	if len(plan.Unmapped) > 0 {
		warn := widget.NewLabel(T("widgets.preview.warn_unmapped", len(plan.Unmapped)))
		warn.Wrapping = fyne.TextWrapWord
		items = append(items, container.NewHBox(widget.NewIcon(theme.NewColoredResource(theme.WarningIcon(), theme.ColorNameWarning)), warn))
	}

	content := container.NewVBox(items...)

	d := dialog.NewCustomConfirm(T("widgets.preview.title"), confirmLabel, T("widgets.preview.cancel"), content, func(ok bool) {
		if ok && onConfirm != nil {
			onConfirm()
		}
	}, win)
	// onClose fires once the dialog closes however that happened (confirm,
	// cancel or escape); the caller uses it to know when it is safe to
	// start another run.
	if onClose != nil {
		d.SetOnClosed(onClose)
	}

	// Size to content instead of a flat box, with a floor so the dialog
	// never gets uncomfortably small when there's almost nothing to show.
	size := d.MinSize()
	h := size.Height
	if h < previewMinHeight {
		h = previewMinHeight
	}
	w := size.Width
	if w < previewWidth {
		w = previewWidth
	}
	d.Resize(fyne.NewSize(w, h))
	d.Show()
}

const (
	previewTableWidth = 480
	previewMaxRows    = 10
	previewWidth      = 700
	previewMinHeight  = 320
)

// previewTableHeight sizes the table to its header plus up to
// previewMaxRows data rows, measured from real header/cell labels so it
// tracks the current theme instead of a hand-picked pixel count.
func previewTableHeight(rowCount int) float32 {
	visible := rowCount
	if visible > previewMaxRows {
		visible = previewMaxRows
	}
	if visible < 1 {
		visible = 1
	}
	headerHeight := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}).MinSize().Height
	rowHeight := widget.NewLabel("").MinSize().Height
	sep := theme.Padding()
	return headerHeight + sep + float32(visible)*(rowHeight+sep)
}
