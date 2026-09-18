// Package cmd wires the command line interface. Running the binary without a
// subcommand launches the GUI.
package cmd

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"fmnewgenfaces/gui"
	"fmnewgenfaces/internal/brand"
	"fmnewgenfaces/internal/i18n"
)

// newRootCmd builds a fresh command tree. It is a constructor (rather than a
// package-level var) so every invocation, and every test, starts with clean
// flag state.
func newRootCmd() *cobra.Command {
	var configDir string
	var lang string

	root := &cobra.Command{
		Use:          brand.BinaryName,
		Short:        brand.AppName + " – faces for Football Manager newgens",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			i18n.Init(lang)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			gui.Run()
			return nil
		},
	}

	root.PersistentFlags().StringVar(&configDir, "config-dir", "", "profile and backup directory (default: the OS user config directory)")
	root.PersistentFlags().StringVar(&lang, "lang", "", "UI language code (e.g. en, de)")

	root.AddCommand(
		newGUICmd(),
		newVersionCmd(),
		newProfilesCmd(&configDir),
		newDetectCmd(),
		newCheckCmd(&configDir),
		newAssignCmd(&configDir),
		newRestoreCmd(&configDir),
	)

	return root
}

// Execute runs the root command and exits the process with the code carried
// by an *exitError (see errors.go), or 1 for any other error.
func Execute() {
	// Cobra refuses to run when a Windows binary is started by double-click
	// from Explorer ("This is a command line tool ..."). This IS a desktop
	// application, so disable that check; the default action launches the GUI.
	cobra.MousetrapHelpText = ""

	err := newRootCmd().Execute()
	if err == nil {
		return
	}
	var ee *exitError
	if errors.As(err, &ee) {
		os.Exit(ee.code)
	}
	os.Exit(1)
}
