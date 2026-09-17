package gui

import (
	"errors"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"fmnewgenfaces/gui/widgets"
	"fmnewgenfaces/internal/core/assign"
	"fmnewgenfaces/internal/core/ethnic"
	"fmnewgenfaces/internal/core/fmconfig"
	"fmnewgenfaces/internal/core/pipeline"
	"fmnewgenfaces/internal/core/rtf"
	"fmnewgenfaces/internal/i18n"
)

func (a *App) backupBaseDir() string {
	return filepath.Join(a.store.Dir(), "backups")
}

// startRun is shared by the Preview and Assign buttons/shortcuts. dryRun
// true means "Preview": the resulting dialog has no functional confirm
// action. Assign always shows the same preview first; only its confirm
// button actually writes anything.
func (a *App) startRun(dryRun bool) {
	if a.current == nil {
		dialog.ShowInformation(i18n.T("gui.run.no_profile_title"), i18n.T("gui.run.no_profile_body"), a.win)
		return
	}

	a.runMu.Lock()
	if a.running {
		a.runMu.Unlock()
		return
	}
	a.running = true
	a.runMu.Unlock()

	a.setRunButtonsEnabled(false)
	a.progress.Reset()
	a.progress.SetIndeterminate(true)

	settings := cloneSettings(a.current.Settings)

	go func() {
		in, err := pipeline.Load(settings)
		if err != nil {
			doUI(func() {
				a.errorf("load failed: %v", err)
				dialog.ShowError(err, a.win)
				a.finishRun()
			})
			return
		}
		for _, w := range in.Warnings {
			a.warnf("%s", w)
		}

		doUI(func() {
			a.inputs = in
			if len(in.RTF.UnmappedPlayers) > 0 {
				widgets.ShowUnmappedResolver(a.win, in.RTF.Unmapped, ethnic.Names(),
					func(sel map[string]string) {
						a.applyUnmappedOverrides(sel)
						a.continueRun(dryRun)
					},
					func() { a.continueRun(dryRun) },
				)
				return
			}
			a.continueRun(dryRun)
		})
	}()
}

// applyUnmappedOverrides saves the user's nation->ethnic choices onto the
// current profile and re-resolves the already-parsed RTF result in place.
func (a *App) applyUnmappedOverrides(sel map[string]string) {
	if len(sel) == 0 || a.current == nil || a.inputs == nil {
		return
	}
	if a.current.Settings.Overrides == nil {
		a.current.Settings.Overrides = map[string]string{}
	}
	for k, v := range sel {
		a.current.Settings.Overrides[strings.ToUpper(k)] = v
	}
	if err := a.store.Save(a.current); err != nil {
		a.errorf("saving overrides: %v", err)
	}

	resolver, rErr := ethnic.NewResolver(a.current.Settings.Overrides)
	if rErr != nil {
		a.warnf("override error: %v", rErr)
	}
	a.inputs.Resolver = resolver
	a.inputs.RTF.Resolve(resolver)
	if a.overrideEditor != nil {
		a.overrideEditor.Refresh()
	}
}

// continueRun builds the plan and shows the preview dialog. The Load phase
// is over, so the run buttons are re-enabled here; the preview/summary
// dialogs are modal and block further interaction until dismissed.
func (a *App) continueRun(dryRun bool) {
	a.finishRun()
	if a.inputs == nil {
		return
	}
	plan := pipeline.Plan(a.inputs)

	if dryRun {
		widgets.ShowPreview(a.win, plan, i18n.T("common.close"), nil)
		return
	}
	widgets.ShowPreview(a.win, plan, i18n.T("gui.run.assign"), func() {
		a.executeRun(plan)
	})
}

