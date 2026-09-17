package gui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"fmnewgenfaces/gui/widgets"
	"fmnewgenfaces/internal/brand"
	"fmnewgenfaces/internal/bugreport"
	"fmnewgenfaces/internal/core/fminstall"
	"fmnewgenfaces/internal/i18n"
)

// exportInstructionsMarkdown renders the step-by-step "export the newgen
// list" instructions with the exact view/filter names, shared by the wizard
// and the help dialog.
func exportInstructionsMarkdown() string {
	return i18n.T("gui.help.export_markdown", fminstall.ViewName, fminstall.FilterName)
}

func (a *App) showHelp() {
	body := widget.NewRichTextFromMarkdown(exportInstructionsMarkdown() + "\n\n" + i18n.T("gui.help.next_steps_markdown"))
	body.Wrapping = fyne.TextWrapWord

	scroll := container.NewVScroll(body)
	scroll.SetMinSize(fyne.NewSize(480, 360))

	tutorialBtn := widget.NewButton(i18n.T("gui.wizard.watch_tutorial"), func() {
		if err := a.openURL(brand.TutorialURL); err != nil {
			a.errorf("opening tutorial: %v", err)
			dialog.ShowError(err, a.win)
		}
	})

	content := container.NewBorder(nil, tutorialBtn, nil, nil, scroll)
	d := dialog.NewCustom(i18n.T("gui.help.title"), i18n.T("common.close"), content, a.win)
	d.Resize(fyne.NewSize(520, 460))
	d.Show()
}

func renderChecklistText(rows []widgets.ChecklistRow) string {
	var b strings.Builder
	for _, r := range rows {
		b.WriteString("- ")
		b.WriteString(r.Title)
		b.WriteString(": ")
		b.WriteString(r.Detail)
		b.WriteString("\n")
	}
	return b.String()
}

// showBugReport collects the redacted report in a goroutine (so it can be
// cancelled by closing the dialog) and shows it once ready.
func (a *App) showBugReport() {
	cur := a.current

	entry := widget.NewMultiLineEntry()
	entry.Wrapping = fyne.TextWrapWord
	entry.SetText(i18n.T("gui.bugreport.collecting"))
	entry.Disable()

	scroll := container.NewVScroll(entry)
	scroll.SetMinSize(fyne.NewSize(560, 420))

	copyBtn := widget.NewButton(i18n.T("gui.bugreport.copy"), func() {
		a.fyneApp.Clipboard().SetContent(entry.Text)
	})
	copyBtn.Disable()

	issueBtn := widget.NewButton(i18n.T("gui.bugreport.open_issue"), func() {
		url := bugreport.IssueURL(i18n.T("gui.bugreport.issue_title"), entry.Text)
		if err := a.openURL(url); err != nil {
			a.errorf("opening issue page: %v", err)
			dialog.ShowError(err, a.win)
		}
	})
	issueBtn.Disable()

	var cancelled bool
	content := container.NewBorder(nil, container.NewHBox(copyBtn, issueBtn), nil, nil, scroll)
	d := dialog.NewCustom(i18n.T("gui.bugreport.title"), i18n.T("common.close"), content, a.win)
	d.SetOnClosed(func() { cancelled = true })
	d.Resize(fyne.NewSize(640, 560))
	d.Show()

	go func() {
		var checklistText string
		if cur != nil {
			rows, _ := a.computeChecklist(cloneSettings(cur.Settings))
			checklistText = renderChecklistText(rows)
		}
		text := bugreport.Build(bugreport.Info{
			Version:   brand.Version,
			Profiles:  a.store.List(),
			Current:   cur,
			LogTail:   a.logTail(100),
			Checklist: checklistText,
		})
		doUI(func() {
			if cancelled {
				return
			}
			entry.Enable()
			entry.SetText(text)
			copyBtn.Enable()
			issueBtn.Enable()
		})
	}()
}
