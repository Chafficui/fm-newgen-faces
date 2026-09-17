// Package cmd wires the command line interface. Running the binary without a
// subcommand launches the GUI.
package cmd

import (
	"log"

	"github.com/spf13/cobra"

	"fmnewgenfaces/gui"
	"fmnewgenfaces/internal/brand"
)

var rootCmd = &cobra.Command{
	Use:   brand.BinaryName,
	Short: brand.AppName + " – faces for Football Manager newgens",
	Run:   func(cmd *cobra.Command, args []string) { gui.Run() },
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}

func init() {
	rootCmd.AddCommand(guiCmd)
}
