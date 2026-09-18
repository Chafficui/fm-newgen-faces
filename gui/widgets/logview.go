package widgets

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func newLogView(maxLines int) *LogView {
	rich := widget.NewRichText()
	rich.Wrapping = fyne.TextWrapWord

	scroll := container.NewVScroll(rich)

	l := &LogView{rich: rich, scroll: scroll, maxLines: maxLines}
	l.ExtendBaseWidget(l)
	return l
}

func logColorName(level LogLevel) fyne.ThemeColorName {
	switch level {
	case LogWarn:
		return theme.ColorNameWarning
	case LogError:
		return theme.ColorNameError
	default:
		return theme.ColorNameForeground
	}
}

func (l *LogView) appendLine(level LogLevel, line string) {
	fyne.Do(func() {
		l.mu.Lock()
		defer l.mu.Unlock()

		seg := &widget.TextSegment{
			Text:  line + "\n",
			Style: widget.RichTextStyle{ColorName: logColorName(level)},
		}
		l.rich.Segments = append(l.rich.Segments, seg)
		l.lines++
		if l.maxLines > 0 && l.lines > l.maxLines {
			drop := l.lines - l.maxLines
			l.rich.Segments = l.rich.Segments[drop:]
			l.lines = l.maxLines
		}
		l.rich.Refresh()
		if sc, ok := l.scroll.(*container.Scroll); ok {
			sc.ScrollToBottom()
		}
		l.Refresh()
	})
}

func (l *LogView) clear() {
	fyne.Do(func() {
		l.mu.Lock()
		defer l.mu.Unlock()

		l.rich.Segments = nil
		l.lines = 0
		l.rich.Refresh()
		l.Refresh()
	})
}
