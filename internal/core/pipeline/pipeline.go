// Package pipeline is the thin glue the GUI and the CLI share: it loads all
// inputs for a profile's settings, builds a plan, applies it and saves with a
// backup. It has no UI and reports progress through callbacks.
package pipeline

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"fmnewgenfaces/internal/core/assign"
	"fmnewgenfaces/internal/core/ethnic"
	"fmnewgenfaces/internal/core/facepack"
	"fmnewgenfaces/internal/core/fmconfig"
	"fmnewgenfaces/internal/core/fmversion"
	"fmnewgenfaces/internal/core/profile"
	"fmnewgenfaces/internal/core/rtf"
)

// KeepBackups is how many config.xml backups are kept per config file.
const KeepBackups = 10

// Inputs is everything loaded for a run.
type Inputs struct {
	Settings profile.Settings
	Version  fmversion.Version
	Pack     *facepack.Pack
	Resolver *ethnic.Resolver
	RTF      *rtf.Result
	Config   *fmconfig.Config
	// Warnings are non-fatal problems (invalid overrides, pack warnings,
	// malformed RTF rows, duplicates), already formatted for display.
	Warnings []string
}

// Problem is a blocking issue found by Check.
type Problem struct {
	Key     string // "pack", "config", "rtf", "version"
	Message string
}

func (p Problem) Error() string { return p.Key + ": " + p.Message }

// Check validates s without parsing the RTF fully: pack dir exists and looks
// like a pack, config path is inside an existing directory, RTF exists,
// version known. It returns all problems, not just the first.
func Check(s profile.Settings) []Problem {
	var out []Problem
	if s.PackDir == "" {
		out = append(out, Problem{"pack", "no face pack folder selected"})
	} else if st, err := os.Stat(s.PackDir); err != nil || !st.IsDir() {
		out = append(out, Problem{"pack", "face pack folder does not exist: " + s.PackDir})
	} else if !facepack.LooksLikePack(s.PackDir, 1) {
		out = append(out, Problem{"pack", "folder has no ethnic subfolders (African, Asian, …): " + s.PackDir})
	}

	if s.ConfigXML == "" {
		out = append(out, Problem{"config", "no config.xml path set"})
	} else if st, err := os.Stat(filepath.Dir(s.ConfigXML)); err != nil || !st.IsDir() {
		out = append(out, Problem{"config", "folder for config.xml does not exist: " + filepath.Dir(s.ConfigXML)})
	}

	if s.PackDir != "" && s.ConfigXML != "" {
		pv, cv := filepath.VolumeName(absOrSelf(s.PackDir)), filepath.VolumeName(absOrSelf(s.ConfigXML))
		if !strings.EqualFold(pv, cv) {
			out = append(out, Problem{"config", "config.xml must be on the same drive as the face pack (FM needs relative image paths): " + pv + " vs " + cv})
		}
	}

	if s.RTFPath == "" {
		out = append(out, Problem{"rtf", "no newgen export (RTF) selected"})
	} else if st, err := os.Stat(s.RTFPath); err != nil || st.IsDir() {
		out = append(out, Problem{"rtf", "RTF file does not exist: " + s.RTFPath})
	}

	if _, ok := versionOf(s); !ok {
		out = append(out, Problem{"version", "unknown Football Manager version: " + s.FMVersion})
	}
	return out
}

