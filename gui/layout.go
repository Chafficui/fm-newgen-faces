package gui

import (
	"image/color"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"fmnewgenfaces/gui/widgets"
	"fmnewgenfaces/internal/brand"
	"fmnewgenfaces/internal/core/ethnic"
	"fmnewgenfaces/internal/core/fmversion"
	"fmnewgenfaces/internal/core/pipeline"
	"fmnewgenfaces/internal/core/profile"
	"fmnewgenfaces/internal/i18n"
)

// contentMaxWidth caps the width of the main content column so lines of
// text and controls stay comfortably readable on a wide window; the column
// still shrinks below this on a narrower one.
const contentMaxWidth = 920

// logMinHeight is how tall the log panel is once the bottom accordion is
// expanded.
const logMinHeight = 220

// buildLayout constructs the whole window content: a one-row header, an
// update-banner slot, and a single scrollable column of three cards (face
// pack, newgen export, assign) followed by a collapsed log. It does not
// touch profiles or the filesystem; that happens in startup.go / profiles.go.
func (a *App) buildLayout() {
	header := a.buildHeader()
	a.bannerSlot = container.NewVBox()

	top := container.NewVBox(container.NewPadded(header), a.bannerSlot)
	a.win.SetContent(container.NewBorder(top, nil, nil, nil, a.buildContent()))
}

func (a *App) buildHeader() fyne.CanvasObject {
	appName := widget.NewLabelWithStyle(brand.AppName, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	a.profileSelect = widget.NewSelect(nil, func(name string) {
		if a.loading {
			return
		}
		a.onProfileSelected(name)
	})
	a.profileSelect.PlaceHolder = i18n.T("gui.header.no_profiles")

	a.profileMenuBtn = widget.NewButtonWithIcon("", theme.MoreVerticalIcon(), nil)
	a.profileMenuBtn.OnTapped = func() { a.showProfileMenu(a.profileMenuBtn) }

	left := container.NewHBox(appName, a.profileSelect, a.profileMenuBtn)

	helpBtn := widget.NewButtonWithIcon("", theme.HelpIcon(), a.showHelp)
	settingsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), nil)
	settingsBtn.OnTapped = func() { a.showSettingsMenu(settingsBtn) }

	right := container.NewHBox(helpBtn, settingsBtn)

	return container.NewBorder(nil, nil, left, right)
}

// showProfileMenu opens the "⋯" popup next to the profile selector: create,
// rename, delete, and re-running the first-run wizard.
func (a *App) showProfileMenu(rel fyne.CanvasObject) {
	items := []*fyne.MenuItem{
		fyne.NewMenuItem(i18n.T("gui.header.new_profile"), a.newProfile),
		fyne.NewMenuItem(i18n.T("gui.header.rename"), a.renameProfile),
		fyne.NewMenuItem(i18n.T("gui.header.delete"), a.deleteProfile),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem(i18n.T("gui.header.rerun_wizard"), a.showWizard),
	}
	menu := fyne.NewMenu("", items...)
	widget.ShowPopUpMenuAtRelativePosition(menu, a.win.Canvas(), fyne.NewPos(0, rel.Size().Height), rel)
}

func (a *App) showSettingsMenu(rel fyne.CanvasObject) {
	themeItems := []*fyne.MenuItem{
		a.themeMenuItem("system", i18n.T("gui.settings_menu.theme_system")),
		a.themeMenuItem("light", i18n.T("gui.settings_menu.theme_light")),
		a.themeMenuItem("dark", i18n.T("gui.settings_menu.theme_dark")),
	}
	themeMenu := fyne.NewMenuItem(i18n.T("gui.settings_menu.theme"), nil)
	themeMenu.ChildMenu = fyne.NewMenu("", themeItems...)

	var langItems []*fyne.MenuItem
	for _, lang := range i18n.Available() {
		lang := lang
		item := fyne.NewMenuItem(lang.Name, func() { a.setLanguage(lang.Code) })
		item.Checked = lang.Code == i18n.Current()
		langItems = append(langItems, item)
	}
	langMenu := fyne.NewMenuItem(i18n.T("gui.settings_menu.language"), nil)
	langMenu.ChildMenu = fyne.NewMenu("", langItems...)

	updateItem := fyne.NewMenuItem(i18n.T("gui.settings_menu.check_updates"), a.checkForUpdatesNow)
	reportBugItem := fyne.NewMenuItem(i18n.T("gui.settings_menu.report_bug"), a.showBugReport)

	menu := fyne.NewMenu("", themeMenu, langMenu, fyne.NewMenuItemSeparator(), updateItem, reportBugItem)
	widget.ShowPopUpMenuAtRelativePosition(menu, a.win.Canvas(), fyne.NewPos(0, rel.Size().Height), rel)
}

