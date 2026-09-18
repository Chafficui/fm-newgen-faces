package widgets

import (
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type unmappedRow struct {
	code string
	sel  *widget.Select
}

func showUnmappedResolver(win fyne.Window, counts map[string]int, ethnicNames []string, onSave func(map[string]string), onSkip func()) {
	codes := make([]string, 0, len(counts))
	for c := range counts {
		codes = append(codes, c)
	}
	sort.Slice(codes, func(i, j int) bool {
		if counts[codes[i]] != counts[codes[j]] {
			return counts[codes[i]] > counts[codes[j]]
		}
		return codes[i] < codes[j]
	})

	rows := make([]unmappedRow, 0, len(codes))
	form := widget.NewForm()
	for _, c := range codes {
		sel := widget.NewSelect(ethnicNames, nil)
		rows = append(rows, unmappedRow{code: c, sel: sel})
		form.Append(T("widgets.unmapped.row_label", c, counts[c]), sel)
	}

	// Cap the form to at most unmappedMaxRows rows worth of height instead
	// of a flat min size, so a couple of unmapped codes don't sit above a
	// large empty area; a longer list scrolls instead of growing forever.
	scroll := container.NewVScroll(form)
	scroll.SetMinSize(fyne.NewSize(unmappedFormWidth, unmappedFormHeight(len(codes))))

	d := dialog.NewCustomConfirm(T("widgets.unmapped.title"), T("widgets.unmapped.save"), T("widgets.unmapped.skip"), scroll, func(ok bool) {
		if ok {
			result := make(map[string]string)
			for _, r := range rows {
				if r.sel.Selected != "" {
					result[r.code] = r.sel.Selected
				}
			}
			if onSave != nil {
				onSave(result)
			}
		} else if onSkip != nil {
			onSkip()
		}
	}, win)

	// Size to content instead of a flat box, with a floor so the dialog
	// never gets uncomfortably small for one or two codes.
	size := d.MinSize()
	h := size.Height
	if h < unmappedMinHeight {
		h = unmappedMinHeight
	}
	w := size.Width
	if w < unmappedWidth {
		w = unmappedWidth
	}
	d.Resize(fyne.NewSize(w, h))
	d.Show()
}

const (
	unmappedFormWidth = 420
	unmappedMaxRows   = 8
	unmappedWidth     = 460
	unmappedMinHeight = 320
)

// unmappedFormHeight sizes the scroll area to up to unmappedMaxRows form
// rows, measured from a real form row so it tracks the current theme
// instead of a hand-picked pixel count.
func unmappedFormHeight(rowCount int) float32 {
	visible := rowCount
	if visible > unmappedMaxRows {
		visible = unmappedMaxRows
	}
	if visible < 1 {
		visible = 1
	}
	sample := widget.NewForm(widget.NewFormItem("Sample", widget.NewSelect(nil, nil)))
	rowHeight := sample.MinSize().Height
	return float32(visible) * rowHeight
}
