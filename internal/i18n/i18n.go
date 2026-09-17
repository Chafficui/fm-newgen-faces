// Package i18n provides UI strings from embedded JSON locale files
// (locales/en.json is the reference; other locales fall back to it).
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

//go:embed locales/*.json
var localeFS embed.FS

// Lang describes an available UI language.
type Lang struct {
	Code string // "en", "de"
	Name string // native name: "English", "Deutsch"
}

// langNames gives the native display name for known language codes; unknown
// codes just show their code.
var langNames = map[string]string{
	"en": "English",
	"de": "Deutsch",
}

var (
	mu       sync.RWMutex
	current  = "en"
	messages map[string]map[string]string // lang -> key -> value
	loaded   bool
)

// loadAll parses every embedded locales/*.json file once, merging files that
// share a language prefix (the part of the filename before the first '.'),
// e.g. "en.json" and "en.widgets.json" both merge into "en".
func loadAll() {
	mu.Lock()
	defer mu.Unlock()
	if loaded {
		return
	}
	loaded = true
	messages = map[string]map[string]string{}

	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		lang := langFromFilename(e.Name())

		data, err := localeFS.ReadFile("locales/" + e.Name())
		if err != nil {
			continue
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		if messages[lang] == nil {
			messages[lang] = map[string]string{}
		}
		for k, v := range m {
			messages[lang][k] = v
		}
	}
}

// langFromFilename returns the language code a locale filename merges into:
// everything before the first '.' ("en.json" -> "en", "en.widgets.json" -> "en").
func langFromFilename(name string) string {
	if idx := strings.IndexByte(name, '.'); idx >= 0 {
		return name[:idx]
	}
	return name
}

// Available lists embedded locales, English first.
func Available() []Lang {
	loadAll()

	mu.RLock()
	codes := make([]string, 0, len(messages))
	for code := range messages {
		codes = append(codes, code)
	}
	mu.RUnlock()

	sort.Strings(codes)

	out := make([]Lang, 0, len(codes))
	hasEnglish := false
	for _, code := range codes {
		if code == "en" {
			hasEnglish = true
			continue
		}
	}
	if hasEnglish {
		out = append(out, Lang{Code: "en", Name: langName("en")})
	}
	for _, code := range codes {
		if code == "en" {
			continue
		}
		out = append(out, Lang{Code: code, Name: langName(code)})
	}
	return out
}

func langName(code string) string {
	if n, ok := langNames[code]; ok {
		return n
	}
	return code
}

// Init selects the language. code "" means: use $LANG / OS locale, default "en".
func Init(code string) {
	loadAll()

	if code == "" {
		code = detectOSLang()
	}
	code = normalizeCode(code)

	mu.Lock()
	defer mu.Unlock()
	if _, ok := messages[code]; ok {
		current = code
	} else {
		current = "en"
	}
}

func normalizeCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if len(code) > 2 {
		code = code[:2]
	}
	return code
}

// detectOSLang reads $LANG, then $LC_ALL, then $LC_MESSAGES, taking the
// first two letters of whichever is set first (e.g. "de_DE.UTF-8" -> "de").
func detectOSLang() string {
	for _, env := range []string{"LANG", "LC_ALL", "LC_MESSAGES"} {
		v := strings.TrimSpace(os.Getenv(env))
		if v == "" {
			continue
		}
		return v
	}
	return "en"
}

// Current returns the active language code.
func Current() string {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// T returns the translated string for key, formatted with fmt.Sprintf when
// args are given. Missing keys fall back to English, then to the key itself.
func T(key string, args ...any) string {
	loadAll()

	mu.RLock()
	cur := current
	mu.RUnlock()

	if v, ok := lookup(cur, key); ok {
		return format(v, args)
	}
	if cur != "en" {
		if v, ok := lookup("en", key); ok {
			return format(v, args)
		}
	}
	return key
}

func lookup(lang, key string) (string, bool) {
	mu.RLock()
	defer mu.RUnlock()
	m, ok := messages[lang]
	if !ok {
		return "", false
	}
	v, ok := m[key]
	return v, ok
}

func format(v string, args []any) string {
	if len(args) == 0 {
		return v
	}
	return fmt.Sprintf(v, args...)
}

// N picks a singular/plural key ("key.one" / "key.other") based on n and
// formats it with n as the first argument.
func N(key string, n int, args ...any) string {
	suffix := ".other"
	if n == 1 {
		suffix = ".one"
	}
	full := make([]any, 0, len(args)+1)
	full = append(full, n)
	full = append(full, args...)
	return T(key+suffix, full...)
}
