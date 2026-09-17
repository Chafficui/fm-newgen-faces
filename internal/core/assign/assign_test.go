package assign

import (
	"os"
	"path/filepath"
	"testing"

	"fmnewgenfaces/internal/core/ethnic"
	"fmnewgenfaces/internal/core/facepack"
	"fmnewgenfaces/internal/core/fmconfig"
	"fmnewgenfaces/internal/core/fmversion"
	"fmnewgenfaces/internal/core/rtf"
)

var fm24 = fmversion.Version{Year: "2024", Label: "FM24", IDPrefix: "r-"}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// buildPack writes real image files under root/<ethnic>/name.png for each
// ethnic->names entry, then scans it with facepack.Scan.
func buildPack(t *testing.T, images map[ethnic.Ethnic][]string) *facepack.Pack {
	t.Helper()
	root := t.TempDir()
	for _, e := range ethnic.All {
		dir := filepath.Join(root, string(e))
		mustMkdir(t, dir)
		for _, name := range images[e] {
			mustWrite(t, filepath.Join(dir, name+".png"), "x")
		}
	}
	pack, err := facepack.Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	return pack
}

func player(id string, e ethnic.Ethnic) rtf.Player {
	return rtf.Player{ID: id, Name: "P" + id, Nation: "GER", Ethnic: e}
}

func TestBuild_UnmappedFromResult(t *testing.T) {
	res := &rtf.Result{
		UnmappedPlayers: []rtf.Player{
			{ID: "1", Name: "A", Nation: "ZZZ"}, // Ethnic == ""
		},
	}
	pack := buildPack(t, nil)
	cfg := fmconfig.New(filepath.Join(t.TempDir(), "config.xml"), fm24)

	plan := Build(res, cfg, pack, Options{})
	if len(plan.Unmapped) != 1 || plan.Unmapped[0].ID != "1" {
		t.Fatalf("Unmapped = %+v", plan.Unmapped)
	}
	if len(plan.New) != 0 || len(plan.Preserved) != 0 {
		t.Errorf("unmapped players must never be New or Preserved")
	}
}

func TestBuild_PreserveSplitsNewVsPreserved(t *testing.T) {
	pack := buildPack(t, map[ethnic.Ethnic][]string{
		ethnic.African: {"a1", "a2", "a3"},
	})
	cfgPath := filepath.Join(t.TempDir(), "config.xml")
	cfg := fmconfig.New(cfgPath, fm24)
	cfg.Set("existing", "African/a1")

	res := &rtf.Result{
		Players: []rtf.Player{
			player("existing", ethnic.African),
			player("newone", ethnic.African),
		},
	}

	plan := Build(res, cfg, pack, Options{Preserve: true})
	if len(plan.Preserved) != 1 || plan.Preserved[0].ID != "existing" {
		t.Errorf("Preserved = %+v", plan.Preserved)
	}
	if len(plan.New) != 1 || plan.New[0].ID != "newone" {
		t.Errorf("New = %+v", plan.New)
	}

	demand := plan.PerEthnic[ethnic.African]
	if demand == nil {
		t.Fatalf("expected Demand for African")
	}
	if demand.Needed != 1 || demand.Preserved != 1 {
		t.Errorf("Demand = %+v, want Needed=1 Preserved=1", demand)
	}
}

func TestBuild_WithoutPreserveEverythingIsNew(t *testing.T) {
	pack := buildPack(t, map[ethnic.Ethnic][]string{ethnic.African: {"a1"}})
	cfg := fmconfig.New(filepath.Join(t.TempDir(), "config.xml"), fm24)
	cfg.Set("existing", "African/a1")

	res := &rtf.Result{Players: []rtf.Player{player("existing", ethnic.African)}}
	plan := Build(res, cfg, pack, Options{Preserve: false})

	if len(plan.Preserved) != 0 {
		t.Errorf("Preserved = %+v, want empty when Preserve=false", plan.Preserved)
	}
	if len(plan.New) != 1 {
		t.Errorf("New = %+v, want 1 entry", plan.New)
	}
}

