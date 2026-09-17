package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"fmnewgenfaces/internal/core/assign"
	"fmnewgenfaces/internal/core/ethnic"
	"fmnewgenfaces/internal/core/pipeline"
)

func newAssignCmd(configDir *string) *cobra.Command {
	var f settingsFlags
	var (
		dryRun       bool
		preserve     bool
		noPreserve   bool
		duplicates   bool
		noDuplicates bool
		seed         int64
		noBackup     bool
		jsonOut      bool
		yes          bool
		overrides    []string
	)

	cmd := &cobra.Command{
		Use:   "assign",
		Short: "Assign newgen faces from the face pack",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			o := assignRunOptions{
				dryRun:        dryRun,
				preserveSet:   cmd.Flags().Changed("preserve"),
				noPreserveSet: cmd.Flags().Changed("no-preserve"),
				dupSet:        cmd.Flags().Changed("duplicates"),
				noDupSet:      cmd.Flags().Changed("no-duplicates"),
				seed:          seed,
				noBackup:      noBackup,
				jsonOut:       jsonOut,
				yes:           yes,
				overrides:     overrides,
			}
			return runAssign(cmd, *configDir, f, o)
		},
	}
	addSettingsFlags(cmd, &f)
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show the plan without writing anything")
	cmd.Flags().BoolVar(&preserve, "preserve", false, "keep existing newgen mappings (default: from profile/settings)")
	cmd.Flags().BoolVar(&noPreserve, "no-preserve", false, "reassign every player, including already-mapped ones")
	cmd.Flags().BoolVar(&duplicates, "duplicates", false, "allow the same image for several players (default: from profile/settings)")
	cmd.Flags().BoolVar(&noDuplicates, "no-duplicates", false, "never reuse an image already mapped in config.xml")
	cmd.Flags().Int64Var(&seed, "seed", 0, "random seed for reproducible runs (0 = random)")
	cmd.Flags().BoolVar(&noBackup, "no-backup", false, "do not keep a backup of the previous config.xml")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print the plan and result as one JSON document instead of tables")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	cmd.Flags().StringArrayVar(&overrides, "override", nil, "nation override CODE=Group for this run, repeatable")
	return cmd
}

// assignRunOptions holds the non-settings flags for one assign invocation.
type assignRunOptions struct {
	dryRun        bool
	preserveSet   bool
	noPreserveSet bool
	dupSet        bool
	noDupSet      bool
	seed          int64
	noBackup      bool
	jsonOut       bool
	yes           bool
	overrides     []string
}

func runAssign(cmd *cobra.Command, configDir string, f settingsFlags, o assignRunOptions) error {
	store, dir, err := openStore(configDir)
	if err != nil {
		return err
	}

	s, _, err := resolveSettings(store, f)
	if err != nil {
		return err
	}

	if len(o.overrides) > 0 {
		merged := make(map[string]string, len(s.Overrides)+len(o.overrides))
		for k, v := range s.Overrides {
			merged[k] = v
		}
		for _, raw := range o.overrides {
			code, group, perr := parseOverrideFlag(raw)
			if perr != nil {
				return perr
			}
			merged[code] = group
		}
		s.Overrides = merged
	}

	if o.preserveSet {
		s.Preserve = true
	} else if o.noPreserveSet {
		s.Preserve = false
	}
	if o.dupSet {
		s.AllowDuplicates = true
	} else if o.noDupSet {
		s.AllowDuplicates = false
	}

	out := cmd.OutOrStdout()

	in, err := pipeline.Load(s)
	if err != nil {
		return err
	}

	opt := assign.Options{Preserve: s.Preserve, AllowDuplicates: s.AllowDuplicates, Seed: o.seed}
	plan := assign.Build(in.RTF, in.Config, in.Pack, opt)

	if !o.jsonOut {
		if len(in.Warnings) > 0 {
			fmt.Fprintln(out, "Warnings:")
			for _, w := range in.Warnings {
				fmt.Fprintf(out, "  - %s\n", w)
			}
			fmt.Fprintln(out)
		}
		printPlan(out, in, plan)
	}

	if o.dryRun {
		if o.jsonOut {
			return printAssignJSON(out, in, plan, nil)
		}
		fmt.Fprintln(out, "\nDry run: nothing written.")
		return nil
	}

	if !o.yes && !o.jsonOut && isInteractive() {
		if !confirm(cmd, "\nProceed? [y/N] ") {
			fmt.Fprintln(out, "aborted.")
			return nil
		}
	}

	backupDir := ""
	if !o.noBackup {
		backupDir = backupBaseDir(dir)
	}

	lastPct := -1
	progress := assign.Progress(func(done, total int) {
		if o.jsonOut || total == 0 {
			return
		}
		pct := done * 100 / total / 10
		if pct > lastPct {
			lastPct = pct
			fmt.Fprintf(out, "  %3d%% (%d/%d)\n", pct*10, done, total)
		}
	})

	rr, err := pipeline.Run(in, plan, backupDir, progress)
	if err != nil {
		return err
	}

	if o.jsonOut {
		return printAssignJSON(out, in, plan, rr)
	}

	printSummary(out, rr)

	if len(rr.Result.Skipped) > 0 {
		return newExitError(2, fmt.Errorf("%d player(s) were skipped", len(rr.Result.Skipped)))
	}
	return nil
}

func parseOverrideFlag(raw string) (code, group string, err error) {
	parts := strings.SplitN(raw, "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("invalid --override %q: expected CODE=Group", raw)
	}
	code = strings.ToUpper(strings.TrimSpace(parts[0]))
	e, ok := ethnic.Parse(strings.TrimSpace(parts[1]))
	if !ok {
		return "", "", fmt.Errorf("invalid --override %q: %q is not an ethnic group (valid: %s)", raw, parts[1], strings.Join(ethnic.Names(), ", "))
	}
	return code, string(e), nil
}

