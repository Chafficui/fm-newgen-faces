package cmd

import (
	"github.com/spf13/cobra"

	"fmnewgenfaces/gui"
)

var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Launch the desktop application (default)",
	Run:   func(cmd *cobra.Command, args []string) { gui.Run() },
}
