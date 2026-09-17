package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fmnewgenfaces/internal/brand"
	"fmnewgenfaces/internal/core/ethnic"
	"fmnewgenfaces/internal/core/profile"
)

// writePack builds a tiny face pack: every ethnic folder gets perGroup PNGs.
func writePack(t *testing.T, root string, perGroup int) {
	t.Helper()
	for _, e := range ethnic.All {
		dir := filepath.Join(root, string(e))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < perGroup; i++ {
			name := filepath.Join(dir, string(e[:2])+string(rune('a'+i))+".png")
			if err := os.WriteFile(name, []byte("png"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// rtfSample mirrors the format used by internal/core/pipeline/pipeline_test.go:
// two valid unique players, one with an unknown nation, one duplicate row and
// one malformed (too few columns) row.
const rtfSample = "| UID       | Nat | 2nd Nat | Name | | | |\r\n" +
	"| 2000000001| ESP |         | Uno  | 1 | 9 | 0 |\r\n" +
	"| 2000000002| GER | RSA     | Dos  | 1 | 16| 3 |\r\n" +
	"| 2000000003| ZZZ |         | Tres | 1 | 5 | 1 |\r\n" +
	"| 2000000002| GER |         | Dup  | 1 | 16| 3 |\r\n" +
	"| 2000000004| FRA | broken\r\n"

// setupEnv builds a face pack + RTF export under a fresh temp dir and returns
// the pack dir, RTF path and a (not yet existing) config.xml path.
func setupEnv(t *testing.T) (pack, rtfPath, configXML string) {
	t.Helper()
	pack = t.TempDir()
	writePack(t, pack, 2)
	rtfPath = filepath.Join(pack, "newgen.rtf")
	if err := os.WriteFile(rtfPath, []byte(rtfSample), 0o644); err != nil {
		t.Fatal(err)
	}
	configXML = filepath.Join(pack, "config.xml")
	return
}

// run executes args against a freshly constructed root command and returns
// its combined stdout+stderr output and the error Execute returned.
func run(t *testing.T, args []string) (string, error) {
	t.Helper()
	root := newRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// runOK executes args and fails the test if it returns an error.
func runOK(t *testing.T, args []string) string {
	t.Helper()
	out, err := run(t, args)
	if err != nil {
		t.Fatalf("command %v failed: %v\noutput:\n%s", args, err, out)
	}
	return out
}

func TestVersionCmd(t *testing.T) {
	out := runOK(t, []string{"version"})
	want := brand.AppName + " " + brand.Version
	if strings.TrimSpace(out) != want {
		t.Errorf("version output = %q, want %q", out, want)
	}
}

func TestProfilesCRUD(t *testing.T) {
	configDir := t.TempDir()

	out := runOK(t, []string{"--config-dir", configDir, "profiles", "create", "FM 2024", "--game", "/games/fm24"})
	if !strings.Contains(out, `"FM 2024"`) || !strings.Contains(out, `"fm-2024"`) {
		t.Fatalf("create output: %q", out)
	}

	out = runOK(t, []string{"--config-dir", configDir, "profiles"})
	if !strings.Contains(out, "fm-2024") || !strings.Contains(out, "FM 2024") {
		t.Fatalf("list output: %q", out)
	}

	out = runOK(t, []string{"--config-dir", configDir, "profiles", "show", "fm-2024"})
	var s profile.Settings
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		t.Fatalf("show output not valid JSON: %v\n%s", err, out)
	}
	if s.FMVersion == "" {
		t.Errorf("show output missing fm_version: %+v", s)
	}

	out = runOK(t, []string{"--config-dir", configDir, "profiles", "delete", "fm-2024"})
	if !strings.Contains(out, "fm-2024") {
		t.Fatalf("delete output: %q", out)
	}

	out = runOK(t, []string{"--config-dir", configDir, "profiles"})
	if !strings.Contains(out, "No profiles") {
		t.Fatalf("expected empty list after delete, got: %q", out)
	}
}

func TestCheckClean(t *testing.T) {
	pack, rtfPath, configXML := setupEnv(t)
	out := runOK(t, []string{"check", "--pack", pack, "--rtf", rtfPath, "--config", configXML, "--fm", "2024"})
	if !strings.Contains(out, "Setup looks good.") {
		t.Fatalf("check output: %q", out)
	}
	if !strings.Contains(out, "players: 2") {
		t.Errorf("expected 2 players in checklist, got: %q", out)
	}
}

func TestCheckMissingRTF(t *testing.T) {
	pack, _, configXML := setupEnv(t)
	missingRTF := filepath.Join(pack, "does-not-exist.rtf")

	out, err := run(t, []string{"check", "--pack", pack, "--rtf", missingRTF, "--config", configXML, "--fm", "2024"})
	if err == nil {
		t.Fatalf("expected error, output: %q", out)
	}
	var ee *exitError
	if !errors.As(err, &ee) || ee.code != 1 {
		t.Fatalf("expected exit code 1, got err=%v", err)
	}
	if !strings.Contains(out, "rtf") {
		t.Errorf("expected rtf problem in output: %q", out)
	}
}

func TestAssignDryRun(t *testing.T) {
	pack, rtfPath, configXML := setupEnv(t)
	out := runOK(t, []string{"assign", "--pack", pack, "--rtf", rtfPath, "--config", configXML, "--fm", "2024", "--dry-run"})

	if _, err := os.Stat(configXML); !os.IsNotExist(err) {
		t.Fatalf("config.xml should not have been written by a dry run")
	}
	if !strings.Contains(out, "Dry run") {
		t.Fatalf("assign --dry-run output: %q", out)
	}
	if !strings.Contains(out, "Totals: 2 new") {
		t.Errorf("expected 2 new players in plan, got: %q", out)
	}
}

func TestAssignYesAndRestore(t *testing.T) {
	pack, rtfPath, configXML := setupEnv(t)
	configDir := t.TempDir()

	out := runOK(t, []string{"--config-dir", configDir, "assign", "--pack", pack, "--rtf", rtfPath, "--config", configXML, "--fm", "2024", "--yes"})
	if !strings.Contains(out, "Assigned 2") {
		t.Fatalf("first assign output: %q", out)
	}
	data, err := os.ReadFile(configXML)
	if err != nil {
		t.Fatalf("config.xml not written: %v", err)
	}
	if !strings.Contains(string(data), `to="graphics/pictures/person/r-2000000001/portrait"`) {
		t.Errorf("config.xml missing r- portrait for player 1:\n%s", data)
	}
	if !strings.Contains(string(data), `to="graphics/pictures/person/r-2000000002/portrait"`) {
		t.Errorf("config.xml missing r- portrait for player 2:\n%s", data)
	}

	// Second run: both players are already mapped and preserved, and the
	// previous config.xml is backed up.
	out2 := runOK(t, []string{"--config-dir", configDir, "assign", "--pack", pack, "--rtf", rtfPath, "--config", configXML, "--fm", "2024", "--yes"})
	if !strings.Contains(out2, "Backup:") {
		t.Fatalf("second assign should report a backup, output: %q", out2)
	}
	if !strings.Contains(out2, "Assigned 0, preserved 2") {
		t.Errorf("expected second run to preserve both players, got: %q", out2)
	}

	listOut := runOK(t, []string{"--config-dir", configDir, "restore", "--list", "--config", configXML})
	if !strings.Contains(listOut, "config-") {
		t.Fatalf("restore --list should show a backup, output: %q", listOut)
	}

	latestOut := runOK(t, []string{"--config-dir", configDir, "restore", "--latest", "--config", configXML})
	if !strings.Contains(latestOut, "restored") {
		t.Fatalf("restore --latest output: %q", latestOut)
	}
	if _, err := os.Stat(configXML + ".before-restore"); err != nil {
		t.Errorf("restore --latest should back up the current file first: %v", err)
	}
}

func TestAssignJSON(t *testing.T) {
	pack, rtfPath, configXML := setupEnv(t)
	out := runOK(t, []string{"assign", "--pack", pack, "--rtf", rtfPath, "--config", configXML, "--fm", "2024", "--dry-run", "--json"})

	var doc struct {
		DryRun bool `json:"dry_run"`
		Plan   struct {
			New       int `json:"new"`
			Preserved int `json:"preserved"`
			Unmapped  int `json:"unmapped"`
		} `json:"plan"`
		Result *struct{} `json:"result"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("assign --json output not valid JSON: %v\n%s", err, out)
	}
	if !doc.DryRun {
		t.Errorf("expected dry_run=true, got %+v", doc)
	}
	if doc.Plan.New != 2 || doc.Plan.Unmapped != 1 {
		t.Errorf("unexpected plan in JSON: %+v", doc.Plan)
	}
	if doc.Result != nil {
		t.Errorf("expected no result for a dry run, got %+v", doc.Result)
	}
}

func TestRestoreRefusesWithoutFlags(t *testing.T) {
	_, _, configXML := setupEnv(t)
	_, err := run(t, []string{"restore", "--config", configXML})
	if err == nil {
		t.Fatal("expected an error when none of --list/--to/--latest is given")
	}
}

func TestFlagsOverlayProfile(t *testing.T) {
	pack, rtfPath, configXML := setupEnv(t)
	configDir := t.TempDir()
	runOK(t, []string{"--config-dir", configDir, "profiles", "create", "Overlay"})
	// The profile has no paths yet, so check alone must fail...
	if _, err := run(t, []string{"--config-dir", configDir, "check", "--profile", "overlay"}); err == nil {
		t.Fatal("check on an empty profile should report problems")
	}
	// ...but individual flags fill in exactly the missing fields.
	out := runOK(t, []string{"--config-dir", configDir, "check", "--profile", "overlay", "--pack", pack, "--rtf", rtfPath, "--config", configXML, "--fm", "2024"})
	if strings.Contains(out, "problem") {
		t.Fatalf("overlayed check should be clean, got: %q", out)
	}
}