func (a *App) themeMenuItem(setting, label string) *fyne.MenuItem {
	item := fyne.NewMenuItem(label, func() { a.setTheme(setting) })
	item.Checked = a.themeSetting == setting || (a.themeSetting == "" && setting == "system")
	return item
}

func (a *App) setTheme(setting string) {
	a.themeSetting = setting
	state := a.updateState(func(s *profile.State) { s.Theme = setting })
	a.fyneApp.Settings().SetTheme(newTheme(setting))
	if err := a.store.SaveState(state); err != nil {
		a.errorf("saving app state: %v", err)
	}
}

func (a *App) setLanguage(code string) {
	state := a.updateState(func(s *profile.State) { s.Language = code })
	if err := a.store.SaveState(state); err != nil {
		a.errorf("saving app state: %v", err)
	}
	dialog.ShowInformation(i18n.T("gui.settings_menu.language"), i18n.T("gui.settings_menu.language_restart"), a.win)
}

// buildContent assembles the three cards and the bottom log accordion into
// a single column, padded and capped to contentMaxWidth, inside a vertical
// scroll.
func (a *App) buildContent() fyne.CanvasObject {
	column := container.NewVBox(
		a.buildCard1(),
		a.buildCard2(),
		a.buildCard3(),
		a.buildLogAccordion(),
	)
	padded := container.NewPadded(column)
	centered := container.New(widgets.NewCenteredLayout(contentMaxWidth), padded)
	return container.NewVScroll(centered)
}

// --- Card 1: face pack ---

func (a *App) buildCard1() fyne.CanvasObject {
	var packRow *widgets.PathRow
	var packRowObj fyne.CanvasObject
	packRowObj, packRow = widgets.NewPathRow(
		i18n.T("gui.setup.pack_dir"), i18n.T("gui.setup.pack_dir_placeholder"),
		func() string { return pickFolder(packRow.Text()) },
		func() { a.openPackFolder() },
		func(v string) { a.onPackDirChanged(v) },
	)
	a.packRow = packRow
	a.packStatusBox = container.NewVBox()

	configRowObj, configRow := widgets.NewPathRow(
		i18n.T("gui.setup.config_file"), i18n.T("gui.setup.config_xml_placeholder"),
		func() string { return pickFile(i18n.T("gui.setup.config_heading"), "xml") },
		nil,
		func(v string) { a.onConfigChanged(v) },
	)
	a.configRow = configRow
	a.configStatusBox = container.NewVBox()

	var versionOptions []string
	for _, v := range fmversion.Known {
		versionOptions = append(versionOptions, v.Display())
	}
	a.versionSelect = widget.NewSelect(versionOptions, func(display string) {
		if a.loading {
			return
		}
		a.onVersionChanged(versionForDisplay(display))
	})
	a.versionStatusBox = container.NewVBox()
	a.installStatusBox = container.NewVBox()
	a.packTable = widgets.NewPackTable()

	sectionLabel := func(text string) fyne.CanvasObject {
		return widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}

	advancedContent := container.NewVBox(
		sectionLabel(i18n.T("gui.setup.fm_version")),
		a.versionSelect,
		a.versionStatusBox,
		widget.NewSeparator(),
		sectionLabel(i18n.T("gui.setup.config_heading")),
		configRowObj,
		a.configStatusBox,
		widget.NewSeparator(),
		sectionLabel(i18n.T("gui.checklist.install_title")),
		a.installStatusBox,
	)
	a.advancedDisclosure = a.newDisclosure(i18n.T("gui.card1.advanced"), advancedContent)

	body := container.NewVBox(packRowObj, a.packStatusBox, a.advancedDisclosure.root())
	return widget.NewCard(i18n.T("gui.card1.title"), i18n.T("gui.card1.subtitle"), body)
}

