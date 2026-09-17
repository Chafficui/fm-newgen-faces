package rtf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fmnewgenfaces/internal/core/ethnic"
)

func mustResolver(t *testing.T, overrides map[string]string) *ethnic.Resolver {
	t.Helper()
	r, err := ethnic.NewResolver(overrides)
	if err != nil {
		t.Fatalf("NewResolver(%v) unexpected error: %v", overrides, err)
	}
	return r
}

func writeCRLF(t *testing.T, dir, name string, lines []string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := strings.Join(lines, "\r\n") + "\r\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

const sep = "| ---------------------------------------------------------------------------------------------------| "
const header = "| UID       | Nat       | 2nd Nat   | Name                       |           |           |           | "

func TestParseExampleRTF(t *testing.T) {
	res := mustResolver(t, nil)
	result, err := Parse("../../../example/newgen.rtf", res)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if len(result.Malformed) != 0 {
		t.Errorf("Malformed = %v, want none", result.Malformed)
	}
	if len(result.UnmappedPlayers) != 0 {
		t.Errorf("UnmappedPlayers = %v, want none", result.UnmappedPlayers)
	}
	if result.Duplicates != 0 {
		t.Errorf("Duplicates = %d, want 0", result.Duplicates)
	}
	if len(result.Players) != 9 {
		t.Fatalf("len(Players) = %d, want 9", len(result.Players))
	}

	var found *Player
	for i := range result.Players {
		if result.Players[i].ID == "2000133469" {
			found = &result.Players[i]
			break
		}
	}
	if found == nil {
		t.Fatal("player 2000133469 not found")
	}
	if found.Nation != "GER" || found.Nation2 != "RSA" || found.EthnicValue != 3 {
		t.Fatalf("player 2000133469 = %+v, want Nation=GER Nation2=RSA EthnicValue=3", *found)
	}
	if found.Ethnic != ethnic.African {
		t.Errorf("player 2000133469 Ethnic = %q, want African", found.Ethnic)
	}
	if found.Name != "Tebogo Maluleke" {
		t.Errorf("player 2000133469 Name = %q, want Tebogo Maluleke", found.Name)
	}
}

func TestParseSyntheticFile(t *testing.T) {
	dir := t.TempDir()
	lines := []string{
		`{\rtf1\ansi\ansicpg1252\deff0{\fonttbl}`,
		`{\colortbl;}`,
		`\viewkind4\uc1\pard\f0\fs20`,
		header,
		sep,
		"| 1234567| GER       |           | Otto Mueller               | 1         | 12        | 0         | ",
		sep,
		"| 1234568| ZZZ       |           | Unknown Nation Guy         | 1         | 12        | 0         | ",
		sep,
		"| 1234567| GER       |           | Otto Mueller Duplicate     | 1         | 12        | 0         | ",
		sep,
		"| 1234569| GER       |           | Bad Ethnic Col             | 1         | 12        | abc       | ",
		sep,
		"| 1234571| GER       | Missing columns",
		sep,
		"| 2000199999| GER       |           | Player1234567890           | 1         | 12        | 0         | ",
		sep,
	}
	path := writeCRLF(t, dir, "synthetic.rtf", lines)

	res := mustResolver(t, nil)
	result, err := Parse(path, res)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if len(result.Players) != 2 {
		t.Fatalf("len(Players) = %d, want 2: %+v", len(result.Players), result.Players)
	}
	if result.Players[0].ID != "1234567" || result.Players[0].Ethnic != ethnic.CentralEuropean {
		t.Errorf("Players[0] = %+v, want ID=1234567 Ethnic=CentralEuropean", result.Players[0])
	}
	if result.Players[1].ID != "2000199999" || result.Players[1].Name != "Player1234567890" {
		t.Errorf("Players[1] = %+v, want ID=2000199999 Name=Player1234567890", result.Players[1])
	}
	if result.Players[1].Ethnic != ethnic.CentralEuropean {
		t.Errorf("Players[1].Ethnic = %q, want CentralEuropean", result.Players[1].Ethnic)
	}

	if len(result.UnmappedPlayers) != 1 {
		t.Fatalf("len(UnmappedPlayers) = %d, want 1: %+v", len(result.UnmappedPlayers), result.UnmappedPlayers)
	}
	up := result.UnmappedPlayers[0]
	if up.ID != "1234568" || up.Nation != "ZZZ" || up.Ethnic != "" {
		t.Errorf("UnmappedPlayers[0] = %+v, want ID=1234568 Nation=ZZZ Ethnic=\"\"", up)
	}
	if result.Unmapped["ZZZ"] != 1 {
		t.Errorf("Unmapped[ZZZ] = %d, want 1", result.Unmapped["ZZZ"])
	}

	if result.Duplicates != 1 {
		t.Errorf("Duplicates = %d, want 1", result.Duplicates)
	}

	if len(result.Malformed) != 2 {
		t.Fatalf("len(Malformed) = %d, want 2: %+v", len(result.Malformed), result.Malformed)
	}
	foundBadEthnic, foundTooFewCols := false, false
	for _, issue := range result.Malformed {
		if strings.Contains(issue.Reason, "non-integer") {
			foundBadEthnic = true
		}
		if strings.Contains(issue.Reason, "fewer than 8 columns") {
			foundTooFewCols = true
		}
		if issue.Line <= 0 {
			t.Errorf("Issue.Line = %d, want > 0", issue.Line)
		}
	}
	if !foundBadEthnic {
		t.Error("expected a malformed issue for the non-integer ethnic column")
	}
	if !foundTooFewCols {
		t.Error("expected a malformed issue for the too-few-columns row")
	}

	if result.Total() != 3 {
		t.Errorf("Total() = %d, want 3", result.Total())
	}
}

func TestParseErrNoPlayersOnEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.rtf")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res := mustResolver(t, nil)
	_, err := Parse(path, res)
	if err != ErrNoPlayers {
		t.Fatalf("Parse(empty file) error = %v, want ErrNoPlayers", err)
	}
}

