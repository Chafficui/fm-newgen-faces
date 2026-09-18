package gui

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"fmnewgenfaces/gui/widgets"
	"fmnewgenfaces/internal/core/ethnic"
	"fmnewgenfaces/internal/core/fmversion"
	"fmnewgenfaces/internal/core/pipeline"
	"fmnewgenfaces/internal/core/profile"
	"fmnewgenfaces/internal/i18n"
)

// buildLayout constructs the whole window content. It does not touch
// profiles or the filesystem; that happens in startup.go / profiles.go.
func (a *App) buildLayout() {
	header := a.buildHeader()
	a.bannerSlot = container.NewVBox()

	a.tabs = container.NewAppTabs(
		container.NewTabItemWithIcon(i18n.T("gui.tabs.setup"), theme.ListIcon(), a.buildSetupTab()),
		container.NewTabItemWithIcon(i18n.T("gui.tabs.settings"), theme.SettingsIcon(), a.buildSettingsTab()),
		container.NewTabItemWithIcon(i18n.T("gui.tabs.run"), theme.MediaPlayIcon(), a.buildRunTab()),
		container.NewTabItemWithIcon(i18n.T("gui.tabs.review"), theme.VisibilityIcon(), a.buildReviewTab()),
	)

	top := container.NewVBox(header, a.bannerSlot)
	a.win.SetContent(container.NewBorder(top, nil, nil, nil, a.tabs))
}

func (a *App) buildHeader() fyne.CanvasObject {
	a.profileSelect = widget.NewSelect(nil, func(name string) {
		if a.loading {
			return
		}
		a.onProfileSelected(name)
	})
	a.profileSelect.PlaceHolder = i18n.T("gui.header.no_profiles")

	a.newProfileBtn = widget.NewButtonWithIcon(i18n.T("gui.header.new_profile"), theme.ContentAddIcon(), a.newProfile)
	a.renameProfileBtn = widget.NewButtonWithIcon(i18n.T("gui.header.rename"), theme.DocumentCreateIcon(), a.renameProfile)
	a.deleteProfileBtn = widget.NewButtonWithIcon(i18n.T("gui.header.delete"), theme.DeleteIcon(), a.deleteProfile)

	left := container.NewHBox(a.profileSelect, a.newProfileBtn, a.renameProfileBtn, a.deleteProfileBtn)

	helpBtn := widget.NewButtonWithIcon(i18n.T("gui.header.help"), theme.HelpIcon(), a.showHelp)
	bugBtn := widget.NewButtonWithIcon(i18n.T("gui.header.report_bug"), theme.MailComposeIcon(), a.showBugReport)
	settingsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), nil)
	settingsBtn.OnTapped = func() { a.showSettingsMenu(settingsBtn) }

	right := container.NewHBox(helpBtn, bugBtn, settingsBtn)

	return container.NewBorder(nil, nil, left, right)
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

	menu := fyne.NewMenu("", themeMenu, langMenu, fyne.NewMenuItemSeparator(), updateItem)
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

