package gui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2/dialog"

	"fmnewgenfaces/assets"
	"fmnewgenfaces/gui/widgets"
	"fmnewgenfaces/internal/core/ethnic"
	"fmnewgenfaces/internal/core/facepack"
	"fmnewgenfaces/internal/core/fmconfig"
	"fmnewgenfaces/internal/core/fminstall"
	"fmnewgenfaces/internal/core/fmversion"
	"fmnewgenfaces/internal/core/profile"
	"fmnewgenfaces/internal/core/rtf"
	"fmnewgenfaces/internal/i18n"
)

const evalDebounce = 300 * time.Millisecond

// evaluate schedules a debounced, off-UI-thread checklist refresh. Calling it
// again before the debounce elapses supersedes the pending one.
func (a *App) evaluate() {
	if a.current == nil {
		return
	}
	settings := cloneSettings(a.current.Settings)

	a.evalMu.Lock()
	if a.evalTimer != nil {
		a.evalTimer.Stop()
	}
	a.evalGen++
	gen := a.evalGen
	a.evalTimer = time.AfterFunc(evalDebounce, func() {
		a.runEvaluate(gen, settings)
	})
	a.evalMu.Unlock()
}

// evaluateSync computes and applies the checklist immediately, without
// debouncing. Used at startup (after the first profile loads) and by tests.
func (a *App) evaluateSync() {
	if a.current == nil {
		return
	}
	settings := cloneSettings(a.current.Settings)
	a.evalMu.Lock()
	a.evalGen++
	gen := a.evalGen
	a.evalMu.Unlock()
	a.runEvaluate(gen, settings)
}

func cloneSettings(s profile.Settings) profile.Settings {
	out := s
	out.Overrides = make(map[string]string, len(s.Overrides))
	for k, v := range s.Overrides {
		out.Overrides[k] = v
	}
	return out
}

func (a *App) runEvaluate(gen uint64, settings profile.Settings) {
	rows, pack := a.computeChecklist(settings)

	a.evalMu.Lock()
	stale := gen != a.evalGen
	a.evalMu.Unlock()
	if stale {
		return
	}

	doUI(func() {
		a.checklist.SetRows(rows)
		a.packTable.SetPack(pack)
	})
}

func versionForSettings(s profile.Settings) fmversion.Version {
	if v, ok := fmversion.Lookup(s.FMVersion); ok {
		return v
	}
	return fmversion.Default()
}

// computeChecklist does all the filesystem/parsing work for the setup
// checklist. It never touches the UI directly; callers apply the result via
// fyne.Do.
func (a *App) computeChecklist(settings profile.Settings) ([]widgets.ChecklistRow, *facepack.Pack) {
	pack, packRow := a.checkPack(settings)
	cfg, configRow := a.checkConfig(settings)
	rtfRow := a.checkRTF(settings, cfg)
	versionRow := a.checkVersion(settings)
	installRow := a.checkInstall(settings)

	return []widgets.ChecklistRow{packRow, configRow, rtfRow, versionRow, installRow}, pack
}

func (a *App) checkPack(settings profile.Settings) (*facepack.Pack, widgets.ChecklistRow) {
	row := widgets.ChecklistRow{Key: "pack", Title: i18n.T("gui.checklist.pack_title")}
	if settings.PackDir == "" {
		row.Status = widgets.StatusError
		row.Detail = i18n.T("gui.checklist.pack_not_set")
		row.Action = i18n.T("gui.checklist.action_browse")
		row.OnAction = a.actionBrowsePack
		return nil, row
	}

	pack, err := facepack.Scan(settings.PackDir)
	if err != nil {
		row.Status = widgets.StatusError
		row.Detail = i18n.T("gui.checklist.pack_not_found", settings.PackDir)
		row.Action = i18n.T("gui.checklist.action_browse")
		row.OnAction = a.actionBrowsePack
		return nil, row
	}

	missing := pack.Missing()
	warns := pack.Warnings()
	switch {
	case len(missing) == len(ethnic.All):
		row.Status = widgets.StatusError
		row.Detail = i18n.T("gui.checklist.pack_no_folders")
		row.Action = i18n.T("gui.checklist.action_browse")
		row.OnAction = a.actionBrowsePack
	case len(missing) > 0 || len(warns) > 0:
		row.Status = widgets.StatusWarn
		row.Detail = i18n.T("gui.checklist.pack_warn", joinDetails(missingNames(missing), warns))
	default:
		row.Status = widgets.StatusOK
		row.Detail = i18n.T("gui.checklist.pack_ok", len(ethnic.All)-len(pack.Missing()), len(ethnic.All), pack.TotalImages)
	}
	return pack, row
}