// --- Card 2: newgen export ---

func (a *App) buildCard2() fyne.CanvasObject {
	rtfRowObj, rtfRow := widgets.NewPathRow(
		i18n.T("gui.setup.rtf_path"), i18n.T("gui.setup.rtf_path_placeholder"),
		func() string { return pickFile(i18n.T("gui.setup.rtf_path"), "rtf") },
		func() { a.openRTFFolder() },
		func(v string) { a.onRTFChanged(v) },
	)
	a.rtfRow = rtfRow
	a.rtfStatusBox = container.NewVBox()

	a.rtfHintLabel = widget.NewLabel(i18n.T("gui.card2.hint"))
	a.rtfHintLabel.Wrapping = fyne.TextWrapWord
	a.rtfHintLabel.Importance = widget.LowImportance

	body := container.NewVBox(rtfRowObj, a.rtfStatusBox, a.rtfHintLabel)
	return widget.NewCard(i18n.T("gui.card2.title"), i18n.T("gui.card2.subtitle"), body)
}

// setRTFHint updates the muted hint line under the RTF row, appending the
// "watching folder" note while the fsnotify watcher is armed.
func (a *App) setRTFHint(watching bool) {
	if a.rtfHintLabel == nil {
		return
	}
	text := i18n.T("gui.card2.hint")
	if watching {
		text = text + " " + i18n.T("gui.card2.watching")
	}
	a.rtfHintLabel.SetText(text)
}

// --- Card 3: assign faces ---

func (a *App) buildCard3() fyne.CanvasObject {
	a.preserveCheck = widget.NewCheck(i18n.T("gui.settings.preserve"), func(bool) {
		if a.loading {
			return
		}
		a.settingsChanged()
	})
	// Checked means "avoid duplicates" (the safe option), the inverse of
	// Settings.AllowDuplicates; settingsChanged/applyProfile invert it back.
	a.allowDupCheck = widget.NewCheck(i18n.T("gui.card3.avoid_duplicates"), func(bool) {
		if a.loading {
			return
		}
		a.settingsChanged()
	})

	a.overrideEditor = widgets.NewOverrideEditor(widgets.OverrideEditorConfig{
		Get: func() map[string]string {
			if a.current == nil {
				return nil
			}
			return a.current.Settings.Overrides
		},
		Set: func(m map[string]string) {
			if a.current == nil {
				return
			}
			a.current.Settings.Overrides = m
			a.settingsChanged()
		},
		KnownCodes:  knownEthnicCodes(),
		DefaultFor:  func(code string) string { e, _ := ethnic.DefaultFor(code); return string(e) },
		EthnicNames: ethnic.Names(),
		Import: func(text string) error {
			overrides, err := pipeline.ParseOverridesTOML(text)
			if err != nil {
				return err
			}
			if a.current != nil {
				a.current.Settings.Overrides = overrides
				a.settingsChanged()
				a.overrideEditor.Refresh()
			}
			return nil
		},
		Export: func() string {
			if a.current == nil {
				return ""
			}
			return pipeline.OverridesTOML(a.current.Settings.Overrides)
		},
		Window: a.win,
	})

	a.overridesBtn = widget.NewButton(i18n.T("gui.card3.overrides"), func() { a.showOverridesDialog() })

	a.reviewTable = widgets.NewReviewTable(widgets.ReviewConfig{
		ImagePath: a.reviewImagePath,
		Reroll:    a.reviewReroll,
		Window:    a.win,
	})

	checksRow := container.NewBorder(nil, nil, nil, a.overridesBtn,
		container.NewHBox(a.preserveCheck, a.allowDupCheck))

	a.primaryBtn = widget.NewButtonWithIcon(i18n.T("gui.card3.primary"), theme.MediaPlayIcon(), func() { a.startRun(false) })
	a.primaryBtn.Importance = widget.HighImportance
	a.undoBtn = widget.NewButtonWithIcon(i18n.T("gui.run.undo"), theme.ContentUndoIcon(), a.restoreLatestBackup)
	a.undoBtn.Disable()

	buttonRow := container.NewHBox(a.primaryBtn, a.undoBtn)

	a.primaryReasonLabel = widget.NewLabel("")
	a.primaryReasonLabel.Wrapping = fyne.TextWrapWord
	a.primaryReasonLabel.Importance = widget.WarningImportance
	a.primaryReasonLabel.Hide()

	a.progress = widgets.NewProgressPanel()
	a.progress.Hide()

	a.resultLabel = widget.NewLabel("")
	a.resultLabel.Wrapping = fyne.TextWrapWord
	a.resultReviewBtn = widget.NewButtonWithIcon(i18n.T("gui.card3.review"), theme.VisibilityIcon(), func() { a.openReviewDialog() })
	a.resultOpenFolderBtn = widget.NewButtonWithIcon(i18n.T("gui.run.open_folder"), theme.FolderOpenIcon(), a.openPackFolder)
	a.resultStrip = container.NewVBox(
		a.resultLabel,
		container.NewHBox(a.resultReviewBtn, a.resultOpenFolderBtn),
	)
	a.resultStrip.Hide()

	body := container.NewVBox(checksRow, buttonRow, a.primaryReasonLabel, a.progress, a.resultStrip)
	return widget.NewCard(i18n.T("gui.card3.title"), i18n.T("gui.card3.subtitle"), body)
}

