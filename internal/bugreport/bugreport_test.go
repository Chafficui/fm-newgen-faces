package bugreport

import (
	"net/url"
	"os"
	"strings"
	"testing"

	"fmnewgenfaces/internal/core/profile"
)

func TestRedact(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory available in this environment")
	}

	s := "log at " + home + "/logs/app.log and also " + home + "\\logs\\app.log"
	got := Redact(s)
	if strings.Contains(got, home) {
		t.Errorf("Redact() left the home directory in: %q", got)
	}
	if !strings.Contains(got, "~/logs/app.log") {
		t.Errorf("Redact() = %q, expected ~/logs/app.log", got)
	}

	got2 := Redact("path is %USERPROFILE%\\Documents")
	if strings.Contains(got2, "%USERPROFILE%") {
		t.Errorf("Redact() left %%USERPROFILE%% in: %q", got2)
	}
	if !strings.Contains(got2, "~") {
		t.Errorf("Redact() = %q, expected ~", got2)
	}
}

func TestBuildIncludesRedactedSections(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory available in this environment")
	}

	p := &profile.Profile{
		Name:     "My Save",
		GamePath: home + "/Documents/Sports Interactive/Football Manager 2024",
		Settings: profile.Settings{
			PackDir:         home + "/packs",
			ConfigXML:       home + "/config.xml",
			RTFPath:         home + "/newgen.rtf",
			FMVersion:       "2024",
			Preserve:        true,
			AllowDuplicates: false,
			Overrides:       map[string]string{"FRA": "european", "BRA": "south_american"},
		},
	}

	info := Info{
		Version:   "1.2.3",
		Profiles:  []*profile.Profile{p},
		Current:   p,
		LogTail:   "line one\n" + home + "/log line two",
		Checklist: "- [x] step one at " + home,
	}

	out := Build(info)

	if strings.Contains(out, home) {
		t.Errorf("Build() output still contains the home directory:\n%s", out)
	}
	for _, want := range []string{
		"1.2.3",
		"My Save",
		"### Profiles",
		"### Log tail",
		"### Checklist",
		"### Description / Steps to reproduce",
		"BRA -> south_american",
		"FRA -> european",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Build() output missing %q:\n%s", want, out)
		}
	}
}

func TestBuildNoProfiles(t *testing.T) {
	out := Build(Info{Version: "dev"})
	if !strings.Contains(out, "No profiles found") {
		t.Errorf("Build() with no profiles should say so:\n%s", out)
	}
	if !strings.Contains(out, "No log available") {
		t.Errorf("Build() with no log tail should say so:\n%s", out)
	}
}

func TestIssueURL(t *testing.T) {
	got := IssueURL("Crash on save", "steps here")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(u.Path, "/issues/new") {
		t.Errorf("IssueURL path = %q", u.Path)
	}
	q := u.Query()
	if q.Get("title") != "Crash on save" {
		t.Errorf("title = %q", q.Get("title"))
	}
	if q.Get("body") != "steps here" {
		t.Errorf("body = %q", q.Get("body"))
	}
}

func TestIssueURLTruncatesLongBody(t *testing.T) {
	body := strings.Repeat("a", 10000)
	got := IssueURL("t", body)
	if len(got) > 40000 {
		t.Fatalf("IssueURL() result unexpectedly huge: %d bytes", len(got))
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	decodedBody := u.Query().Get("body")
	if len(decodedBody) > 6000 {
		t.Errorf("decoded body is %d bytes, want <= 6000", len(decodedBody))
	}
	if !strings.Contains(decodedBody, "truncated") {
		t.Errorf("expected a truncation note in body, got %q", decodedBody)
	}
}
