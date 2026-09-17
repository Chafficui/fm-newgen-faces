package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"fmnewgenfaces/internal/core/profile"
)

// openStore resolves the profile directory (configDir, or
// profile.DefaultDir() when empty) and opens the profile store there. It
// returns the resolved directory alongside the store since callers also use
// it to locate the backups folder.
func openStore(configDir string) (*profile.Store, string, error) {
	dir := configDir
	if dir == "" {
		d, err := profile.DefaultDir()
		if err != nil {
			return nil, "", err
		}
		dir = d
	}
	store, err := profile.Open(dir)
	if err != nil {
		return nil, "", err
	}
	return store, dir, nil
}

// backupBaseDir is the root directory config.xml backups are kept under, for
// a profile store resolved to dir.
func backupBaseDir(dir string) string {
	return filepath.Join(dir, "backups")
}

// resolveProfile picks the profile named or sluged by name. When name is
// empty it falls back to State.LastProfile, then the only profile, and
// finally errors if the choice is ambiguous or there are none.
func resolveProfile(store *profile.Store, name string) (*profile.Profile, error) {
	list := store.List()

	if name != "" {
		for _, p := range list {
			if p.Slug == name || strings.EqualFold(p.Name, name) {
				return p, nil
			}
		}
		return nil, fmt.Errorf("no profile matches %q", name)
	}

	if st := store.LoadState(); st.LastProfile != "" {
		if p, ok := store.Get(st.LastProfile); ok {
			return p, nil
		}
	}

	switch len(list) {
	case 0:
		return nil, errors.New("no profiles exist; create one with \"profiles create <name>\" or pass --pack/--rtf/--config/--fm directly")
	case 1:
		return list[0], nil
	default:
		return nil, fmt.Errorf("multiple profiles exist (%d); specify --profile <name or slug>", len(list))
	}
}

// settingsFlags are the flags shared by commands that resolve profile.Settings
// either from a profile or from explicit paths.
type settingsFlags struct {
	profileName string
	pack        string
	rtf         string
	config      string
	fm          string
}

// addSettingsFlags registers --profile, --pack, --rtf, --config and --fm on cmd.
func addSettingsFlags(cmd *cobra.Command, f *settingsFlags) {
	cmd.Flags().StringVar(&f.profileName, "profile", "", "profile name or slug (default: last used, or the only profile)")
	cmd.Flags().StringVar(&f.pack, "pack", "", "face pack folder (used instead of --profile)")
	cmd.Flags().StringVar(&f.rtf, "rtf", "", "newgen export (RTF) file (used instead of --profile)")
	cmd.Flags().StringVar(&f.config, "config", "", "config.xml path (used instead of --profile)")
	cmd.Flags().StringVar(&f.fm, "fm", "", "Football Manager version, e.g. 2024 (used instead of --profile)")
}

// explicit reports whether any of the direct-path flags were given, in which
// case they take over from profile resolution entirely.
func (f settingsFlags) explicit() bool {
	return f.pack != "" || f.rtf != "" || f.config != "" || f.fm != ""
}

// resolveSettings resolves profile.Settings either from the explicit flags or
// from a profile (see settingsFlags.explicit). profile is nil when the
// explicit flags were used.
func resolveSettings(store *profile.Store, f settingsFlags) (profile.Settings, *profile.Profile, error) {
	if f.explicit() {
		s := profile.DefaultSettings()
		s.PackDir = f.pack
		s.RTFPath = f.rtf
		s.ConfigXML = f.config
		if f.fm != "" {
			s.FMVersion = f.fm
		}
		return s, nil, nil
	}

	p, err := resolveProfile(store, f.profileName)
	if err != nil {
		return profile.Settings{}, nil, err
	}
	return p.Settings, p, nil
}

// writeTable prints an aligned table (tab-separated headers/rows rendered
// through text/tabwriter) to w.
func writeTable(w io.Writer, headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, r := range rows {
		fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	tw.Flush()
}

// isInteractive reports whether os.Stdin is a terminal (character device),
// used to decide whether to show a confirmation prompt.
func isInteractive() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// confirm prints prompt and reads a yes/no answer from os.Stdin.
func confirm(cmd *cobra.Command, prompt string) bool {
	fmt.Fprint(cmd.OutOrStdout(), prompt)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}
