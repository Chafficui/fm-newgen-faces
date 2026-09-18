package gui

import (
	"fyne.io/fyne/v2/dialog"

	"fmnewgenfaces/internal/core/fminstall"
	"fmnewgenfaces/internal/core/profile"
	"fmnewgenfaces/internal/i18n"
)

// refreshProfileList re-reads the store and repopulates the header dropdown,
// keeping the current profile selected.
func (a *App) refreshProfileList() {
	a.profiles = a.store.List()
	names := make([]string, len(a.profiles))
	for i, p := range a.profiles {
		names[i] = p.Name
	}
	a.profileSelect.SetOptions(names)
	if a.current != nil {
		a.loading = true
		a.profileSelect.SetSelected(a.current.Name)
		a.loading = false
	}
}

func (a *App) onProfileSelected(name string) {
	for _, p := range a.profiles {
		if p.Name == name {
			a.selectProfile(p.Slug)
			return
		}
	}
}

// selectProfile flushes the outgoing profile's pending autosave, loads slug,
// applies it to the UI and re-evaluates the checklist.
func (a *App) selectProfile(slug string) {
	p, ok := a.store.Get(slug)
	if !ok {
		return
	}
	if a.current != nil && a.current.Slug != p.Slug {
		a.autosaver.Flush()
	}
	a.applyProfile(p)

	a.updateState(func(s *profile.State) { s.LastProfile = p.Slug })
	a.rearmWatcher()
	a.evaluate()
}

// applyProfile fills every UI field from p WITHOUT firing change handlers.
func (a *App) applyProfile(p *profile.Profile) {
	a.loading = true
	defer func() { a.loading = false }()

	a.current = p
	a.setInputs(nil)

	if a.profileSelect != nil {
		a.profileSelect.SetSelected(p.Name)
	}

	a.packRow.SetText(p.Settings.PackDir)
	a.configRow.SetText(p.Settings.ConfigXML)
	a.rtfRow.SetText(p.Settings.RTFPath)

	disp := displayForVersion(p.Settings.FMVersion)
	a.versionSelect.SetSelected(disp)

	a.preserveCheck.SetChecked(p.Settings.Preserve)
	a.allowDupCheck.SetChecked(p.Settings.AllowDuplicates)

	if a.overrideEditor != nil {
		a.overrideEditor.Refresh()
	}
	if a.reviewTable != nil {
		a.reviewTable.SetAssignments(nil)
	}
}

// settingsChanged writes every UI field back into a.current.Settings,
// schedules an autosave and re-evaluates the checklist.
func (a *App) settingsChanged() {
	if a.loading || a.current == nil {
		return
	}
	a.current.Settings.PackDir = a.packRow.Text()
	a.current.Settings.ConfigXML = a.configRow.Text()
	a.current.Settings.RTFPath = a.rtfRow.Text()
	if a.versionSelect.Selected != "" {
		if year := versionForDisplay(a.versionSelect.Selected); year != "" {
			a.current.Settings.FMVersion = year
		}
	}
	a.current.Settings.Preserve = a.preserveCheck.Checked
	a.current.Settings.AllowDuplicates = a.allowDupCheck.Checked

	a.autosaver.Trigger(a.current)
	a.evaluate()
}

func (a *App) onPackDirChanged(v string) {
	a.settingsChanged()
	a.rearmWatcher()
	a.maybeOfferProfileSwitch(v)
}

func (a *App) onConfigChanged(string) {
	a.settingsChanged()
}

func (a *App) onRTFChanged(string) {
	a.settingsChanged()
	a.rearmWatcher()
}

func (a *App) onVersionChanged(string) {
	a.settingsChanged()
}

// maybeOfferProfileSwitch offers to switch when the newly entered pack path
// belongs to a different FM install that already has its own profile.
func (a *App) maybeOfferProfileSwitch(packDir string) {
	if packDir == "" || a.current == nil {
		return
	}
	inst, ok := fminstall.FromPath(packDir)
	if !ok {
		return
	}
	other, ok := a.store.FindByGamePath(inst.BasePath)
	if !ok || other.Slug == a.current.Slug {
		return
	}
	otherSlug := other.Slug
	otherPath := packDir
	dialog.ShowConfirm(
		i18n.T("gui.profile.switch_title"),
		i18n.T("gui.profile.switch_body", other.Name),
		func(yes bool) {
			if !yes {
				return
			}
			target, ok := a.store.Get(otherSlug)
			if !ok {
				return
			}
			target.Settings.PackDir = otherPath
			if err := a.store.Save(target); err != nil {
				a.errorf("saving profile %q: %v", target.Name, err)
			}
			a.selectProfile(otherSlug)
		},
		a.win,
	)
}

func (a *App) newProfile() {
	d := dialog.NewEntryDialog(i18n.T("gui.profile.new_title"), i18n.T("gui.profile.new_prompt"), func(name string) {
		if name == "" {
			return
		}
		p, err := profile.Create(a.store, name, "", nil)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		a.refreshProfileList()
		a.selectProfile(p.Slug)
	}, a.win)
	d.Show()
}

func (a *App) renameProfile() {
	if a.current == nil {
		return
	}
	slug := a.current.Slug
	d := dialog.NewEntryDialog(i18n.T("gui.profile.rename_title"), i18n.T("gui.profile.rename_prompt"), func(name string) {
		if name == "" {
			return
		}
		if err := a.store.Rename(slug, name); err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		if p, ok := a.store.Get(slug); ok {
			a.current = p
		}
		a.refreshProfileList()
	}, a.win)
	d.SetText(a.current.Name)
	d.Show()
}

func (a *App) deleteProfile() {
	if a.current == nil {
		return
	}
	if len(a.profiles) <= 1 {
		dialog.ShowInformation(i18n.T("gui.profile.delete_title"), i18n.T("gui.profile.delete_last"), a.win)
		return
	}
	target := a.current
	dialog.ShowConfirm(i18n.T("gui.profile.delete_title"), i18n.T("gui.profile.delete_confirm", target.Name), func(yes bool) {
		if !yes {
			return
		}
		if err := a.store.Delete(target.Slug); err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		remaining := a.store.List()
		a.current = nil
		a.refreshProfileList()
		if len(remaining) > 0 {
			a.selectProfile(remaining[0].Slug)
		}
	}, a.win)
}