func absOrSelf(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

func versionOf(s profile.Settings) (fmversion.Version, bool) {
	if s.FMVersion == "" {
		return fmversion.Default(), true
	}
	if v, ok := fmversion.Lookup(s.FMVersion); ok {
		return v, true
	}
	// Accept an unknown but plausible year (e.g. a new release) the same way
	// fmversion.FromPath does.
	if v, ok := fmversion.FromPath("Football Manager " + s.FMVersion); ok {
		return v, true
	}
	return fmversion.Version{}, false
}

// Load scans the pack, builds the resolver, parses the RTF and loads (or
// creates in memory) the config. It fails only on unrecoverable errors
// (unreadable pack root, unreadable RTF, malformed config.xml).
func Load(s profile.Settings) (*Inputs, error) {
	if probs := Check(s); len(probs) > 0 {
		msgs := make([]string, len(probs))
		for i, p := range probs {
			msgs[i] = p.Message
		}
		return nil, errors.New(strings.Join(msgs, "; "))
	}
	var err error
	if s.PackDir, err = filepath.Abs(s.PackDir); err != nil {
		return nil, err
	}
	if s.ConfigXML, err = filepath.Abs(s.ConfigXML); err != nil {
		return nil, err
	}
	if s.RTFPath, err = filepath.Abs(s.RTFPath); err != nil {
		return nil, err
	}

	in := &Inputs{Settings: s}
	in.Version, _ = versionOf(s)

	if in.Pack, err = facepack.Scan(s.PackDir); err != nil {
		return nil, fmt.Errorf("face pack: %w", err)
	}
	for _, m := range in.Pack.Missing() {
		in.Warnings = append(in.Warnings, fmt.Sprintf("face pack: folder %q is missing – players of that group will be skipped", m))
	}
	in.Warnings = append(in.Warnings, in.Pack.Warnings()...)

	var oerr *ethnic.OverrideError
	in.Resolver, err = ethnic.NewResolver(s.Overrides)
	if errors.As(err, &oerr) {
		codes := make([]string, 0, len(oerr.Invalid))
		for c := range oerr.Invalid {
			codes = append(codes, c)
		}
		sort.Strings(codes)
		for _, c := range codes {
			in.Warnings = append(in.Warnings, fmt.Sprintf("override %s = %q ignored: not an ethnic group", c, oerr.Invalid[c]))
		}
	} else if err != nil {
		return nil, err
	}

	if in.RTF, err = rtf.Parse(s.RTFPath, in.Resolver); err != nil {
		return nil, fmt.Errorf("newgen export: %w", err)
	}
	if n := len(in.RTF.Malformed); n > 0 {
		in.Warnings = append(in.Warnings, fmt.Sprintf("%d malformed row(s) in the RTF were skipped (first: line %d, %s)", n, in.RTF.Malformed[0].Line, in.RTF.Malformed[0].Reason))
	}
	if in.RTF.Duplicates > 0 {
		in.Warnings = append(in.Warnings, fmt.Sprintf("%d duplicate player row(s) in the RTF were ignored", in.RTF.Duplicates))
	}

	if in.Config, err = fmconfig.LoadOrNew(s.ConfigXML, in.Version); err != nil {
		return nil, fmt.Errorf("config.xml: %w", err)
	}
	return in, nil
}

// Options derives assign.Options from settings (seed 0).
func Options(s profile.Settings) assign.Options {
	return assign.Options{Preserve: s.Preserve, AllowDuplicates: s.AllowDuplicates}
}

// Plan builds the dry-run plan for in.
func Plan(in *Inputs) *assign.Plan {
	return assign.Build(in.RTF, in.Config, in.Pack, Options(in.Settings))
}

// RunResult is the outcome of Run.
type RunResult struct {
	Result     *assign.Result
	BackupPath string
	ConfigPath string
}

func saveOptions(in *Inputs, backupBaseDir string) fmconfig.SaveOptions {
	opts := fmconfig.SaveOptions{KeepBackups: KeepBackups}
	if backupBaseDir != "" {
		opts.BackupDir = fmconfig.BackupDirFor(backupBaseDir, in.Config.Path)
	}
	return opts
}

// Run applies plan to in.Config and saves it, keeping a backup under
// backupBaseDir (see fmconfig.BackupDirFor) when backupBaseDir != "".
func Run(in *Inputs, plan *assign.Plan, backupBaseDir string, progress assign.Progress) (*RunResult, error) {
	res, err := assign.Apply(plan, in.Config, in.Pack, progress)
	if err != nil {
		return nil, err
	}
	backup, err := in.Config.Save(saveOptions(in, backupBaseDir))
	if err != nil {
		return nil, fmt.Errorf("save config.xml: %w", err)
	}
	return &RunResult{Result: res, BackupPath: backup, ConfigPath: in.Config.Path}, nil
}

// Reroll picks a new image for player p and saves the config (with backup).
func Reroll(in *Inputs, p rtf.Player, backupBaseDir string) (assign.Assignment, error) {
	a, err := assign.Reroll(in.Config, in.Pack, p, Options(in.Settings))
	if err != nil {
		return assign.Assignment{}, err
	}
	if _, err := in.Config.Save(saveOptions(in, backupBaseDir)); err != nil {
		return assign.Assignment{}, fmt.Errorf("save config.xml: %w", err)
	}
	return a, nil
}

// Assignments lists the current newgen mappings of in.Config joined with the
// RTF players (players not in the RTF are listed with an empty Name).
// Players from the RTF come first in RTF order, then the remaining mappings
// sorted by ID.
func Assignments(in *Inputs) []assign.Assignment {
	mapped := in.Config.Newgens()
	seen := make(map[string]bool, len(mapped))
	var out []assign.Assignment
	add := func(p rtf.Player, from string) {
		out = append(out, assign.Assignment{Player: p, Image: baseName(from), From: from})
	}
	for _, p := range in.RTF.Players {
		if from, ok := mapped[p.ID]; ok {
			seen[p.ID] = true
			add(p, from)
		}
	}
	for _, p := range in.RTF.UnmappedPlayers {
		if from, ok := mapped[p.ID]; ok {
			seen[p.ID] = true
			add(p, from)
		}
	}
	rest := make([]string, 0)
	for id := range mapped {
		if !seen[id] {
			rest = append(rest, id)
		}
	}
	sort.Strings(rest)
	for _, id := range rest {
		from := mapped[id]
		p := rtf.Player{ID: id}
		if ref, ok := facepack.ParseRef(from); ok {
			p.Ethnic = ref.Ethnic
		}
		add(p, from)
	}
	return out
}

func baseName(from string) string {
	from = strings.ReplaceAll(from, "\\", "/")
	return from[strings.LastIndex(from, "/")+1:]
}

// ImageFile returns the absolute path of the image behind an assignment
// (first existing extension), or "".
func ImageFile(in *Inputs, a assign.Assignment) string {
	if a.From == "" {
		return ""
	}
	base := filepath.Join(filepath.Dir(in.Config.Path), filepath.FromSlash(a.From))
	for _, ext := range facepack.ImageExtensions {
		for _, candidate := range []string{base + ext, base + strings.ToUpper(ext)} {
			if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
				return candidate
			}
		}
	}
	return ""
}

