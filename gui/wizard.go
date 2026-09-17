package gui

import (
	"errors"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"fmnewgenfaces/gui/widgets"
	"fmnewgenfaces/internal/brand"
	"fmnewgenfaces/internal/core/facepack"
	"fmnewgenfaces/internal/core/fminstall"
	"fmnewgenfaces/internal/core/profile"
	"fmnewgenfaces/internal/i18n"
)

// showWizard runs the first-run setup wizard.
func (a *App) showWizard() {
	installs := fminstall.Detect()
	instByName := make(map[string]fminstall.Install, len(installs))

	var options []string
	for _, inst := range installs {
		instByName[inst.Name] = inst
		options = append(options, inst.Name)
	}
	customLabel := i18n.T("gui.wizard.custom_location")
	options = append(options, customLabel)

	radio := widget.NewRadioGroup(options, nil)
	radio.SetSelected(options[0])

	welcome := widget.NewLabel(i18n.T("gui.wizard.welcome_body"))
	welcome.Wrapping = fyne.TextWrapWord
	step1 := container.NewVBox(
		welcome,
		widget.NewSeparator(),
		widget.NewLabel(i18n.T("gui.wizard.pick_install")),
		radio,
	)

	var packRow *widgets.PathRow
	var packRowObj fyne.CanvasObject
	packRowObj, packRow = widgets.NewPathRow(
		i18n.T("gui.setup.pack_dir"), i18n.T("gui.setup.pack_dir_placeholder"),
		func() string { return pickFolder(packRow.Text()) }, nil, nil,
	)
	packBody := widget.NewLabel(i18n.T("gui.wizard.pack_body"))
	packBody.Wrapping = fyne.TextWrapWord
	step2 := container.NewVBox(packBody, packRowObj)

	rtfRowObj, rtfRow := widgets.NewPathRow(
		i18n.T("gui.setup.rtf_path"), i18n.T("gui.setup.rtf_path_placeholder"),
		func() string { return pickFile(i18n.T("gui.setup.rtf_path"), "rtf") }, nil, nil,
	)
	tutorialBtn := widget.NewButton(i18n.T("gui.wizard.watch_tutorial"), func() {
		if err := a.openURL(brand.TutorialURL); err != nil {
			dialog.ShowError(err, a.win)
		}
	})
	exportBody := widget.NewRichTextFromMarkdown(exportInstructionsMarkdown())
	exportBody.Wrapping = fyne.TextWrapWord
	step3 := container.NewVBox(exportBody, tutorialBtn, widget.NewSeparator(), rtfRowObj)

	doneLabel := widget.NewLabel("")
	doneLabel.Wrapping = fyne.TextWrapWord
	step4 := container.NewVBox(doneLabel)

	// lastPackCheck/lastPackOK cache the previous facepack.LooksLikePack
	// result for step 2's Validate, keyed by path, so repeated Next clicks
	// on an unchanged path don't rescan the directory.
	var lastPackCheck string
	var lastPackOK bool

	steps := []widgets.WizardStep{
		{Title: i18n.T("gui.wizard.step1_title"), Content: step1},
		{
			Title:   i18n.T("gui.wizard.step2_title"),
			Content: step2,
			// widgets.WizardStep.Validate is synchronous by contract (the
			// stepper calls it inline from Next and expects an immediate
			// result), so this stays on the UI thread. facepack.LooksLikePack
			// only does a single os.ReadDir, which is cheap enough for that;
			// the cache below just avoids repeating it when the user mashes
			// Next without having changed the path.
			Validate: func() error {
				v := packRow.Text()
				if v == "" {
					lastPackCheck, lastPackOK = "", false
					return errors.New(i18n.T("gui.wizard.pack_required"))
				}
				if v != lastPackCheck {
					lastPackCheck = v
					lastPackOK = facepack.LooksLikePack(v, 10)
				}
				if !lastPackOK {
					return errors.New(i18n.T("gui.wizard.pack_invalid"))
				}
				return nil
			},
		},
		{
			Title:   i18n.T("gui.wizard.step3_title"),
			Content: step3,
			Validate: func() error {
				v := rtfRow.Text()
				if v != "" {
					if st, err := os.Stat(v); err != nil || st.IsDir() {
						return errors.New(i18n.T("gui.wizard.rtf_invalid"))
					}
				}
				summaryRTF := i18n.T("gui.wizard.done_none")
				if v != "" {
					summaryRTF = v
				}
				doneLabel.SetText(i18n.T("gui.wizard.done_summary", packRow.Text(), summaryRTF))
				return nil
			},
		},
		{Title: i18n.T("gui.wizard.step4_title"), Content: step4},
	}

	onDone := func() {
		state := a.updateState(func(s *profile.State) { s.WizardDone = true })
		if err := a.store.SaveState(state); err != nil {
			a.errorf("saving app state: %v", err)
		}

		if a.current == nil {
			name := i18n.T("gui.wizard.custom_profile_name")
			gamePath := ""
			if inst, ok := instByName[radio.Selected]; ok {
				name = inst.Name
				gamePath = inst.BasePath
			}
			p, err := profile.Create(a.store, name, gamePath, nil)
			if err != nil {
				a.errorf("creating profile: %v", err)
				dialog.ShowError(err, a.win)
				return
			}
			a.current = p
		}

		a.current.Settings.PackDir = packRow.Text()
		if rtfRow.Text() != "" {
			a.current.Settings.RTFPath = rtfRow.Text()
		}
		if a.current.Settings.ConfigXML == "" && packRow.Text() != "" {
			a.current.Settings.ConfigXML = filepath.Join(packRow.Text(), "config.xml")
		}
		if inst, ok := instByName[radio.Selected]; ok {
			a.current.GamePath = inst.BasePath
			a.current.Settings.FMVersion = inst.Version.Year
		}
		if err := a.store.Save(a.current); err != nil {
			a.errorf("saving profile: %v", err)
		}

		a.refreshProfileList()
		a.applyProfile(a.current)
		a.rearmWatcher()
		a.evaluate()
	}

	onCancel := func() {
		state := a.updateState(func(s *profile.State) { s.WizardDone = true })
		if err := a.store.SaveState(state); err != nil {
			a.errorf("saving app state: %v", err)
		}
	}

	widgets.ShowWizard(a.win, i18n.T("gui.wizard.title"), steps, onDone, onCancel)
}
