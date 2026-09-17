package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"fmnewgenfaces/internal/brand"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), brand.AppName+" "+brand.Version)
			return nil
		},
	}
}