func missingNames(missing []ethnic.Ethnic) []string {
	out := make([]string, len(missing))
	for i, e := range missing {
		out[i] = i18n.T("gui.checklist.pack_missing_folder", string(e))
	}
	return out
}

func joinDetails(parts ...[]string) string {
	var all []string
	for _, p := range parts {
		all = append(all, p...)
	}
	return strings.Join(all, "; ")
}

func (a *App) checkConfig(settings profile.Settings) (*fmconfig.Config, widgets.ChecklistRow) {
	row := widgets.ChecklistRow{Key: "config", Title: i18n.T("gui.checklist.config_title")}
	if settings.ConfigXML == "" {
		row.Status = widgets.StatusWarn
		row.Detail = i18n.T("gui.checklist.config_not_set")
		return nil, row
	}

	version := versionForSettings(settings)
	if _, err := os.Stat(settings.ConfigXML); err != nil {
		if os.IsNotExist(err) {
			row.Status = widgets.StatusWarn
			row.Detail = i18n.T("gui.checklist.config_missing")
			row.Action = i18n.T("gui.checklist.action_create_now")
			row.OnAction = a.actionCreateConfigNow
			return nil, row
		}
		row.Status = widgets.StatusError
		row.Detail = i18n.T("gui.checklist.config_error", err)
		return nil, row
	}

	cfg, err := fmconfig.Load(settings.ConfigXML, version)
	if err != nil {
		row.Status = widgets.StatusError
		row.Detail = i18n.T("gui.checklist.config_error", err)
		return nil, row
	}
	row.Status = widgets.StatusOK
	row.Detail = i18n.T("gui.checklist.config_ok", cfg.Count())
	return cfg, row
}

func (a *App) checkRTF(settings profile.Settings, cfg *fmconfig.Config) widgets.ChecklistRow {
	row := widgets.ChecklistRow{Key: "rtf", Title: i18n.T("gui.checklist.rtf_title")}
	if settings.RTFPath == "" {
		// An export already sitting in the face pack folder is adopted
		// automatically (the common "save newgen.rtf next to the faces" case).
		if found, ok := findRTFInDir(settings.PackDir); ok {
			settings.RTFPath = found
			a.adoptRTFPath(found)
		}
	}
	if settings.RTFPath == "" {
		row.Status = widgets.StatusError
		row.Detail = i18n.T("gui.checklist.rtf_not_set")
		row.Action = i18n.T("gui.checklist.action_how_to_export")
		row.OnAction = a.actionShowExportHelp
		return row
	}
	if st, err := os.Stat(settings.RTFPath); err != nil || st.IsDir() {
		row.Status = widgets.StatusError
		row.Detail = i18n.T("gui.checklist.rtf_missing", settings.RTFPath)
		row.Action = i18n.T("gui.checklist.action_how_to_export")
		row.OnAction = a.actionShowExportHelp
		return row
	}

	resolver, _ := ethnic.NewResolver(settings.Overrides)
	res, err := rtf.Parse(settings.RTFPath, resolver)
	if err != nil {
		row.Status = widgets.StatusError
		row.Detail = i18n.T("gui.checklist.rtf_parse_error", err)
		row.Action = i18n.T("gui.checklist.action_how_to_export")
		row.OnAction = a.actionShowExportHelp
		return row
	}

	kept := 0
	if cfg != nil {
		for _, p := range res.Players {
			if cfg.Has(p.ID) {
				kept++
			}
		}
	}

	var extra []string
	if len(res.Malformed) > 0 {
		extra = append(extra, i18n.T("gui.checklist.rtf_malformed", len(res.Malformed)))
	}
	if res.Duplicates > 0 {
		extra = append(extra, i18n.T("gui.checklist.rtf_duplicates", res.Duplicates))
	}

	if len(res.UnmappedPlayers) > 0 {
		row.Status = widgets.StatusWarn
		row.Detail = i18n.T("gui.checklist.rtf_warn_unmapped", len(res.UnmappedPlayers))
	} else {
		row.Status = widgets.StatusOK
		row.Detail = i18n.T("gui.checklist.rtf_ok", len(res.Players), kept)
	}
	if len(extra) > 0 {
		row.Detail = row.Detail + " (" + strings.Join(extra, "; ") + ")"
	}
	return row
}

