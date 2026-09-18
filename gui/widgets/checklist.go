package widgets

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// checklistMinHeight reserves enough room for the 5 fixed checklist rows
// (pack, config, rtf, version, install) up front. Without this, the
// checklist starts life empty with ~0 height; when SetRows later fills it
// in, nothing tells the enclosing VBox (built once, before data loads) to
// redo its layout, so the siblings below it (the path rows, version
// select…) stay at their stale positions and the rows painted here overlap
// them. Wrapping in a scroll with a fixed minimum size keeps the
// checklist's footprint constant from the very first layout pass, so nothing
// needs to be re-laid-out later; any overflow (more rows than fit) simply
// scrolls instead of overlapping.
const checklistMinHeight = 420

func newChecklist() *Checklist {
	box := container.NewVBox()
	scroll := container.NewVScroll(box)
	scroll.SetMinSize(fyne.NewSize(0, checklistMinHeight))

	c := &Checklist{box: box, content: scroll}
	c.ExtendBaseWidget(c)
	return c
}

// statusIcon returns a themed, status-tinted icon that stays readable in
// both light and dark themes.
func statusIcon(s Status) *widget.Icon {
	var res fyne.Resource
	switch s {
	case StatusOK:
		res = theme.NewColoredResource(theme.ConfirmIcon(), theme.ColorNameSuccess)
	case StatusWarn:
		res = theme.NewColoredResource(theme.WarningIcon(), theme.ColorNameWarning)
	case StatusError:
		res = theme.NewColoredResource(theme.ErrorIcon(), theme.ColorNameError)
	default:
		res = theme.NewColoredResource(theme.RadioButtonIcon(), theme.ColorNameDisabled)
	}
	return widget.NewIcon(res)
}

func (c *Checklist) buildRow(row ChecklistRow) fyne.CanvasObject {
	icon := statusIcon(row.Status)
	title := widget.NewLabelWithStyle(row.Title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	detail := widget.NewLabel(row.Detail)
	detail.Wrapping = fyne.TextWrapWord
	text := container.NewVBox(title, detail)

	var trailing fyne.CanvasObject
	if row.Action != "" && row.OnAction != nil {
		action := row.OnAction
		trailing = widget.NewButton(row.Action, func() { action() })
	}

	line := container.NewBorder(nil, nil, container.NewPadded(icon), trailing, text)
	return container.NewVBox(line, widget.NewSeparator())
}

func (c *Checklist) setRows(rows []ChecklistRow) {
	c.rows = rows
	objects := make([]fyne.CanvasObject, 0, len(rows))
	for _, row := range rows {
		objects = append(objects, c.buildRow(row))
	}
	c.box.Objects = objects
	c.box.Refresh()
	c.Refresh()
}

func (c *Checklist) allOK() bool {
	for _, r := range c.rows {
		if r.Status == StatusError {
			return false
		}
	}
	return true
}