func (a *App) buildSetupTab() fyne.CanvasObject {
	a.checklist = widgets.NewChecklist()

	var packRow *widgets.PathRow
	var packRowObj fyne.CanvasObject
	packRowObj, packRow = widgets.NewPathRow(
		i18n.T("gui.setup.pack_dir"), i18n.T("gui.setup.pack_dir_placeholder"),
		func() string { return pickFolder(packRow.Text()) },
		func() { a.openPackFolder() },
		func(v string) { a.onPackDirChanged(v) },
	)
	a.packRow = packRow

	configRowObj, configRow := widgets.NewPathRow(
		i18n.T("gui.setup.config_xml"), i18n.T("gui.setup.config_xml_placeholder"),
		func() string { return pickFile(i18n.T("gui.setup.config_xml"), "xml") },
		nil,
		func(v string) { a.onConfigChanged(v) },
	)
	a.configRow = configRow

	rtfRowObj, rtfRow := widgets.NewPathRow(
		i18n.T("gui.setup.rtf_path"), i18n.T("gui.setup.rtf_path_placeholder"),
		func() string { return pickFile(i18n.T("gui.setup.rtf_path"), "rtf") },
		func() { a.openRTFFolder() },
		func(v string) { a.onRTFChanged(v) },
	)
	a.rtfRow = rtfRow

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

	a.packTable = widgets.NewPackTable()

	top := container.NewVBox(
		a.checklist,
		widget.NewSeparator(),
		packRowObj,
		configRowObj,
		rtfRowObj,
		container.NewBorder(nil, nil, widget.NewLabel(i18n.T("gui.setup.fm_version")), nil, a.versionSelect),
		widget.NewSeparator(),
	)

	// The compact checklist, the three path rows and the version selector
	// fit above the pack table at the default window height; the table is
	// the Border's center and scrolls within whatever height remains.
	return container.NewBorder(top, nil, nil, nil, a.packTable)
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

func (a *App) buildSettingsTab() fyne.CanvasObject {
	a.preserveCheck = widget.NewCheck(i18n.T("gui.settings.preserve"), func(bool) {
		if a.loading {
			return
		}
		a.settingsChanged()
	})
	preserveHelp := widget.NewLabel(i18n.T("gui.settings.preserve_help"))
	preserveHelp.Wrapping = fyne.TextWrapWord

	a.allowDupCheck = widget.NewCheck(i18n.T("gui.settings.allow_duplicates"), func(bool) {
		if a.loading {
			return
		}
		a.settingsChanged()
	})
	dupHelp := widget.NewLabel(i18n.T("gui.settings.allow_duplicates_help"))
	dupHelp.Wrapping = fyne.TextWrapWord

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

	top := container.NewVBox(
		a.preserveCheck, preserveHelp,
		widget.NewSeparator(),
		a.allowDupCheck, dupHelp,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(i18n.T("gui.settings.overrides_header"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)
	return container.NewBorder(top, nil, nil, nil, a.overrideEditor)
}

func (a *App) buildRunTab() fyne.CanvasObject {
	a.previewBtn = widget.NewButtonWithIcon(i18n.T("gui.run.preview"), theme.VisibilityIcon(), func() { a.startRun(true) })
	a.assignBtn = widget.NewButtonWithIcon(i18n.T("gui.run.assign"), theme.MediaPlayIcon(), func() { a.startRun(false) })
	a.assignBtn.Importance = widget.HighImportance
	a.undoBtn = widget.NewButtonWithIcon(i18n.T("gui.run.undo"), theme.ContentUndoIcon(), a.restoreLatestBackup)
	a.openFolderBtn = widget.NewButtonWithIcon(i18n.T("gui.run.open_folder"), theme.FolderOpenIcon(), a.openPackFolder)

	buttons := container.NewHBox(a.previewBtn, a.assignBtn, a.undoBtn, a.openFolderBtn)

	a.progress = widgets.NewProgressPanel()
	logView := widgets.NewLogView(500)
	a.setLogView(logView)

	top := container.NewVBox(buttons, a.progress, widget.NewSeparator())
	return container.NewBorder(top, nil, nil, nil, logView)
}

func (a *App) buildReviewTab() fyne.CanvasObject {
	a.reviewTable = widgets.NewReviewTable(widgets.ReviewConfig{
		ImagePath: a.reviewImagePath,
		Reroll:    a.reviewReroll,
		Window:    a.win,
	})

	loadBtn := widget.NewButton(i18n.T("gui.review.load_current"), a.loadCurrentMappings)

	return container.NewBorder(loadBtn, nil, nil, nil, a.reviewTable)
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
}

func (a *App) hideBanner() {
	a.bannerSlot.Objects = nil
	a.bannerSlot.Refresh()
}

// knownEthnicCodes returns every nation code the default table knows, for
// the override editor's autocomplete.
func knownEthnicCodes() []string {
	r, _ := ethnic.NewResolver(nil)
	return r.Codes()
}
