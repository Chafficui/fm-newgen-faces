package widgets

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"fmnewgenfaces/internal/core/assign"
	"fmnewgenfaces/internal/core/rtf"
)

func TestChecklistRendersRowsAndAllOK(t *testing.T) {
	test.NewApp()

	c := NewChecklist()
	rows := []ChecklistRow{
		{Key: "pack", Status: StatusOK, Title: "Face pack", Detail: "14/14 folders"},
		{Key: "config", Status: StatusError, Title: "Config", Detail: "broken", Action: "Fix", OnAction: func() {}},
	}
	c.SetRows(rows)

	if len(c.box.Objects) != len(rows) {
		t.Fatalf("expected %d rendered rows, got %d", len(rows), len(c.box.Objects))
	}
	if c.AllOK() {
		t.Fatalf("expected AllOK() == false with an error row present")
	}

	c.SetRows([]ChecklistRow{{Key: "a", Status: StatusOK, Title: "A", Detail: "ok"}})
	if len(c.box.Objects) != 1 {
		t.Fatalf("expected SetRows to rebuild rows, got %d objects", len(c.box.Objects))
	}
	if !c.AllOK() {
		t.Fatalf("expected AllOK() == true with no error rows")
	}
}

func TestPathRowSetTextVsTyping(t *testing.T) {
	test.NewApp()

	var called int32
	var mu sync.Mutex
	var lastValue string
	_, pr := NewPathRow("Path", "placeholder", nil, nil, func(s string) {
		atomic.AddInt32(&called, 1)
		mu.Lock()
		lastValue = s
		mu.Unlock()
	})

	pr.SetText("/preset/path")
	time.Sleep(400 * time.Millisecond)
	if atomic.LoadInt32(&called) != 0 {
		t.Fatalf("SetText must not fire onChanged, but it fired %d time(s)", called)
	}
	if pr.Text() != "/preset/path" {
		t.Fatalf("SetText did not set the entry text, got %q", pr.Text())
	}

	test.Type(pr.entry, "x")
	if atomic.LoadInt32(&called) != 0 {
		t.Fatalf("typing should not fire onChanged before the debounce window elapses")
	}
	time.Sleep(400 * time.Millisecond)
	if atomic.LoadInt32(&called) != 1 {
		t.Fatalf("expected onChanged to fire exactly once after debounce, got %d", called)
	}
	mu.Lock()
	got := lastValue
	mu.Unlock()
	if got != pr.Text() {
		t.Fatalf("onChanged value %q does not match final entry text %q", got, pr.Text())
	}
}

func TestLogViewKeepsMaxLines(t *testing.T) {
	test.NewApp()

	lv := NewLogView(3)
	for i := 0; i < 5; i++ {
		lv.Append(LogInfo, fmt.Sprintf("line%d", i))
	}

	if lv.lines != 3 {
		t.Fatalf("expected 3 lines retained, got %d", lv.lines)
	}
	if len(lv.rich.Segments) != 3 {
		t.Fatalf("expected 3 rich text segments, got %d", len(lv.rich.Segments))
	}
	first, ok := lv.rich.Segments[0].(*widget.TextSegment)
	if !ok {
		t.Fatalf("expected a *widget.TextSegment")
	}
	if first.Text != "line2\n" {
		t.Fatalf("expected the oldest surviving line to be line2, got %q", first.Text)
	}

	lv.Clear()
	if lv.lines != 0 || len(lv.rich.Segments) != 0 {
		t.Fatalf("expected Clear() to empty the panel")
	}
}

func TestProgressPanelValues(t *testing.T) {
	test.NewApp()

	p := NewProgressPanel()
	p.SetPhase("Scanning")
	p.SetProgress(3, 10)

	if p.phase.Text != "Scanning" {
		t.Fatalf("expected phase text 'Scanning', got %q", p.phase.Text)
	}
	if p.bar.Value != 0.3 {
		t.Fatalf("expected progress bar value 0.3, got %v", p.bar.Value)
	}
	if got, want := p.counts.Text, T("widgets.progress.counts", 3, 10); got != want {
		t.Fatalf("expected counts text %q, got %q", want, got)
	}

	p.SetIndeterminate(true)
	if p.bar.Visible() {
		t.Fatalf("expected the determinate bar to be hidden while indeterminate")
	}
	if !p.inf.Visible() {
		t.Fatalf("expected the infinite bar to be visible while indeterminate")
	}

	p.SetIndeterminate(false)
	if !p.bar.Visible() || p.inf.Visible() {
		t.Fatalf("expected to switch back to the determinate bar")
	}

	p.Reset()
	if p.bar.Value != 0 || p.phase.Text != "" || p.counts.Text != "" {
		t.Fatalf("expected Reset() to clear phase, bar and counts")
	}
}

