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

	scroll := container.NewVScroll(form)
	scroll.SetMinSize(fyne.NewSize(420, 360))

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
	d.Resize(fyne.NewSize(460, 440))
	d.Show()
}