func TestBuild_AvailableExcludesAlreadyMappedImages(t *testing.T) {
	pack := buildPack(t, map[ethnic.Ethnic][]string{
		ethnic.African: {"a1", "a2", "a3"},
	})
	cfg := fmconfig.New(filepath.Join(t.TempDir(), "config.xml"), fm24)
	cfg.Set("other", "African/a1") // already mapped, recognised by ParseRef

	res := &rtf.Result{Players: []rtf.Player{player("p1", ethnic.African)}}

	plan := Build(res, cfg, pack, Options{AllowDuplicates: false})
	demand := plan.PerEthnic[ethnic.African]
	if demand == nil {
		t.Fatalf("expected Demand for African")
	}
	if demand.Available != 2 {
		t.Errorf("Available = %d, want 2 (3 images - 1 excluded)", demand.Available)
	}
	if plan.Excluded != 1 {
		t.Errorf("Excluded = %d, want 1", plan.Excluded)
	}

	// With AllowDuplicates, nothing is excluded.
	plan2 := Build(res, cfg, pack, Options{AllowDuplicates: true})
	demand2 := plan2.PerEthnic[ethnic.African]
	if demand2.Available != 3 {
		t.Errorf("Available (AllowDuplicates) = %d, want 3", demand2.Available)
	}
	if plan2.Excluded != 0 {
		t.Errorf("Excluded (AllowDuplicates) = %d, want 0", plan2.Excluded)
	}
}

func TestBuild_DemandOnlyForGroupsWithPlayers(t *testing.T) {
	pack := buildPack(t, map[ethnic.Ethnic][]string{
		ethnic.African: {"a1"},
		ethnic.Asian:   {"b1"},
	})
	cfg := fmconfig.New(filepath.Join(t.TempDir(), "config.xml"), fm24)
	res := &rtf.Result{Players: []rtf.Player{player("p1", ethnic.African)}}

	plan := Build(res, cfg, pack, Options{})
	if _, ok := plan.PerEthnic[ethnic.African]; !ok {
		t.Errorf("expected Demand entry for African")
	}
	if _, ok := plan.PerEthnic[ethnic.Asian]; ok {
		t.Errorf("did not expect Demand entry for Asian (no players)")
	}
}

func TestShortfallAndTotalShortfall(t *testing.T) {
	pack := buildPack(t, map[ethnic.Ethnic][]string{ethnic.African: {"a1"}})
	cfg := fmconfig.New(filepath.Join(t.TempDir(), "config.xml"), fm24)
	res := &rtf.Result{Players: []rtf.Player{
		player("p1", ethnic.African),
		player("p2", ethnic.African),
		player("p3", ethnic.African),
	}}

	plan := Build(res, cfg, pack, Options{AllowDuplicates: false})
	demand := plan.PerEthnic[ethnic.African]
	if demand.Shortfall(false) != 2 {
		t.Errorf("Shortfall = %d, want 2 (3 needed, 1 available)", demand.Shortfall(false))
	}
	if demand.Shortfall(true) != 0 {
		t.Errorf("Shortfall with allowDuplicates = %d, want 0", demand.Shortfall(true))
	}
	if plan.TotalShortfall() != 2 {
		t.Errorf("TotalShortfall = %d, want 2", plan.TotalShortfall())
	}
}