func TestWizardValidateBlocksNext(t *testing.T) {
	a := test.NewApp()
	win := test.NewWindow(nil)
	defer win.Close()
	_ = a

	steps := []WizardStep{
		{
			Title:   "Step one",
			Content: widget.NewLabel("content"),
			Validate: func() error {
				return errors.New("please fill this in")
			},
		},
		{
			Title:   "Step two",
			Content: widget.NewLabel("content 2"),
		},
	}

	var done, cancelled bool
	h := buildWizard(win, "Wizard", steps, func() { done = true }, func() { cancelled = true })

	if h.indicator.Text != T("widgets.wizard.step", 1, 2) {
		t.Fatalf("expected step indicator for step 1, got %q", h.indicator.Text)
	}

	test.Tap(h.nextBtn)

	if h.indicator.Text != T("widgets.wizard.step", 1, 2) {
		t.Fatalf("Validate error must block advancing, but indicator changed to %q", h.indicator.Text)
	}
	if h.errLabel.Hidden {
		t.Fatalf("expected the validation error label to be shown")
	}
	if h.errLabel.Text != "please fill this in" {
		t.Fatalf("expected the validation error text to be shown, got %q", h.errLabel.Text)
	}
	if done || cancelled {
		t.Fatalf("onDone/onCancel must not fire when validation blocks Next")
	}

	// Once the step validates, Next must be allowed to advance.
	steps[0].Validate = func() error { return nil }
	test.Tap(h.nextBtn)
	if h.indicator.Text != T("widgets.wizard.step", 2, 2) {
		t.Fatalf("expected to advance to step 2, got %q", h.indicator.Text)
	}
}

func TestReviewTableSearchFilters(t *testing.T) {
	test.NewApp()
	win := test.NewWindow(nil)
	defer win.Close()

	rt := NewReviewTable(ReviewConfig{
		ImagePath: func(a assign.Assignment) string { return "" },
		Window:    win,
	})

	rt.SetAssignments([]assign.Assignment{
		{Player: rtf.Player{ID: "1", Name: "Alice Example", Nation: "GER"}, Image: "img1"},
		{Player: rtf.Player{ID: "2", Name: "Bob Sample", Nation: "FRA"}, Image: "img2"},
		{Player: rtf.Player{ID: "3", Name: "Carol Test", Nation: "GER", Nation2: "FRA"}, Image: "img3"},
	})

	if len(rt.visible) != 3 {
		t.Fatalf("expected 3 visible rows with no filter, got %d", len(rt.visible))
	}

	rt.search.SetText("ali")
	if len(rt.visible) != 1 || rt.visible[0].Player.Name != "Alice Example" {
		t.Fatalf("expected name filter to match only Alice, got %#v", rt.visible)
	}

	rt.search.SetText("2")
	if len(rt.visible) != 1 || rt.visible[0].Player.ID != "2" {
		t.Fatalf("expected ID filter to match only player 2, got %#v", rt.visible)
	}

	rt.search.SetText("fra")
	if len(rt.visible) != 2 {
		t.Fatalf("expected nation filter to match 2 rows (primary + secondary), got %d", len(rt.visible))
	}

	rt.search.SetText("")
	if len(rt.visible) != 3 {
		t.Fatalf("expected clearing the search to restore all rows, got %d", len(rt.visible))
	}
}

var _ fyne.CanvasObject = (*reviewRow)(nil)

func TestCenteredLayoutClampsAndCenters(t *testing.T) {
	test.NewApp()

	child := widget.NewLabel("hello")
	l := newCenteredLayout(300)

	// Wider than maxWidth: child is clamped to maxWidth and centred.
	l.Layout([]fyne.CanvasObject{child}, fyne.NewSize(700, 100))
	if child.Size().Width != 300 {
		t.Fatalf("expected width clamped to 300, got %v", child.Size().Width)
	}
	if got, want := child.Position().X, float32(200); got != want {
		t.Fatalf("expected x=%v to centre a 300-wide child in 700, got %v", want, got)
	}

	// Narrower than maxWidth: child shrinks to fill, x=0.
	l.Layout([]fyne.CanvasObject{child}, fyne.NewSize(150, 100))
	if child.Size().Width != 150 {
		t.Fatalf("expected width to shrink to 150, got %v", child.Size().Width)
	}
	if child.Position().X != 0 {
		t.Fatalf("expected x=0 when narrower than maxWidth, got %v", child.Position().X)
	}

	wide := widget.NewLabel("hello")
	wide.Resize(fyne.NewSize(500, 40)) // Resize doesn't change a Label's intrinsic MinSize
	if got, want := l.MinSize([]fyne.CanvasObject{wide}).Width, wide.MinSize().Width; got != want {
		t.Fatalf("expected MinSize width to track the child's own min size (%v), got %v", want, got)
	}
}
