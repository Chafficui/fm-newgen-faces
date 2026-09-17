package gui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2/dialog"

	"fmnewgenfaces/gui/widgets"
	"fmnewgenfaces/internal/brand"
	"fmnewgenfaces/internal/core/facepack"
	"fmnewgenfaces/internal/core/fminstall"
	"fmnewgenfaces/internal/core/profile"
	"fmnewgenfaces/internal/core/update"
	"fmnewgenfaces/internal/i18n"
)

const updateCheckPeriod = 24 * time.Hour

// runStartup does all the background work that must not block the first
// paint: detecting FM installs, creating/prefilling profiles for them,
// distributing the bundled view/filter, selecting the initial profile (or
// showing the wizard) and checking for updates.
func (a *App) runStartup() {
	a.detectAndDistributeInstalls()
	a.selectInitialProfile()
	a.runUpdateCheck(false)
}

func (a *App) detectAndDistributeInstalls() {
	for _, inst := range fminstall.Detect() {
		if _, ok := a.store.FindByGamePath(inst.BasePath); !ok {
			a.createProfileForInstall(inst)
		}

		res, err := fminstall.Distribute(inst)
		if err != nil {
			a.warnf("installing view/filter into %s: %v", inst.Name, err)
			continue
		}
		a.logf("%s: %d file(s) installed, %d already up to date", inst.Name, len(res.Copied), len(res.Skipped))
	}
}

func (a *App) createProfileForInstall(inst fminstall.Install) {
	base := profile.DefaultSettings()
	base.FMVersion = inst.Version.Year
	if pack, ok := facepack.Find(inst.GraphicsDir, 3); ok {
		base.PackDir = pack
		base.ConfigXML = filepath.Join(pack, "config.xml")
		if rtfPath, ok := findRTFInDir(pack); ok {
			base.RTFPath = rtfPath
		}
	} else {
		base.ConfigXML = filepath.Join(inst.GraphicsDir, "config.xml")
	}

	name := "FM " + inst.Version.Year
	if _, err := profile.Create(a.store, name, inst.BasePath, &base); err != nil {
		// Name already taken (e.g. two installs of the same FM year from
		// different sources): dedupe with the source and try once more.
		alt := name + " (" + inst.Source + ")"
		if _, err2 := profile.Create(a.store, alt, inst.BasePath, &base); err2 != nil {
			a.warnf("creating profile for %s: %v", inst.Name, err2)
			return
		}
	}
	a.logf("created profile for %s", inst.Name)
}

// selectInitialProfile picks state.LastProfile (or the first profile),
// applies it to the UI, evaluates, and shows the first-run wizard when no
// profile has a pack directory yet.
func (a *App) selectInitialProfile() {
	profiles := a.store.List()

	doUI(func() {
		a.refreshProfileList()

		slug := ""
		for _, p := range profiles {
			if p.Slug == a.state.LastProfile {
				slug = p.Slug
				break
			}
		}
		if slug == "" && len(profiles) > 0 {
			slug = profiles[0].Slug
		}
		if slug != "" {
			a.selectProfile(slug)
		}

		anyPackSet := false
		for _, p := range profiles {
			if p.Settings.PackDir != "" {
				anyPackSet = true
				break
			}
		}
		if !anyPackSet && !a.state.WizardDone {
			a.showWizard()
		}
	})
}

// checkForUpdatesNow runs an update check immediately, ignoring the 24h
// throttle and the skipped-version memory, and always tells the user the
// result.
func (a *App) checkForUpdatesNow() {
	go a.runUpdateCheck(true)
}

func (a *App) runUpdateCheck(force bool) {
	if !force && time.Since(a.state.LastUpdateCheck) < updateCheckPeriod {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rel, err := update.CheckLatest(ctx, brand.RepoOwner, brand.RepoName)
	if err != nil {
		a.warnf("update check failed: %v", err)
		return
	}

	a.state.LastUpdateCheck = time.Now()
	if err := a.store.SaveState(a.state); err != nil {
		a.errorf("saving app state: %v", err)
	}

	if !update.IsNewer(rel.Version, brand.Version) {
		if force {
			doUI(func() {
				dialog.ShowInformation(i18n.T("gui.update.title"), i18n.T("gui.update.up_to_date"), a.win)
			})
		}
		return
	}
	if !force && rel.Version == a.state.SkippedVersion {
		return
	}

	doUI(func() {
		a.showBanner(widgets.NewUpdateBanner(rel.Version, func() {
			if err := a.openURL(rel.URL); err != nil {
				a.errorf("opening release page: %v", err)
			}
		}, func() {
			a.state.SkippedVersion = rel.Version
			if err := a.store.SaveState(a.state); err != nil {
				a.errorf("saving app state: %v", err)
			}
		}))
	})
}

// findRTFInDir returns "<dir>/newgen.rtf" when it exists, otherwise the most
// recently modified *.rtf file in dir (case-insensitive extension).
func findRTFInDir(dir string) (string, bool) {
	if dir == "" {
		return "", false
	}
	preferred := filepath.Join(dir, "newgen.rtf")
	if st, err := os.Stat(preferred); err == nil && !st.IsDir() {
		return preferred, true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	var best string
	var bestTime time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".rtf") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if best == "" || info.ModTime().After(bestTime) {
			best, bestTime = filepath.Join(dir, e.Name()), info.ModTime()
		}
	}
	return best, best != ""
}