// executeRun applies plan and saves config.xml (with backup).
func (a *App) executeRun(plan *assign.Plan) {
	a.runMu.Lock()
	if a.running {
		a.runMu.Unlock()
		return
	}
	a.running = true
	a.runMu.Unlock()

	a.setRunButtonsEnabled(false)
	a.progress.Reset()

	in := a.inputs
	backupBase := a.backupBaseDir()

	go func() {
		rr, err := pipeline.Run(in, plan, backupBase, func(done, total int) {
			a.progress.SetProgress(done, total)
		})
		doUI(func() {
			defer a.finishRun()
			if err != nil {
				a.errorf("assign failed: %v", err)
				dialog.ShowError(err, a.win)
				return
			}
			for _, s := range rr.Result.Skipped {
				a.warnf("skipped %s (%s): %s", s.Player.Name, s.Player.ID, s.Reason)
			}
			a.logf("assigned %d, preserved %d, skipped %d, unmapped %d", len(rr.Result.Assigned), rr.Result.Preserved, len(rr.Result.Skipped), rr.Result.Unmapped)

			widgets.ShowSummary(a.win, plan, rr.Result, rr.BackupPath, widgets.SummaryActions{
				OpenFolder: a.openPackFolder,
				ShowLog:    func() { a.tabs.SelectIndex(2) },
				Undo:       a.restoreLatestBackup,
				Review: func() {
					a.reviewTable.SetAssignments(pipeline.Assignments(in))
					a.tabs.SelectIndex(3)
				},
			})
			a.evaluate()
		})
	}()
}

func (a *App) finishRun() {
	a.runMu.Lock()
	a.running = false
	a.runMu.Unlock()
	a.setRunButtonsEnabled(true)
	a.progress.SetIndeterminate(false)
}

func (a *App) setRunButtonsEnabled(enabled bool) {
	for _, b := range []*widget.Button{a.previewBtn, a.assignBtn, a.undoBtn, a.openFolderBtn} {
		if b == nil {
			continue
		}
		if enabled {
			b.Enable()
		} else {
			b.Disable()
		}
	}
}

// restoreLatestBackup lists the current config.xml's backups and, after
// confirmation, restores the newest one.
func (a *App) restoreLatestBackup() {
	if a.current == nil || a.current.Settings.ConfigXML == "" {
		return
	}
	cfgPath := a.current.Settings.ConfigXML
	backupDir := fmconfig.BackupDirFor(a.backupBaseDir(), cfgPath)

	go func() {
		backups, err := fmconfig.ListBackups(backupDir)
		doUI(func() {
			if err != nil {
				a.errorf("listing backups: %v", err)
				dialog.ShowError(err, a.win)
				return
			}
			if len(backups) == 0 {
				dialog.ShowInformation(i18n.T("gui.run.undo_title"), i18n.T("gui.run.undo_none"), a.win)
				return
			}
			latest := backups[0]
			dialog.ShowConfirm(i18n.T("gui.run.undo_title"), i18n.T("gui.run.undo_confirm", latest.Time.Format("2006-01-02 15:04:05")), func(yes bool) {
				if !yes {
					return
				}
				go func() {
					restoreErr := fmconfig.Restore(latest.Path, cfgPath)
					doUI(func() {
						if restoreErr != nil {
							a.errorf("restore backup: %v", restoreErr)
							dialog.ShowError(restoreErr, a.win)
							return
						}
						a.logf("restored backup %s", latest.Path)
						a.evaluate()
					})
				}()
			}, a.win)
		})
	}()
}

// loadCurrentMappings loads the profile's inputs (if needed) and fills the
// review table from the config.xml currently on disk.
func (a *App) loadCurrentMappings() {
	if a.current == nil {
		return
	}
	settings := cloneSettings(a.current.Settings)
	go func() {
		in, err := pipeline.Load(settings)
		doUI(func() {
			if err != nil {
				a.errorf("load failed: %v", err)
				dialog.ShowError(err, a.win)
				return
			}
			a.inputs = in
			a.reviewTable.SetAssignments(pipeline.Assignments(in))
		})
	}()
}

func (a *App) reviewImagePath(asn assign.Assignment) string {
	if a.inputs == nil {
		return ""
	}
	return pipeline.ImageFile(a.inputs, asn)
}

func (a *App) reviewReroll(p rtf.Player) (assign.Assignment, error) {
	if a.inputs == nil {
		return assign.Assignment{}, errors.New("no data loaded: use \"Load current mappings\" or run a preview/assign first")
	}
	asn, err := pipeline.Reroll(a.inputs, p, a.backupBaseDir())
	if err == nil {
		a.logf("rerolled %s (%s) -> %s", p.Name, p.ID, asn.Image)
	}
	return asn, err
}