// OverridesTOML renders overrides as a "[mapping_override]" TOML block, sorted.
func OverridesTOML(overrides map[string]string) string {
	keys := make([]string, 0, len(overrides))
	for k := range overrides {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("[mapping_override]\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "%s = %q\n", k, overrides[k])
	}
	return b.String()
}

// ParseOverridesTOML parses text produced by OverridesTOML (a bare
// "KEY = 'Value'" list without the header is accepted too). Keys are
// upper-cased; values validated with ethnic.Parse; invalid lines error.
func ParseOverridesTOML(text string) (map[string]string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return map[string]string{}, nil
	}
	var doc map[string]any
	if err := toml.Unmarshal([]byte(text), &doc); err != nil {
		return nil, fmt.Errorf("not valid TOML: %w", err)
	}
	raw := doc
	if sub, ok := doc["mapping_override"].(map[string]any); ok {
		raw = sub
	}
	out := make(map[string]string, len(raw))
	var bad []string
	for k, v := range raw {
		s, ok := v.(string)
		if !ok {
			bad = append(bad, k)
			continue
		}
		e, ok := ethnic.Parse(s)
		if !ok {
			bad = append(bad, fmt.Sprintf("%s = %q", k, s))
			continue
		}
		out[strings.ToUpper(strings.TrimSpace(k))] = string(e)
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return out, fmt.Errorf("invalid override(s): %s (valid groups: %s)", strings.Join(bad, ", "), strings.Join(ethnic.Names(), ", "))
	}
	return out, nil
}

// IsNotExist reports whether err means a file was missing.
func IsNotExist(err error) bool { return errors.Is(err, fs.ErrNotExist) }