func TestApply_AssignsAndSetsFrom(t *testing.T) {
	pack := buildPack(t, map[ethnic.Ethnic][]string{
		ethnic.African: {"a1", "a2"},
	})
	configPath := filepath.Join(pack.Root, "config.xml") // config in the pack root
	cfg := fmconfig.New(configPath, fm24)

	res := &rtf.Result{Players: []rtf.Player{
		player("p1", ethnic.African),
		player("p2", ethnic.African),
	}}
	plan := Build(res, cfg, pack, Options{AllowDuplicates: true, Seed: 42})

	result, err := Apply(plan, cfg, pack, nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(result.Assigned) != 2 {
		t.Fatalf("Assigned = %+v, want 2", result.Assigned)
	}
	for _, a := range result.Assigned {
		from, ok := cfg.Get(a.Player.ID)
		if !ok || from != a.From {
			t.Errorf("cfg not updated for %s: Get=%q,%v want %q", a.Player.ID, from, ok, a.From)
		}
		wantFrom := "African/" + a.Image
		if a.From != wantFrom {
			t.Errorf("From = %q, want %q", a.From, wantFrom)
		}
	}
}

func TestApply_PreservedCountFromPlan(t *testing.T) {
	pack := buildPack(t, map[ethnic.Ethnic][]string{ethnic.African: {"a1", "a2"}})
	configPath := filepath.Join(t.TempDir(), "config.xml")
	cfg := fmconfig.New(configPath, fm24)
	cfg.Set("existing", "African/a1")

	res := &rtf.Result{Players: []rtf.Player{
		player("existing", ethnic.African),
		player("newone", ethnic.African),
	}}
	plan := Build(res, cfg, pack, Options{Preserve: true})

	result, err := Apply(plan, cfg, pack, nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.Preserved != 1 {
		t.Errorf("Preserved = %d, want 1", result.Preserved)
	}
	// Preserved player's mapping must be untouched.
	from, _ := cfg.Get("existing")
	if from != "African/a1" {
		t.Errorf("preserved mapping changed: %q", from)
	}
}

func TestApply_SkipsWhenPoolExhausted(t *testing.T) {
	pack := buildPack(t, map[ethnic.Ethnic][]string{ethnic.African: {"a1"}})
	configPath := filepath.Join(t.TempDir(), "config.xml")
	cfg := fmconfig.New(configPath, fm24)

	res := &rtf.Result{Players: []rtf.Player{
		player("p1", ethnic.African),
		player("p2", ethnic.African), // no image left for this one
	}}
	plan := Build(res, cfg, pack, Options{AllowDuplicates: false})

	result, err := Apply(plan, cfg, pack, nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(result.Assigned) != 1 {
		t.Fatalf("Assigned = %+v, want 1", result.Assigned)
	}
	if len(result.Skipped) != 1 {
		t.Fatalf("Skipped = %+v, want 1", result.Skipped)
	}
	if result.Skipped[0].Player.ID != "p2" || result.Skipped[0].Reason == "" {
		t.Errorf("Skipped entry wrong: %+v", result.Skipped[0])
	}
}

func TestApply_NoDuplicatesExcludesAlreadyMapped(t *testing.T) {
	pack := buildPack(t, map[ethnic.Ethnic][]string{ethnic.African: {"a1", "a2"}})
	configPath := filepath.Join(t.TempDir(), "config.xml")
	cfg := fmconfig.New(configPath, fm24)
	cfg.Set("other", "African/a1")

	res := &rtf.Result{Players: []rtf.Player{player("p1", ethnic.African)}}
	plan := Build(res, cfg, pack, Options{AllowDuplicates: false})

	result, err := Apply(plan, cfg, pack, nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(result.Assigned) != 1 {
		t.Fatalf("Assigned = %+v", result.Assigned)
	}
	if result.Assigned[0].Image != "a2" {
		t.Errorf("Image = %q, want a2 (a1 already mapped)", result.Assigned[0].Image)
	}
}

func TestApply_ProgressCalledPeriodicallyAndAtEnd(t *testing.T) {
	names := make([]string, 250)
	for i := range names {
		names[i] = "img" + string(rune('a'+i%26)) + string(rune('0'+i/26))
	}
	pack := buildPack(t, map[ethnic.Ethnic][]string{ethnic.African: names})
	configPath := filepath.Join(t.TempDir(), "config.xml")
	cfg := fmconfig.New(configPath, fm24)

	var players []rtf.Player
	for i := 0; i < 250; i++ {
		players = append(players, player(rtfID(i), ethnic.African))
	}
	res := &rtf.Result{Players: players}
	plan := Build(res, cfg, pack, Options{AllowDuplicates: true, Seed: 7})

	var calls []int
	progress := func(done, total int) {
		if total != 250 {
			t.Errorf("progress total = %d, want 250", total)
		}
		calls = append(calls, done)
	}

	if _, err := Apply(plan, cfg, pack, progress); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(calls) == 0 {
		t.Fatalf("progress was never called")
	}
	if calls[len(calls)-1] != 250 {
		t.Errorf("last progress call done=%d, want 250 (called at the end)", calls[len(calls)-1])
	}
	// Called at least every 100.
	prev := 0
	for _, c := range calls {
		if c-prev > 100 {
			t.Errorf("gap between progress calls too large: %d -> %d", prev, c)
		}
		prev = c
	}
}

func TestApply_ProgressCalledEvenWhenNoPlayers(t *testing.T) {
	pack := buildPack(t, map[ethnic.Ethnic][]string{ethnic.African: {"a1"}})
	configPath := filepath.Join(t.TempDir(), "config.xml")
	cfg := fmconfig.New(configPath, fm24)
	res := &rtf.Result{}
	plan := Build(res, cfg, pack, Options{})

	called := false
	progress := func(done, total int) { called = true }
	if _, err := Apply(plan, cfg, pack, progress); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !called {
		t.Errorf("progress must be called at least once even with zero players")
	}
}

func TestApply_NeverWritesToDisk(t *testing.T) {
	pack := buildPack(t, map[ethnic.Ethnic][]string{ethnic.African: {"a1"}})
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.xml")
	cfg := fmconfig.New(configPath, fm24)

	res := &rtf.Result{Players: []rtf.Player{player("p1", ethnic.African)}}
	plan := Build(res, cfg, pack, Options{})

	if _, err := Apply(plan, cfg, pack, nil); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, err := os.Stat(configPath); err == nil {
		t.Errorf("Apply must never write to disk, but %s exists", configPath)
	}
}

func rtfID(i int) string {
	// Produce distinct 7+ digit-looking IDs; content doesn't matter for tests.
	return "p" + itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}

func TestReroll_DrawsDifferentImageWhenPossible(t *testing.T) {
	pack := buildPack(t, map[ethnic.Ethnic][]string{ethnic.African: {"a1", "a2", "a3"}})
	configPath := filepath.Join(t.TempDir(), "config.xml")
	cfg := fmconfig.New(configPath, fm24)
	cfg.Set("p1", "African/a1")

	p := player("p1", ethnic.African)

	for i := 0; i < 20; i++ {
		asn, err := Reroll(cfg, pack, p, Options{AllowDuplicates: true})
		if err != nil {
			t.Fatalf("Reroll: %v", err)
		}
		if asn.Image == "a1" {
			t.Fatalf("Reroll returned the same image (a1) while others are available")
		}
		if got, _ := cfg.Get("p1"); got != asn.From {
			t.Errorf("cfg not updated: %q != %q", got, asn.From)
		}
		// Reset for next iteration.
		cfg.Set("p1", "African/a1")
	}
}

func TestReroll_KeepsSameImageWhenOnlyOneAvailable(t *testing.T) {
	pack := buildPack(t, map[ethnic.Ethnic][]string{ethnic.African: {"a1"}})
	configPath := filepath.Join(t.TempDir(), "config.xml")
	cfg := fmconfig.New(configPath, fm24)
	cfg.Set("p1", "African/a1")

	p := player("p1", ethnic.African)
	asn, err := Reroll(cfg, pack, p, Options{AllowDuplicates: true})
	if err != nil {
		t.Fatalf("Reroll: %v", err)
	}
	if asn.Image != "a1" {
		t.Errorf("Image = %q, want a1 (only option)", asn.Image)
	}
}

func TestReroll_NoDuplicatesAvoidsOtherUIDsImages(t *testing.T) {
	pack := buildPack(t, map[ethnic.Ethnic][]string{ethnic.African: {"a1", "a2", "a3"}})
	configPath := filepath.Join(t.TempDir(), "config.xml")
	cfg := fmconfig.New(configPath, fm24)
	cfg.Set("p1", "African/a1")
	cfg.Set("other", "African/a2")

	p := player("p1", ethnic.African)

	for i := 0; i < 20; i++ {
		asn, err := Reroll(cfg, pack, p, Options{AllowDuplicates: false})
		if err != nil {
			t.Fatalf("Reroll: %v", err)
		}
		if asn.Image == "a2" {
			t.Fatalf("Reroll picked an image mapped to another uid: %q", asn.Image)
		}
		cfg.Set("p1", "African/a1") // reset for next attempt
	}
}

func TestReroll_ErrorWhenExhausted(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.xml")
	cfg := fmconfig.New(configPath, fm24)
	cfg.Set("p1", "African/a1")
	// Empty pack: no images at all for African.
	emptyPack := buildPack(t, map[ethnic.Ethnic][]string{})
	p := player("p1", ethnic.African)

	_, err := Reroll(cfg, emptyPack, p, Options{AllowDuplicates: true})
	if err == nil {
		t.Fatalf("expected error when pool is exhausted")
	}
}
