package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"fmnewgenfaces/internal/core/profile"
)

func newProfilesCmd(configDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profiles",
		Short: "List and manage profiles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfilesList(cmd, *configDir)
		},
	}
	cmd.AddCommand(newProfilesCreateCmd(configDir))
	cmd.AddCommand(newProfilesDeleteCmd(configDir))
	cmd.AddCommand(newProfilesShowCmd(configDir))
	return cmd
}

func runProfilesList(cmd *cobra.Command, configDir string) error {
	store, _, err := openStore(configDir)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	list := store.List()
	if len(list) == 0 {
		fmt.Fprintln(out, "No profiles yet. Create one with \"profiles create <name>\".")
		return nil
	}

	rows := make([][]string, 0, len(list))
	for _, p := range list {
		rows = append(rows, []string{p.Slug, p.Name, p.Settings.FMVersion, p.Settings.PackDir, p.GamePath})
	}
	writeTable(out, []string{"SLUG", "NAME", "FM VERSION", "PACK DIR", "GAME PATH"}, rows)
	return nil
}

func newProfilesCreateCmd(configDir *string) *cobra.Command {
	var gamePath string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := openStore(*configDir)
			if err != nil {
				return err
			}
			p, err := profile.Create(store, args[0], gamePath, nil)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created profile %q (slug %q)\n", p.Name, p.Slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&gamePath, "game", "", "FM install path this profile belongs to")
	return cmd
}

func newProfilesDeleteCmd(configDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <slug>",
		Short: "Delete a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := openStore(*configDir)
			if err != nil {
				return err
			}
			if err := store.Delete(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted profile %q\n", args[0])
			return nil
		},
	}
}

func newProfilesShowCmd(configDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "show <slug>",
		Short: "Print a profile's settings as JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := openStore(*configDir)
			if err != nil {
				return err
			}
			p, ok := store.Get(args[0])
			if !ok {
				return fmt.Errorf("profile %q not found", args[0])
			}
			data, err := json.MarshalIndent(p.Settings, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
}
