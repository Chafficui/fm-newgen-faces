package widgets

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func newProgressPanel() *ProgressPanel {
	p := &ProgressPanel{
		phase:  widget.NewLabel(""),
		bar:    widget.NewProgressBar(),
		inf:    widget.NewProgressBarInfinite(),
		counts: widget.NewLabel(""),
	}
	p.inf.Hide()

	p.content = container.NewVBox(p.phase, container.NewStack(p.bar, p.inf), p.counts)
	p.ExtendBaseWidget(p)
	return p
}

func (p *ProgressPanel) setPhase(text string) {
	fyne.Do(func() {
		p.phase.SetText(text)
		p.Refresh()
	})
}

func (p *ProgressPanel) setProgress(done, total int) {
	fyne.Do(func() {
		ratio := 0.0
		if total > 0 {
			ratio = float64(done) / float64(total)
		}
		p.bar.SetValue(ratio)
		p.counts.SetText(T("widgets.progress.counts", done, total))
		p.Refresh()
	})
}

func (p *ProgressPanel) setIndeterminate(on bool) {
	fyne.Do(func() {
		if on {
			p.bar.Hide()
			p.inf.Show()
			p.inf.Start()
		} else {
			p.inf.Stop()
			p.inf.Hide()
			p.bar.Show()
		}
		p.Refresh()
	})
}

func (p *ProgressPanel) reset() {
	fyne.Do(func() {
		p.inf.Stop()
		p.inf.Hide()
		p.bar.Show()
		p.bar.SetValue(0)
		p.phase.SetText("")
		p.counts.SetText("")
		p.Refresh()
	})
}