// showOverridesDialog opens the nation->ethnic override editor in a dialog;
// it lives in Card 3 permanently but is only shown on demand.
func (a *App) showOverridesDialog() {
	a.overrideEditor.Refresh()
	d := dialog.NewCustom(i18n.T("gui.card3.overrides_title"), i18n.T("common.close"), a.overrideEditor, a.win)
	d.Resize(fyne.NewSize(480, 420))
	d.Show()
}

// --- bottom: log accordion ---

func (a *App) buildLogAccordion() fyne.CanvasObject {
	logView := widgets.NewLogView(500)
	a.setLogView(logView)

	// Stacking the log view over an invisible rectangle with a fixed
	// minimum size reserves logMinHeight for it once expanded, without
	// forcing the log view's own (small) intrinsic min size on it.
	minHeight := canvas.NewRectangle(color.Transparent)
	minHeight.SetMinSize(fyne.NewSize(0, logMinHeight))
	wrapped := container.NewStack(minHeight, logView)

	a.logDisclosure = a.newDisclosure(i18n.T("gui.log.title"), wrapped)

	if a.logTee != nil {
		a.logTee.onError = func() {
			doUI(func() { a.logDisclosure.setOpen(a, true) })
		}
	}

	return a.logDisclosure.root()
}

func versionForDisplay(display string) string {
	for _, v := range fmversion.Known {
		if v.Display() == display {
			return v.Year
		}
	}
	return ""
}

func displayForVersion(year string) string {
	if v, ok := fmversion.Lookup(year); ok {
		return v.Display()
	}
	return ""
}

// openPackFolder opens the current pack directory in the OS file manager.
func (a *App) openPackFolder() {
	if a.current == nil || a.current.Settings.PackDir == "" {
		return
	}
	if err := openFolder(a.current.Settings.PackDir); err != nil {
		a.errorf("open pack folder: %v", err)
		dialog.ShowError(err, a.win)
	}
}

func (a *App) openRTFFolder() {
	if a.current == nil || a.current.Settings.RTFPath == "" {
		return
	}
	if err := openFolder(filepath.Dir(a.current.Settings.RTFPath)); err != nil {
		a.errorf("open RTF folder: %v", err)
		dialog.ShowError(err, a.win)
	}
}

func (a *App) showBanner(obj fyne.CanvasObject) {
	a.bannerSlot.Objects = []fyne.CanvasObject{obj}
	a.bannerSlot.Refresh()
	a.refreshPage()
}

func (a *App) hideBanner() {
	a.bannerSlot.Objects = nil
	a.bannerSlot.Refresh()
	a.refreshPage()
}

// knownEthnicCodes returns every nation code the default table knows, for
// the override editor's autocomplete.
func knownEthnicCodes() []string {
	r, _ := ethnic.NewResolver(nil)
	return r.Codes()
}
