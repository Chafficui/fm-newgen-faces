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
//
// a.running stays true for the whole flow this kicks off — Load, the
// unmapped-nation resolver (if shown), the preview dialog and, if
// confirmed, the run itself — and is only cleared by finishRun, which runs
// from whichever of those steps ends the flow without handing off to the
// next one (see continueRun and executeRun). This keeps a second startRun
// from being able to replace a.inputs (or start a second write) while a
// dialog for the current inputs/plan is still open.
func (a *App) startRun(dryRun bool) {
	if a.current == nil {
		dialog.ShowInformation(i18n.T("gui.run.no_profile_title"), i18n.T("gui.run.no_profile_body"), a.win)
		return
	}

	a.runMu.Lock()
	if a.running {
		a.runMu.Unlock()
		a.logf("a run is already in progress; ignoring")
		return
	}
	a.running = true
	a.runMu.Unlock()

	a.setBusy(true)
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
			a.setInputs(in)
			if len(in.RTF.UnmappedPlayers) > 0 {
				widgets.ShowUnmappedResolver(a.win, in.RTF.Unmapped, ethnic.Names(),
					func(sel map[string]string) {
						a.applyUnmappedOverrides(in, sel)
						a.continueRun(dryRun, in)
					},
					func() { a.continueRun(dryRun, in) },
				)
				return
			}
			a.continueRun(dryRun, in)
		})
	}()
}

// applyUnmappedOverrides saves the user's nation->ethnic choices onto the
// current profile and re-resolves the already-parsed RTF result (in, the
// same *pipeline.Inputs the caller is about to build a plan from) in place.
func (a *App) applyUnmappedOverrides(in *pipeline.Inputs, sel map[string]string) {
	if len(sel) == 0 || a.current == nil || in == nil {
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
	in.Resolver = resolver
	in.RTF.Resolve(resolver)
	if a.overrideEditor != nil {
		a.overrideEditor.Refresh()
	}
}

// continueRun builds the plan from in (the inputs startRun loaded) and
// shows the preview dialog. It does NOT call finishRun before showing the
// dialog: a.running stays true until the dialog is dismissed (onClose,
// below) or, once confirmed, until the run it starts completes.
func (a *App) continueRun(dryRun bool, in *pipeline.Inputs) {
	if in == nil {
		a.finishRun()
		return
	}
	plan := pipeline.Plan(in)

	// Load is done; stop the indeterminate spinner for the preview dialog
	// even though a.running (and the disabled buttons) stay in effect until
	// it is dismissed or a run it starts completes.
	a.progress.SetIndeterminate(false)

	if dryRun {
		// Read-only: whichever button closes it, there is nothing left to
		// wait on.
		widgets.ShowPreview(a.win, plan, i18n.T("common.close"), nil, a.finishRun)
		return
	}

	var confirmed bool
	widgets.ShowPreview(a.win, plan, i18n.T("gui.run.assign"), func() {
		confirmed = true
		a.executeRun(in, plan)
	}, func() {
		// onConfirm (above) always runs before onClose for the same button
		// press, so confirmed is already set when the user actually
		// confirmed; executeRun's own completion calls finishRun in that
		// case. Otherwise (cancel/escape) nothing else will, so do it here.
		if !confirmed {
			a.finishRun()
		}
	})
}

// executeRun applies plan (built from in) and saves config.xml (with
// backup). in and plan are exactly what the just-dismissed preview showed;
// the caller (continueRun) guarantees a.running is already true and that no
// other run can be in flight, so this never re-reads a.inputs, which may
// already have been replaced by a newer startRun by the time this runs.
func (a *App) executeRun(in *pipeline.Inputs, plan *assign.Plan) {
	a.progress.Reset()

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

// finishRun ends the busy flow started by startRun: clears a.running and
// re-enables the run/profile controls. It runs from every path that ends
// the flow without handing off to the next step (load failure, no inputs,
// a dismissed/cancelled preview) and from the run itself completing.
func (a *App) finishRun() {
	a.runMu.Lock()
	a.running = false
	a.runMu.Unlock()
	a.setBusy(false)
	a.progress.SetIndeterminate(false)
}

// isRunning reports whether a run/preview flow is currently in progress.
// Safe to call from any goroutine.
func (a *App) isRunning() bool {
	a.runMu.Lock()
	defer a.runMu.Unlock()
	return a.running
}

// setRunButtonsEnabled enables/disables just the Preview/Assign/Undo/Open
// buttons. setBusy also covers the profile controls; prefer it for the
// run/preview flow. Kept separate because tests and the undo flow only
// need the run buttons.
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

// setBusy disables (or re-enables) everything that could otherwise
// interfere with the current run/preview flow: the run buttons plus the
// profile Select and New/Rename/Delete buttons, since switching or
// deleting the active profile mid-run would pull a.current (and the
// backing files) out from under it.
func (a *App) setBusy(busy bool) {
	a.setRunButtonsEnabled(!busy)

	if a.profileSelect != nil {
		if busy {
			a.profileSelect.Disable()
		} else {
			a.profileSelect.Enable()
		}
	}
	for _, b := range []*widget.Button{a.newProfileBtn, a.renameProfileBtn, a.deleteProfileBtn} {
		if b == nil {
			continue
		}
		if busy {
			b.Disable()
		} else {
			b.Enable()
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
			a.setInputs(in)
			a.reviewTable.SetAssignments(pipeline.Assignments(in))
		})
	}()
}

func (a *App) reviewImagePath(asn assign.Assignment) string {
	in := a.getInputs()
	if in == nil {
		return ""
	}
	return pipeline.ImageFile(in, asn)
}

// reviewReroll is called from widgets.ReviewTable's own goroutine (its
// Reroll button), concurrently with the UI thread possibly clearing
// a.inputs (applyProfile). Take a single snapshot via getInputs and use
// only that, rather than reading a.inputs more than once.
func (a *App) reviewReroll(p rtf.Player) (assign.Assignment, error) {
	in := a.getInputs()
	if in == nil {
		return assign.Assignment{}, errors.New("no data loaded: use \"Load current mappings\" or run a preview/assign first")
	}
	asn, err := pipeline.Reroll(in, p, a.backupBaseDir())
	if err == nil {
		a.logf("rerolled %s (%s) -> %s", p.Name, p.ID, asn.Image)
	}
	return asn, err
}