func TestParseErrNoPlayersWhenOnlyStructuralRows(t *testing.T) {
	dir := t.TempDir()
	lines := []string{
		`{\rtf1\ansi\ansicpg1252\deff0{\fonttbl}`,
		header,
		sep,
		sep,
	}
	path := writeCRLF(t, dir, "no-players.rtf", lines)

	res := mustResolver(t, nil)
	_, err := Parse(path, res)
	if err != ErrNoPlayers {
		t.Fatalf("Parse(structural-only file) error = %v, want ErrNoPlayers", err)
	}
}

func TestParseMissingFile(t *testing.T) {
	res := mustResolver(t, nil)
	_, err := Parse("/no/such/path/does-not-exist.rtf", res)
	if err == nil {
		t.Fatal("expected an I/O error for a missing file")
	}
	if err == ErrNoPlayers {
		t.Fatal("missing file should not report ErrNoPlayers")
	}
}

func TestResultUnmappedCodesSortedByCountDescThenCodeAsc(t *testing.T) {
	result := &Result{
		Unmapped: map[string]int{
			"BBB": 2,
			"AAA": 2,
			"CCC": 5,
			"DDD": 1,
		},
	}
	got := result.UnmappedCodes()
	want := []string{"CCC", "AAA", "BBB", "DDD"}
	if len(got) != len(want) {
		t.Fatalf("UnmappedCodes() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("UnmappedCodes() = %v, want %v", got, want)
		}
	}
}

func TestResultResolveMovesPlayersWhenTheyNowResolve(t *testing.T) {
	baseRes := mustResolver(t, nil)

	result := &Result{
		Players: []Player{
			{ID: "1", Nation: "GER", EthnicValue: 0, Ethnic: ethnic.CentralEuropean},
		},
		UnmappedPlayers: []Player{
			{ID: "2", Nation: "XYZ", Nation2: "", EthnicValue: 3},
			{ID: "3", Nation: "ABC", Nation2: "", EthnicValue: 3},
		},
		Unmapped: map[string]int{"XYZ": 1, "ABC": 1},
	}

	_ = baseRes // baseline resolver not used further; kept for clarity of setup

	// Now the user adds an override that makes XYZ resolve, but ABC remains unknown.
	res2 := mustResolver(t, map[string]string{"XYZ": "African"})

	result.Resolve(res2)

	if len(result.Players) != 2 {
		t.Fatalf("len(Players) = %d, want 2: %+v", len(result.Players), result.Players)
	}
	if result.Players[0].ID != "1" {
		t.Errorf("Players[0].ID = %q, want 1 (stable order)", result.Players[0].ID)
	}
	if result.Players[1].ID != "2" || result.Players[1].Ethnic != ethnic.African {
		t.Errorf("Players[1] = %+v, want ID=2 Ethnic=African", result.Players[1])
	}

	if len(result.UnmappedPlayers) != 1 || result.UnmappedPlayers[0].ID != "3" {
		t.Fatalf("UnmappedPlayers = %+v, want just ID=3", result.UnmappedPlayers)
	}
	if result.Unmapped["ABC"] != 1 {
		t.Errorf("Unmapped[ABC] = %d, want 1", result.Unmapped["ABC"])
	}
	if _, ok := result.Unmapped["XYZ"]; ok {
		t.Errorf("Unmapped should no longer contain XYZ: %v", result.Unmapped)
	}
}
