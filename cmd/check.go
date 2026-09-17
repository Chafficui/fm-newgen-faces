package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"fmnewgenfaces/internal/core/ethnic"
	"fmnewgenfaces/internal/core/pipeline"
)

func newCheckCmd(configDir *string) *cobra.Command {
	var f settingsFlags
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Validate the current profile's setup",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(cmd, *configDir, f)
		},
	}
	addSettingsFlags(cmd, &f)
	return cmd
}

func runCheck(cmd *cobra.Command, configDir string, f settingsFlags) error {
	store, _, err := openStore(configDir)
	if err != nil {
		return err
	}

	s, _, err := resolveSettings(store, f)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	probs := pipeline.Check(s)
	if len(probs) > 0 {
		for _, p := range probs {
			fmt.Fprintf(out, "[%s] %s\n", p.Key, p.Message)
		}
		return newExitError(1, fmt.Errorf("%d problem(s) found", len(probs)))
	}

	in, err := pipeline.Load(s)
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "Setup looks good.")

	fmt.Fprintf(out, "\nFace pack: %s\n", in.Settings.PackDir)
	for _, e := range ethnic.All {
		folder := in.Pack.Folders[e]
		status := "missing"
		if folder != nil && folder.Exists {
			status = fmt.Sprintf("%d image(s)", len(folder.Images))
		}
		fmt.Fprintf(out, "  %-16s %s\n", e, status)
	}
	if missing := in.Pack.Missing(); len(missing) > 0 {
		names := make([]string, len(missing))
		for i, e := range missing {
			names[i] = string(e)
		}
		fmt.Fprintf(out, "  missing folders: %s\n", strings.Join(names, ", "))
	}

	fmt.Fprintf(out, "\nRTF export: %s\n", in.Settings.RTFPath)
	fmt.Fprintf(out, "  players: %d\n", len(in.RTF.Players))
	if codes := in.RTF.UnmappedCodes(); len(codes) > 0 {
		fmt.Fprintln(out, "  unmapped nations:")
		for _, c := range codes {
			fmt.Fprintf(out, "    %s: %d player(s)\n", c, in.RTF.Unmapped[c])
		}
	}
	fmt.Fprintf(out, "  malformed rows: %d\n", len(in.RTF.Malformed))

	fmt.Fprintf(out, "\nconfig.xml: %s\n", in.Settings.ConfigXML)
	fmt.Fprintf(out, "  existing mappings: %d\n", in.Config.Count())

	if len(in.Warnings) > 0 {
		fmt.Fprintln(out, "\nWarnings:")
		for _, w := range in.Warnings {
			fmt.Fprintf(out, "  - %s\n", w)
		}
	}

	return nil
}
