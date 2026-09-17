package i18n

import (
	"testing"
)

func resetState(t *testing.T) {
	t.Helper()
	mu.Lock()
	current = "en"
	mu.Unlock()
}

func TestLangFromFilename(t *testing.T) {
	cases := map[string]string{
		"en.json":         "en",
		"de.json":         "de",
		"en.widgets.json": "en",
		"de.widgets.json": "de",
		"en.gui.json":     "en",
		"noextension":     "noextension",
	}
	for in, want := range cases {
		if got := langFromFilename(in); got != want {
			t.Errorf("langFromFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAvailableListsEnglishFirst(t *testing.T) {
	langs := Available()
	if len(langs) == 0 {
		t.Fatal("Available() returned no languages")
	}
	if langs[0].Code != "en" {
		t.Fatalf("Available()[0].Code = %q, want en", langs[0].Code)
	}
	if langs[0].Name != "English" {
		t.Errorf("Available()[0].Name = %q, want English", langs[0].Name)
	}

	found := map[string]bool{}
	for _, l := range langs {
		found[l.Code] = true
	}
	if !found["de"] {
		t.Errorf("Available() missing de: %+v", langs)
	}
}

func TestInitExplicitCode(t *testing.T) {
	defer resetState(t)
	Init("de")
	if Current() != "de" {
		t.Errorf("Current() = %q, want de", Current())
	}
}

func TestInitUnknownFallsBackToEnglish(t *testing.T) {
	defer resetState(t)
	Init("xx")
	if Current() != "en" {
		t.Errorf("Current() = %q, want en for an unknown language", Current())
	}
}

func TestInitFromEnv(t *testing.T) {
	defer resetState(t)
	t.Setenv("LANG", "de_DE.UTF-8")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	Init("")
	if Current() != "de" {
		t.Errorf("Current() = %q, want de (from $LANG)", Current())
	}
}

func TestInitEnvFallsBackToEnglishWhenUnset(t *testing.T) {
	defer resetState(t)
	t.Setenv("LANG", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	Init("")
	if Current() != "en" {
		t.Errorf("Current() = %q, want en when no locale env vars are set", Current())
	}
}

func TestT(t *testing.T) {
	defer resetState(t)
	Init("en")
	if got := T("common.ok"); got != "OK" {
		t.Errorf("T(common.ok) = %q, want OK", got)
	}
	if got := T("greeting", "World"); got != "Hello, World!" {
		t.Errorf("T(greeting, World) = %q, want %q", got, "Hello, World!")
	}
	if got := T("does.not.exist"); got != "does.not.exist" {
		t.Errorf("T(missing key) = %q, want the key itself", got)
	}
}

func TestTFallsBackToEnglish(t *testing.T) {
	defer resetState(t)
	Init("de")
	// A key that only exists in en (simulated: unlikely real key).
	if got := T("common.ok"); got != "OK" {
		t.Errorf("T(common.ok) under de = %q, want OK (both locales share it)", got)
	}
	if got := T("totally.unknown.key"); got != "totally.unknown.key" {
		t.Errorf("T(unknown) = %q, want the key itself", got)
	}
}

func TestN(t *testing.T) {
	defer resetState(t)
	Init("en")
	if got := N("profile.count", 1); got != "1 profile" {
		t.Errorf("N(profile.count, 1) = %q, want %q", got, "1 profile")
	}
	if got := N("profile.count", 5); got != "5 profiles" {
		t.Errorf("N(profile.count, 5) = %q, want %q", got, "5 profiles")
	}
	if got := N("profile.count", 0); got != "0 profiles" {
		t.Errorf("N(profile.count, 0) = %q, want %q", got, "0 profiles")
	}
}

func TestNGerman(t *testing.T) {
	defer resetState(t)
	Init("de")
	if got := N("profile.count", 1); got != "1 Profil" {
		t.Errorf("N(profile.count, 1) = %q, want %q", got, "1 Profil")
	}
	if got := N("profile.count", 3); got != "3 Profile" {
		t.Errorf("N(profile.count, 3) = %q, want %q", got, "3 Profile")
	}
}

func TestConcurrentAccess(t *testing.T) {
	defer resetState(t)
	Init("en")
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			T("common.ok")
			Current()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}