func printPlan(out io.Writer, in *pipeline.Inputs, plan *assign.Plan) {
	rows := make([][]string, 0, len(plan.PerEthnic))
	for _, e := range ethnic.All {
		d, ok := plan.PerEthnic[e]
		if !ok {
			continue
		}
		rows = append(rows, []string{
			string(e),
			fmt.Sprintf("%d", d.Needed),
			fmt.Sprintf("%d", d.Available),
			fmt.Sprintf("%d", d.Shortfall(plan.Options.AllowDuplicates)),
		})
	}

	fmt.Fprintln(out, "Plan:")
	if len(rows) > 0 {
		writeTable(out, []string{"ETHNIC", "NEEDED", "AVAILABLE", "SHORTFALL"}, rows)
	}
	fmt.Fprintf(out, "\nTotals: %d new, %d preserved, %d unmapped\n", len(plan.New), len(plan.Preserved), len(plan.Unmapped))

	if codes := in.RTF.UnmappedCodes(); len(codes) > 0 {
		fmt.Fprintln(out, "\nUnmapped nation codes:")
		for _, c := range codes {
			fmt.Fprintf(out, "  %s: %d player(s)\n", c, in.RTF.Unmapped[c])
		}
		fmt.Fprintf(out, "Hint: add --override %s=<Group> to map an unknown nation (repeatable).\n", codes[0])
	}
}

func printSummary(out io.Writer, rr *pipeline.RunResult) {
	fmt.Fprintf(out, "\nAssigned %d, preserved %d", len(rr.Result.Assigned), rr.Result.Preserved)
	if len(rr.Result.Skipped) > 0 {
		fmt.Fprintf(out, ", skipped %d", len(rr.Result.Skipped))
	}
	fmt.Fprintln(out)
	if len(rr.Result.Skipped) > 0 {
		fmt.Fprintln(out, "Skipped:")
		for _, sk := range rr.Result.Skipped {
			fmt.Fprintf(out, "  %s (%s): %s\n", sk.Player.Name, sk.Player.ID, sk.Reason)
		}
	}
	if rr.BackupPath != "" {
		fmt.Fprintf(out, "Backup: %s\n", rr.BackupPath)
	}
	fmt.Fprintf(out, "Config: %s\n", rr.ConfigPath)
}

type demandJSON struct {
	Ethnic    string `json:"ethnic"`
	Needed    int    `json:"needed"`
	Available int    `json:"available"`
	Shortfall int    `json:"shortfall"`
}

type planJSON struct {
	New            int            `json:"new"`
	Preserved      int            `json:"preserved"`
	Unmapped       int            `json:"unmapped"`
	UnmappedCodes  map[string]int `json:"unmapped_codes,omitempty"`
	PerEthnic      []demandJSON   `json:"per_ethnic,omitempty"`
	TotalShortfall int            `json:"total_shortfall"`
}

type skippedJSON struct {
	Player string `json:"player"`
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type resultJSON struct {
	Assigned   int           `json:"assigned"`
	Preserved  int           `json:"preserved"`
	Skipped    []skippedJSON `json:"skipped,omitempty"`
	Unmapped   int           `json:"unmapped"`
	BackupPath string        `json:"backup_path,omitempty"`
	ConfigPath string        `json:"config_path"`
}

type assignDocJSON struct {
	DryRun   bool        `json:"dry_run"`
	Warnings []string    `json:"warnings,omitempty"`
	Plan     planJSON    `json:"plan"`
	Result   *resultJSON `json:"result,omitempty"`
}

func buildPlanJSON(in *pipeline.Inputs, plan *assign.Plan) planJSON {
	pj := planJSON{
		New:            len(plan.New),
		Preserved:      len(plan.Preserved),
		Unmapped:       len(plan.Unmapped),
		TotalShortfall: plan.TotalShortfall(),
	}
	if len(in.RTF.Unmapped) > 0 {
		pj.UnmappedCodes = in.RTF.Unmapped
	}
	for _, e := range ethnic.All {
		d, ok := plan.PerEthnic[e]
		if !ok {
			continue
		}
		pj.PerEthnic = append(pj.PerEthnic, demandJSON{
			Ethnic:    string(e),
			Needed:    d.Needed,
			Available: d.Available,
			Shortfall: d.Shortfall(plan.Options.AllowDuplicates),
		})
	}
	return pj
}

func printAssignJSON(out io.Writer, in *pipeline.Inputs, plan *assign.Plan, rr *pipeline.RunResult) error {
	doc := assignDocJSON{
		DryRun:   rr == nil,
		Warnings: in.Warnings,
		Plan:     buildPlanJSON(in, plan),
	}
	if rr != nil {
		rj := &resultJSON{
			Assigned:   len(rr.Result.Assigned),
			Preserved:  rr.Result.Preserved,
			Unmapped:   rr.Result.Unmapped,
			BackupPath: rr.BackupPath,
			ConfigPath: rr.ConfigPath,
		}
		for _, sk := range rr.Result.Skipped {
			rj.Skipped = append(rj.Skipped, skippedJSON{Player: sk.Player.Name, ID: sk.Player.ID, Reason: sk.Reason})
		}
		doc.Result = rj
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(out, string(data))

	if rr != nil && len(rr.Result.Skipped) > 0 {
		return newExitError(2, fmt.Errorf("%d player(s) were skipped", len(rr.Result.Skipped)))
	}
	return nil
}
