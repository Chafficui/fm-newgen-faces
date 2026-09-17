package cmd

import (
	"github.com/spf13/cobra"

	"fmnewgenfaces/gui"
)

func newGUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gui",
		Short: "Launch the desktop application (default)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			gui.Run()
			return nil
		},
	}
}