func (a *App) checkVersion(settings profile.Settings) widgets.ChecklistRow {
	row := widgets.ChecklistRow{Key: "version", Title: i18n.T("gui.checklist.version_title")}
	version := versionForSettings(settings)

	if settings.PackDir != "" {
		if detected, ok := fmversion.FromPath(settings.PackDir); ok && detected.Year != version.Year {
			row.Status = widgets.StatusWarn
			row.Detail = i18n.T("gui.checklist.version_mismatch", detected.Display(), version.Display())
			row.Action = i18n.T("gui.checklist.action_use_detected")
			year := detected.Year
			row.OnAction = func() { a.actionUseDetectedVersion(year) }
			return row
		}
	}
	row.Status = widgets.StatusOK
	row.Detail = i18n.T("gui.checklist.version_ok", version.Display())
	return row
}

// distributedFileNames returns the *.fmf file names embedded under
// assets/<dir> (mirrors fminstall.Distribute's own filter).
func distributedFileNames(dir string) []string {
	entries, err := assets.FS.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".fmf") {
			out = append(out, e.Name())
		}
	}
	return out
}

func (a *App) checkInstall(settings profile.Settings) widgets.ChecklistRow {
	row := widgets.ChecklistRow{Key: "install", Title: i18n.T("gui.checklist.install_title")}
	if settings.PackDir == "" {
		row.Status = widgets.StatusPending
		row.Detail = i18n.T("gui.checklist.install_unknown")
		return row
	}

	inst, ok := fminstall.FromPath(settings.PackDir)
	if !ok {
		row.Status = widgets.StatusPending
		row.Detail = i18n.T("gui.checklist.install_custom")
		return row
	}

	var missing []string
	for _, name := range distributedFileNames("views") {
		if _, err := os.Stat(filepath.Join(inst.ViewsDir(), name)); err != nil {
			missing = append(missing, name)
		}
	}
	for _, name := range distributedFileNames("filters") {
		if _, err := os.Stat(filepath.Join(inst.FiltersDir(), name)); err != nil {
			missing = append(missing, name)
		}
	}

	if len(missing) == 0 {
		row.Status = widgets.StatusOK
		row.Detail = i18n.T("gui.checklist.install_ok", inst.Name)
		return row
	}
	row.Status = widgets.StatusWarn
	row.Detail = i18n.T("gui.checklist.install_warn", inst.Name)
	row.Action = i18n.T("gui.checklist.action_install")
	instCopy := inst
	row.OnAction = func() { a.actionInstallViewFilter(instCopy) }
	return row
}

// --- checklist row actions ---

func (a *App) actionBrowsePack() {
	start := ""
	if a.current != nil {
		start = a.current.Settings.PackDir
	}
	path := pickFolder(start)
	if path == "" {
		return
	}
	a.packRow.SetText(path)
	a.onPackDirChanged(path)
}

func (a *App) actionCreateConfigNow() {
	if a.current == nil || a.current.Settings.ConfigXML == "" {
		return
	}
	path := a.current.Settings.ConfigXML
	go func() {
		err := fmconfig.Generate(path)
		doUI(func() {
			if err != nil {
				a.errorf("create config.xml: %v", err)
				dialog.ShowError(err, a.win)
				return
			}
			a.logf("created %s", path)
			a.evaluate()
		})
	}()
}

func (a *App) actionUseDetectedVersion(year string) {
	disp := displayForVersion(year)
	if disp == "" {
		return
	}
	a.loading = true
	a.versionSelect.SetSelected(disp)
	a.loading = false
	a.settingsChanged()
}

func (a *App) actionInstallViewFilter(inst fminstall.Install) {
	go func() {
		res, err := fminstall.Distribute(inst)
		doUI(func() {
			if err != nil {
				a.errorf("install view/filter: %v", err)
				dialog.ShowError(err, a.win)
				return
			}
			a.logf("installed view/filter into %s (%d copied, %d skipped)", inst.Name, len(res.Copied), len(res.Skipped))
			a.evaluate()
		})
	}()
}

func (a *App) actionShowExportHelp() {
	a.showHelp()
}

// adoptRTFPath stores an auto-detected RTF path in the current profile and
// reflects it in the Setup tab without firing the change handlers.
func (a *App) adoptRTFPath(path string) {
	doUI(func() {
		if a.current == nil || a.current.Settings.RTFPath == path {
			return
		}
		a.current.Settings.RTFPath = path
		if a.rtfRow != nil {
			a.rtfRow.SetText(path)
		}
		a.autosaver.Trigger(a.current)
		a.logf("using newgen export found in the face pack folder: %s", path)
	})
}
