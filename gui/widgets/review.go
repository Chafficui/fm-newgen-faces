package widgets

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"fmnewgenfaces/internal/core/assign"
)

const thumbSize = 48

// reviewRow is a recycled list row. It must be a real widget (not just a
// *fyne.Container wrapper) because the driver's paint walk only descends
// into *fyne.Container or fyne.Widget values by concrete/interface type;
// a struct that merely embeds *fyne.Container (rather than being one) is
// neither, so its children would never be painted even though widget.List
// places it directly as a row.
type reviewRow struct {
	widget.BaseWidget
	content *fyne.Container
	thumb   *canvas.Image
	name    *widget.Label
	detail  *widget.Label
	reroll  *widget.Button
	path    string // the image path currently loading/loaded into thumb
}

// CreateRenderer implements fyne.Widget.
func (r *reviewRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.content)
}

func newReviewTable(cfg ReviewConfig) *ReviewTable {
	t := &ReviewTable{cfg: cfg, thumbs: make(map[string]fyne.Resource)}

	t.search = widget.NewEntry()
	t.search.SetPlaceHolder(T("widgets.review.search_placeholder"))
	t.search.OnChanged = func(string) { t.applyFilter() }

	t.list = widget.NewList(
		func() int { return len(t.visible) },
		func() fyne.CanvasObject { return t.newRowWidget() },
		func(id widget.ListItemID, o fyne.CanvasObject) { t.updateRow(id, o) },
	)

	t.content = container.NewBorder(t.search, nil, nil, nil, t.list)
	t.ExtendBaseWidget(t)
	return t
}

func (t *ReviewTable) newRowWidget() *reviewRow {
	thumb := canvas.NewImageFromResource(theme.AccountIcon())
	thumb.FillMode = canvas.ImageFillContain
	thumb.SetMinSize(fyne.NewSize(thumbSize, thumbSize))

	name := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	detail := widget.NewLabel("")
	detail.Wrapping = fyne.TextWrapWord
	text := container.NewVBox(name, detail)

	reroll := widget.NewButton(T("widgets.review.reroll"), nil)

	c := container.NewBorder(nil, nil, thumb, reroll, text)
	row := &reviewRow{content: c, thumb: thumb, name: name, detail: detail, reroll: reroll}
	row.ExtendBaseWidget(row)
	return row
}

func (t *ReviewTable) updateRow(id widget.ListItemID, o fyne.CanvasObject) {
	row := o.(*reviewRow)
	if id < 0 || id >= len(t.visible) {
		return
	}
	a := t.visible[id]

	row.name.SetText(a.Player.Name)
	nations := a.Player.Nation
	if a.Player.Nation2 != "" {
		nations += "/" + a.Player.Nation2
	}
	row.detail.SetText(T("widgets.review.detail", a.Player.ID, nations, string(a.Player.Ethnic), a.Image))

	t.renderMu.Lock()
	row.thumb.Resource = theme.AccountIcon()
	row.thumb.Image = nil
	row.thumb.Refresh()
	t.renderMu.Unlock()
	t.loadThumb(a, row)

	row.reroll.Enable()
	row.reroll.OnTapped = func() { t.doReroll(a, row) }
	row.Refresh()
}

// loadThumb resolves a's thumbnail off the UI thread and applies it (via
// fyne.Do) only if the row has not since been recycled to a different image.
func (t *ReviewTable) loadThumb(a assign.Assignment, row *reviewRow) {
	if t.cfg.ImagePath == nil {
		return
	}
	path := t.cfg.ImagePath(a)
	row.path = path
	if path == "" {
		return
	}

	t.thumbMu.Lock()
	cached, ok := t.thumbs[path]
	t.thumbMu.Unlock()
	if ok {
		t.renderMu.Lock()
		row.thumb.Resource = cached
		row.thumb.Image = nil
		row.thumb.Refresh()
		t.renderMu.Unlock()
		row.Refresh()
		return
	}

	go func() {
		res, err := fyne.LoadResourceFromPath(path)
		if err != nil {
			return
		}
		t.thumbMu.Lock()
		t.thumbs[path] = res
		t.thumbMu.Unlock()
		fyne.Do(func() {
			if row.path != path {
				return // recycled to a different row while loading
			}
			t.renderMu.Lock()
			row.thumb.Resource = res
			row.thumb.Image = nil
			row.thumb.Refresh()
			t.renderMu.Unlock()
			row.Refresh()
		})
	}()
}

func (t *ReviewTable) doReroll(a assign.Assignment, row *reviewRow) {
	if t.cfg.Reroll == nil {
		return
	}
	row.reroll.Disable()
	player := a.Player
	go func() {
		newA, err := t.cfg.Reroll(player)
		fyne.Do(func() {
			row.reroll.Enable()
			if err != nil {
				dialog.ShowError(err, t.cfg.Window)
				return
			}
			t.replaceAssignment(player.ID, newA)
		})
	}()
}

func (t *ReviewTable) replaceAssignment(playerID string, newA assign.Assignment) {
	for i := range t.all {
		if t.all[i].Player.ID == playerID {
			t.all[i] = newA
			break
		}
	}
	for i := range t.visible {
		if t.visible[i].Player.ID == playerID {
			t.visible[i] = newA
			break
		}
	}
	t.list.Refresh()
	t.Refresh()
}

func (t *ReviewTable) setAssignments(rows []assign.Assignment) {
	t.all = rows
	t.applyFilter()
}

func (t *ReviewTable) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(t.search.Text))
	visible := make([]assign.Assignment, 0, len(t.all))
	for _, a := range t.all {
		if q == "" || matchesReviewQuery(a, q) {
			visible = append(visible, a)
		}
	}
	t.visible = visible
	t.list.Refresh()
	t.Refresh()
}

func matchesReviewQuery(a assign.Assignment, q string) bool {
	hay := strings.ToLower(a.Player.Name + " " + a.Player.ID + " " + a.Player.Nation + " " + a.Player.Nation2)
	return strings.Contains(hay, q)
}
