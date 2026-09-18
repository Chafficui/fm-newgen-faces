// Package widgets holds reusable, data-in/callbacks-out Fyne components for
// the FM NewGen Faces GUI. Nothing here touches profiles, files or the
// pipeline directly; the gui package wires them together.
package widgets

import (
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"fmnewgenfaces/internal/core/assign"
	"fmnewgenfaces/internal/core/facepack"
	"fmnewgenfaces/internal/core/rtf"
)

// T translates UI strings; the gui package sets this to i18n.T at startup.
var T = func(key string, args ...any) string {
	if len(args) == 0 {
		return key
	}
	return fmt.Sprintf(key, args...)
}

// Status of a checklist row.
type Status int

const (
	StatusPending Status = iota // grey: not checked yet
	StatusOK                    // green
	StatusWarn                  // yellow: usable but with caveats
	StatusError                 // red: blocks running
)

// ChecklistRow is one line of the setup checklist.
type ChecklistRow struct {
	Key    string // stable identifier ("pack", "config", "rtf", "version")
	Status Status
	Title  string // "Face pack"
	Detail string // "14/14 folders, 96,120 images" or "Not found – export it in FM"
	// Action, when non-empty, renders a button with this label calling OnAction.
	Action   string
	OnAction func()
}

// Checklist renders rows with a status icon, title, detail and optional action.
type Checklist struct {
	widget.BaseWidget
	// unexported
	rows    []ChecklistRow
	box     *fyne.Container
	content fyne.CanvasObject
}

// CreateRenderer implements fyne.Widget.
func (c *Checklist) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.content)
}

// NewChecklist creates an empty checklist.
func NewChecklist() *Checklist { return newChecklist() }

// SetRows replaces all rows and refreshes. Safe to call from fyne.Do.
func (c *Checklist) SetRows(rows []ChecklistRow) { c.setRows(rows) }

// AllOK reports whether no row is StatusError.
func (c *Checklist) AllOK() bool { return c.allOK() }

// PackTable shows per-ethnic folder status for a scanned pack.
type PackTable struct {
	widget.BaseWidget
	// unexported
	table   *widget.Table
	total   *widget.Label
	rows    []packTableRow
	content fyne.CanvasObject
}

// CreateRenderer implements fyne.Widget.
func (t *PackTable) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.content)
}

// NewPackTable creates the table (columns: group, status, images, note).
func NewPackTable() *PackTable { return newPackTable() }

// SetPack fills the table; nil clears it.
func (t *PackTable) SetPack(p *facepack.Pack) { t.setPack(p) }

// ShowPreview opens the dry-run dialog: totals (new / preserved / unmapped),
// a per-ethnic table (needed / available / shortfall) and warnings. The
// confirm button label is confirmLabel; onConfirm runs when pressed. onClose
// runs once the dialog closes, regardless of how (confirm, cancel or
// escape), after onConfirm when that ran; either may be nil.
func ShowPreview(win fyne.Window, plan *assign.Plan, confirmLabel string, onConfirm func(), onClose func()) {
	showPreview(win, plan, confirmLabel, onConfirm, onClose)
}

// SummaryActions are the buttons offered after a run; nil funcs hide a button.
type SummaryActions struct {
	OpenFolder func()
	ShowLog    func()
	Undo       func()
	Review     func()
}

// ShowSummary opens the post-run dialog with counts, skipped reasons, the
// backup path, "what to do next in FM" text and the action buttons.
func ShowSummary(win fyne.Window, plan *assign.Plan, res *assign.Result, backupPath string, actions SummaryActions) {
	showSummary(win, plan, res, backupPath, actions)
}

// ShowUnmappedResolver lets the user pick an ethnic group for each unknown
// nation code (counts shown), then calls onSave with code->ethnic for the
// rows that were set. onSkip continues without them. ethnicNames feeds the
// dropdowns.
func ShowUnmappedResolver(win fyne.Window, counts map[string]int, ethnicNames []string, onSave func(map[string]string), onSkip func()) {
	showUnmappedResolver(win, counts, ethnicNames, onSave, onSkip)
}

// OverrideEditor edits nation→ethnic overrides with a validated code picker
// (autocomplete over knownCodes), showing the default for each code, plus
// import/export of a TOML "[mapping_override]" block via callbacks.
type OverrideEditor struct {
	widget.BaseWidget
	// unexported
	cfg     OverrideEditorConfig
	list    *fyne.Container
	content fyne.CanvasObject
}

// CreateRenderer implements fyne.Widget.
func (e *OverrideEditor) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(e.content)
}

// OverrideEditorConfig wires the editor to its owner.
type OverrideEditorConfig struct {
	Get        func() map[string]string
	Set        func(map[string]string)
	KnownCodes []string
	// DefaultFor returns the built-in mapping for a code ("" if unknown).
	DefaultFor  func(code string) string
	EthnicNames []string
	// Import/Export handle the TOML text; Import returns an error to display.
	Import func(text string) error
	Export func() string
	Window fyne.Window
}

