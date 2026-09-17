package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fmnewgenfaces/internal/core/ethnic"
	"fmnewgenfaces/internal/core/profile"
)

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

const rtfSample = "| UID       | Nat | 2nd Nat | Name | | | |\r\n" +
	"| 2000000001| ESP |         | Uno  | 1 | 9 | 0 |\r\n" +
	"| 2000000002| GER | RSA     | Dos  | 1 | 16| 3 |\r\n" +
	"| 2000000003| ZZZ |         | Tres | 1 | 5 | 1 |\r\n" +
	"| 2000000002| GER |         | Dup  | 1 | 16| 3 |\r\n" +
	"| 2000000004| FRA | broken\r\n"

func settings(t *testing.T) profile.Settings {
	t.Helper()
	root := t.TempDir()
	pack := filepath.Join(root, "faces")
	writePack(t, pack, 3)
	rtfPath := filepath.Join(pack, "newgen.rtf")
	if err := os.WriteFile(rtfPath, []byte(rtfSample), 0o644); err != nil {
		t.Fatal(err)
	}
	return profile.Settings{
		PackDir:         pack,
		ConfigXML:       filepath.Join(pack, "config.xml"),
		RTFPath:         rtfPath,
		FMVersion:       "2024",
		Preserve:        true,
		AllowDuplicates: false,
		Overrides:       map[string]string{"xxx": "nope"},
	}
}

func TestCheckReportsAllProblems(t *testing.T) {
	probs := Check(profile.Settings{FMVersion: "1999"})
	keys := map[string]bool{}
	for _, p := range probs {
		keys[p.Key] = true
	}
	for _, k := range []string{"pack", "config", "rtf", "version"} {
		if !keys[k] {
			t.Errorf("missing problem %q in %v", k, probs)
		}
	}
	if len(Check(settings(t))) != 0 {
		t.Errorf("valid settings reported problems: %v", Check(settings(t)))
	}
}

func TestLoadPlanRunAndAssignments(t *testing.T) {
	s := settings(t)
	in, err := Load(s)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(in.Warnings, "\n")
	for _, want := range []string{"override XXX", "malformed", "duplicate"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings missing %q:\n%s", want, joined)
		}
	}
	if in.RTF.Total() != 3 || len(in.RTF.UnmappedPlayers) != 1 {
		t.Fatalf("unexpected rtf result: players=%d unmapped=%d", len(in.RTF.Players), len(in.RTF.UnmappedPlayers))
	}

	plan := Plan(in)
	if len(plan.New) != 2 || len(plan.Unmapped) != 1 {
		t.Fatalf("plan new=%d unmapped=%d", len(plan.New), len(plan.Unmapped))
	}

	backups := t.TempDir()
	rr, err := Run(in, plan, backups, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rr.BackupPath != "" {
		t.Errorf("first save should not create a backup, got %q", rr.BackupPath)
	}
	if len(rr.Result.Assigned) != 2 {
		t.Fatalf("assigned %d", len(rr.Result.Assigned))
	}
	data, _ := os.ReadFile(s.ConfigXML)
	if !strings.Contains(string(data), `to="graphics/pictures/person/r-2000000001/portrait"`) {
		t.Errorf("config.xml missing r- portrait path:\n%s", data)
	}
	if strings.Contains(string(data), `\`) || strings.Contains(string(data), "../") {
		t.Errorf("config.xml has bad path style:\n%s", data)
	}

	// Second run with preserve keeps both and creates a backup.
	in2, err := Load(s)
	if err != nil {
		t.Fatal(err)
	}
	plan2 := Plan(in2)
	if len(plan2.Preserved) != 2 || len(plan2.New) != 0 {
		t.Fatalf("second plan preserved=%d new=%d", len(plan2.Preserved), len(plan2.New))
	}
	rr2, err := Run(in2, plan2, backups, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rr2.BackupPath == "" {
		t.Error("second save should create a backup")
	}

	as := Assignments(in2)
	if len(as) != 2 || as[0].Player.Name == "" {
		t.Fatalf("assignments: %+v", as)
	}
	if ImageFile(in2, as[0]) == "" {
		t.Errorf("image file not found for %+v", as[0])
	}

	a, err := Reroll(in2, as[0].Player, backups)
	if err != nil {
		t.Fatal(err)
	}
	if a.Image == as[0].Image {
		t.Errorf("reroll returned the same image %q with 3 available", a.Image)
	}
}

func TestOverridesTOMLRoundTrip(t *testing.T) {
	text := OverridesTOML(map[string]string{"ENG": "Caucasian", "AFG": "MESA"})
	if !strings.HasPrefix(text, "[mapping_override]\nAFG = \"MESA\"\nENG") {
		t.Fatalf("unexpected text:\n%s", text)
	}
	m, err := ParseOverridesTOML(text)
	if err != nil || m["ENG"] != "Caucasian" || m["AFG"] != "MESA" {
		t.Fatalf("round trip failed: %v %v", m, err)
	}
	m, err = ParseOverridesTOML("eng = 'caucasian'\nbad = 'Nope'")
	if err == nil || m["ENG"] != "Caucasian" {
		t.Fatalf("expected error for bad value and ENG kept: %v %v", m, err)
	}
	if _, err := ParseOverridesTOML("not toml ==="); err == nil {
		t.Error("expected TOML syntax error")
	}
}
