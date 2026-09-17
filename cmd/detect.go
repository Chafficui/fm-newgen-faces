package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"fmnewgenfaces/internal/core/fminstall"
)

func newDetectCmd() *cobra.Command {
	var installViews bool
	cmd := &cobra.Command{
		Use:   "detect",
		Short: "List Football Manager installations found on this machine",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			installs := fminstall.Detect()
			if len(installs) == 0 {
				fmt.Fprintln(out, "No Football Manager installations found. Pass --pack/--rtf/--config to \"check\"/\"assign\" directly, or create a profile with \"profiles create\".")
				return nil
			}

			rows := make([][]string, 0, len(installs))
			for _, inst := range installs {
				rows = append(rows, []string{inst.Name, inst.Version.Display(), inst.BasePath, inst.Source})
			}
			writeTable(out, []string{"NAME", "VERSION", "BASE PATH", "SOURCE"}, rows)

			if installViews {
				fmt.Fprintln(out)
				for _, inst := range installs {
					res, err := fminstall.Distribute(inst)
					if err != nil {
						return fmt.Errorf("install views for %s: %w", inst.Name, err)
					}
					fmt.Fprintf(out, "%s: copied %d, skipped %d\n", inst.Name, len(res.Copied), len(res.Skipped))
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&installViews, "install-views", false, "install the bundled view and filter into every installation found")
	return cmd
}