// NewOverrideEditor builds the editor.
func NewOverrideEditor(cfg OverrideEditorConfig) *OverrideEditor { return newOverrideEditor(cfg) }

// Refresh re-reads Get() and redraws.
func (e *OverrideEditor) Refresh() { e.refresh() }

// ReviewTable lists assignments with a thumbnail, player, nation, group and
// a Reroll button; a search box filters by name/ID/nation.
type ReviewTable struct {
	widget.BaseWidget
	// unexported
	cfg     ReviewConfig
	search  *widget.Entry
	list    *widget.List
	all     []assign.Assignment
	visible []assign.Assignment
	thumbMu sync.Mutex
	thumbs  map[string]fyne.Resource
	// renderMu serializes canvas.Image mutation/decode across rows: each
	// thumbnail finishes loading on its own goroutine (via fyne.Do), and
	// without this Fyne's decode path can be entered concurrently for
	// different rows, which it is not designed to tolerate.
	renderMu sync.Mutex
	content  fyne.CanvasObject
}

// CreateRenderer implements fyne.Widget.
func (t *ReviewTable) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.content)
}

// ReviewConfig wires the table.
type ReviewConfig struct {
	// ImagePath returns the absolute file path for an assignment's image ("" if unknown).
	ImagePath func(a assign.Assignment) string
	// Reroll replaces the image and returns the new assignment (or error).
	Reroll func(p rtf.Player) (assign.Assignment, error)
	Window fyne.Window
}

// NewReviewTable builds the table.
func NewReviewTable(cfg ReviewConfig) *ReviewTable { return newReviewTable(cfg) }

// SetAssignments replaces the rows.
func (t *ReviewTable) SetAssignments(rows []assign.Assignment) { t.setAssignments(rows) }

// LogLevel colours log lines.
type LogLevel int

const (
	LogInfo LogLevel = iota
	LogWarn
	LogError
)

// LogView is a scrolling, bounded, colour-coded log panel.
type LogView struct {
	widget.BaseWidget
	// unexported
	mu       sync.Mutex
	rich     *widget.RichText
	scroll   fyne.CanvasObject
	maxLines int
	lines    int
}

// CreateRenderer implements fyne.Widget.
func (l *LogView) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(l.scroll) }

// NewLogView keeps at most maxLines lines.
func NewLogView(maxLines int) *LogView { return newLogView(maxLines) }

// Append adds a line and scrolls to the bottom. Safe from any goroutine.
func (l *LogView) Append(level LogLevel, line string) { l.appendLine(level, line) }

// Clear empties the panel.
func (l *LogView) Clear() { l.clear() }

// ProgressPanel shows a phase label, a bar and "done / total".
type ProgressPanel struct {
	widget.BaseWidget
	// unexported
	phase   *widget.Label
	bar     *widget.ProgressBar
	inf     *widget.ProgressBarInfinite
	counts  *widget.Label
	content fyne.CanvasObject
}

// CreateRenderer implements fyne.Widget.
func (p *ProgressPanel) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(p.content)
}

func NewProgressPanel() *ProgressPanel               { return newProgressPanel() }
func (p *ProgressPanel) SetPhase(text string)        { p.setPhase(text) }
func (p *ProgressPanel) SetProgress(done, total int) { p.setProgress(done, total) }
func (p *ProgressPanel) SetIndeterminate(on bool)    { p.setIndeterminate(on) }
func (p *ProgressPanel) Reset()                      { p.reset() }

// WizardStep is one page of the first-run wizard.
type WizardStep struct {
	Title   string
	Content fyne.CanvasObject
	// Validate returns an error message to show when Next is pressed; nil allows advancing.
	Validate func() error
}

// ShowWizard opens a modal stepper (Back / Next / Finish, Cancel). onDone runs
// after the last step validates; onCancel when dismissed.
func ShowWizard(win fyne.Window, title string, steps []WizardStep, onDone func(), onCancel func()) {
	showWizard(win, title, steps, onDone, onCancel)
}

// NewUpdateBanner returns a dismissible bar "Version X is available" with
// Download (opens url via onOpen) and Skip buttons.
func NewUpdateBanner(version string, onOpen func(), onSkip func()) fyne.CanvasObject {
	return newUpdateBanner(version, onOpen, onSkip)
}

// NewPathRow builds a labelled entry with Browse and (optional) Open-folder
// buttons; onChanged fires debounced (300 ms) after typing and immediately
// after Browse. onBrowse performs the platform picker and returns the path.
func NewPathRow(label, placeholder string, onBrowse func() string, onOpen func(), onChanged func(string)) (fyne.CanvasObject, *PathRow) {
	return newPathRow(label, placeholder, onBrowse, onOpen, onChanged)
}

// PathRow exposes the entry of NewPathRow.
type PathRow struct {
	// SetText sets the value WITHOUT firing onChanged.
	SetText func(string)
	Text    func() string

	// unexported
	entry     *widget.Entry
	guard     bool
	timer     *time.Timer
	debounce  time.Duration
	onChanged func(string)
}
