package widgets

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// checklistVisibleRows is how many rows must be fully visible without
// scrolling: the 5 fixed checklist rows (pack, config, rtf, version,
// install).
const checklistVisibleRows = 5

// checklistTitleWidth is the fixed width of the bold title column, so the
// detail text that follows it lines up across rows regardless of how long
// each title is.
const checklistTitleWidth = 200

// newChecklist builds an empty checklist. Reserving a fixed minimum height
// up front (rather than sizing to the rows once they arrive) matters
// because the checklist starts life empty with ~0 height; when SetRows
// later fills it in, nothing tells the enclosing VBox (built once, before
// data loads) to redo its layout, so the siblings below it (the path rows,
// version select…) stay at their stale positions and the rows painted here
// overlap them. Wrapping in a scroll with a fixed minimum size keeps the
// checklist's footprint constant from the very first layout pass, so
// nothing needs to be re-laid-out later; any overflow (more rows than fit)
// simply scrolls instead of overlapping. The minimum height is measured
// from a real row (built the same way as any other, including an action
// button, which is the tallest case) so it tracks the current theme's
// metrics instead of a hand-picked pixel count.
func newChecklist() *Checklist {
	c := &Checklist{box: container.NewVBox()}

	sample := c.buildRow(ChecklistRow{Title: "Sample", Detail: "Sample", Action: "Sample", OnAction: func() {}})
	rowHeight := sample.MinSize().Height
	// The rows sit in a VBox, which adds theme.Padding() between every pair
	// of consecutive rows; without that here the reserved height falls a
	// few pixels short of what checklistVisibleRows rows actually need,
	// leaving the last one partly scrolled out of view.
	total := rowHeight*checklistVisibleRows + theme.Padding()*(checklistVisibleRows-1)

	scroll := container.NewVScroll(c.box)
	scroll.SetMinSize(fyne.NewSize(0, total))
	c.content = scroll

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

// buildRow renders one checklist entry as a single line: status icon | bold
// title (fixed width, truncated so it never pushes the detail text out of
// view) | detail (truncated, not wrapped, so the row never grows past one
// line) | optional action button.
func (c *Checklist) buildRow(row ChecklistRow) fyne.CanvasObject {
	icon := statusIcon(row.Status)

	title := widget.NewLabelWithStyle(row.Title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.Truncation = fyne.TextTruncateEllipsis
	titleBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(checklistTitleWidth, title.MinSize().Height)), title)

	detail := widget.NewLabel(row.Detail)
	detail.Truncation = fyne.TextTruncateEllipsis

	var trailing fyne.CanvasObject
	if row.Action != "" && row.OnAction != nil {
		action := row.OnAction
		trailing = widget.NewButton(row.Action, func() { action() })
	}

	left := container.NewHBox(container.NewPadded(icon), titleBox)
	line := container.NewBorder(nil, nil, left, trailing, detail)
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
