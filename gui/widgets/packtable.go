package widgets

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"fmnewgenfaces/internal/core/ethnic"
	"fmnewgenfaces/internal/core/facepack"
)

type packTableRow struct {
	group, status, count, note string
}

func packTableHeaders() []string {
	return []string{
		T("widgets.packtable.col_group"),
		T("widgets.packtable.col_status"),
		T("widgets.packtable.col_images"),
		T("widgets.packtable.col_note"),
	}
}

func newPackTable() *PackTable {
	t := &PackTable{}

	table := widget.NewTable(
		func() (int, int) { return len(t.rows), 4 },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			lbl := o.(*widget.Label)
			if id.Row < 0 || id.Row >= len(t.rows) {
				lbl.SetText("")
				return
			}
			r := t.rows[id.Row]
			switch id.Col {
			case 0:
				lbl.SetText(r.group)
			case 1:
				lbl.SetText(r.status)
			case 2:
				lbl.SetText(r.count)
			case 3:
				lbl.SetText(r.note)
			}
		},
	)
	table.ShowHeaderRow = true
	table.CreateHeader = func() fyne.CanvasObject {
		return widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	table.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		headers := packTableHeaders()
		lbl := o.(*widget.Label)
		if id.Col >= 0 && id.Col < len(headers) {
			lbl.SetText(headers[id.Col])
		}
	}
	table.SetColumnWidth(0, 150)
	table.SetColumnWidth(1, 90)
	table.SetColumnWidth(2, 90)
	table.SetColumnWidth(3, 280)

	total := widget.NewLabel("")

	t.table = table
	t.total = total
	t.content = container.NewBorder(nil, container.NewPadded(total), nil, nil, table)
	t.ExtendBaseWidget(t)
	return t
}

func (t *PackTable) setPack(p *facepack.Pack) {
	t.rows = nil
	if p == nil {
		t.total.SetText("")
		t.table.Refresh()
		t.Refresh()
		return
	}

	for _, e := range ethnic.All {
		f := p.Folders[e]
		row := packTableRow{group: string(e)}
		if f == nil || !f.Exists {
			row.status = T("widgets.packtable.status_missing")
			row.count = "—"
			row.note = T("widgets.packtable.note_missing")
			t.rows = append(t.rows, row)
			continue
		}

		n := len(f.Images)
		row.count = fmt.Sprintf("%d", n)
		if n == 0 {
			row.status = T("widgets.packtable.status_empty")
		} else {
			row.status = T("widgets.packtable.status_ok")
		}

		var notes []string
		if len(f.Ignored) > 0 {
			notes = append(notes, T("widgets.packtable.note_ignored", len(f.Ignored)))
		}
		if f.NestedDirs > 0 {
			notes = append(notes, T("widgets.packtable.note_nested", f.NestedDirs))
		}
		row.note = strings.Join(notes, "; ")
		t.rows = append(t.rows, row)
	}

	t.total.SetText(T("widgets.packtable.total", p.TotalImages))
	t.table.Refresh()
	t.Refresh()
}
