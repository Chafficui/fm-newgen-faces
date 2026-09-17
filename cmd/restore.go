package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"fmnewgenfaces/internal/core/fmconfig"
)

func newRestoreCmd(configDir *string) *cobra.Command {
	var f settingsFlags
	var (
		list   bool
		to     string
		latest bool
	)

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "List or restore config.xml backups",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRestore(cmd, *configDir, f, list, to, latest)
		},
	}
	cmd.Flags().StringVar(&f.profileName, "profile", "", "profile name or slug (default: last used, or the only profile)")
	cmd.Flags().StringVar(&f.config, "config", "", "config.xml path (used instead of --profile)")
	cmd.Flags().BoolVar(&list, "list", false, "list available backups for the resolved config.xml")
	cmd.Flags().StringVar(&to, "to", "", "restore from this backup file")
	cmd.Flags().BoolVar(&latest, "latest", false, "restore from the most recent backup")
	return cmd
}

func runRestore(cmd *cobra.Command, configDir string, f settingsFlags, list bool, to string, latest bool) error {
	if !list && to == "" && !latest {
		return fmt.Errorf("restore: pass --list, --to <backup>, or --latest")
	}

	store, dir, err := openStore(configDir)
	if err != nil {
		return err
	}

	configPath := f.config
	if configPath == "" {
		p, perr := resolveProfile(store, f.profileName)
		if perr != nil {
			return perr
		}
		configPath = p.Settings.ConfigXML
	}
	if configPath == "" {
		return fmt.Errorf("restore: no config.xml path (pass --config, or select a profile that has one)")
	}

	out := cmd.OutOrStdout()
	backupDir := fmconfig.BackupDirFor(backupBaseDir(dir), configPath)

	if list {
		backups, err := fmconfig.ListBackups(backupDir)
		if err != nil {
			return err
		}
		if len(backups) == 0 {
			fmt.Fprintf(out, "No backups for %s.\n", configPath)
		} else {
			rows := make([][]string, 0, len(backups))
			for _, b := range backups {
				rows = append(rows, []string{b.Path, b.Time.Format("2006-01-02 15:04:05"), fmt.Sprintf("%d", b.Size)})
			}
			writeTable(out, []string{"BACKUP", "TIME", "SIZE"}, rows)
		}
		if to == "" && !latest {
			return nil
		}
	}

	var backupPath string
	switch {
	case latest:
		backups, err := fmconfig.ListBackups(backupDir)
		if err != nil {
			return err
		}
		if len(backups) == 0 {
			return fmt.Errorf("restore: no backups found for %s", configPath)
		}
		backupPath = backups[0].Path
	case to != "":
		backupPath = to
	default:
		return nil
	}

	if _, err := os.Stat(configPath); err == nil {
		if err := copyFile(configPath, configPath+".before-restore"); err != nil {
			return fmt.Errorf("restore: backing up current config.xml: %w", err)
		}
	}

	if err := fmconfig.Restore(backupPath, configPath); err != nil {
		return err
	}
	fmt.Fprintf(out, "restored %s from %s (previous copy: %s)\n", configPath, backupPath, configPath+".before-restore")
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
