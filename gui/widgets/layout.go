package widgets

import "fyne.io/fyne/v2"

// centeredLayout centres a single child horizontally, constraining its
// width to at most maxWidth and letting it shrink below that on a narrower
// window. Height always follows the child's own minimum height, so it works
// naturally inside a container.Scroll.
type centeredLayout struct {
	maxWidth float32
}

func newCenteredLayout(maxWidth float32) fyne.Layout {
	return &centeredLayout{maxWidth: maxWidth}
}

// MinSize implements fyne.Layout.
func (c *centeredLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		min := o.MinSize()
		if min.Width > w {
			w = min.Width
		}
		if min.Height > h {
			h = min.Height
		}
	}
	if w > c.maxWidth {
		w = c.maxWidth
	}
	return fyne.NewSize(w, h)
}

// Layout implements fyne.Layout.
func (c *centeredLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	width := size.Width
	if width > c.maxWidth {
		width = c.maxWidth
	}
	x := (size.Width - width) / 2
	if x < 0 {
		x = 0
	}
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		h := o.MinSize().Height
		o.Resize(fyne.NewSize(width, h))
		o.Move(fyne.NewPos(x, 0))
	}
}
