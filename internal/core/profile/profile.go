// Package profile stores per-installation settings as JSON files in the user
// config directory, migrates pre-2.0 ("jaqen") profiles, and persists app
// state (window geometry, theme, language).
package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"fmnewgenfaces/internal/brand"
	"fmnewgenfaces/internal/core/fmversion"
)

// stateFileName / migratedMarker are the reserved file names inside a
// Store's directory.
const (
	stateFileName  = "state.json"
	migratedMarker = ".migrated"
)

// Settings are the per-profile user choices.
type Settings struct {
	PackDir         string            `json:"pack_dir"`
	ConfigXML       string            `json:"config_xml"`
	RTFPath         string            `json:"rtf_path"`
	FMVersion       string            `json:"fm_version"` // fmversion Year key
	Preserve        bool              `json:"preserve"`
	AllowDuplicates bool              `json:"allow_duplicates"`
	Overrides       map[string]string `json:"overrides"` // nation code -> ethnic
}

// DefaultSettings: Preserve=true, AllowDuplicates=true, newest FM version.
func DefaultSettings() Settings {
	return Settings{
		FMVersion:       fmversion.Default().Year,
		Preserve:        true,
		AllowDuplicates: true,
		Overrides:       map[string]string{},
	}
}

func cloneSettings(s Settings) Settings {
	out := s
	out.Overrides = make(map[string]string, len(s.Overrides))
	for k, v := range s.Overrides {
		out.Overrides[k] = v
	}
	return out
}

// Profile is one saved configuration.
type Profile struct {
	// Slug is the file name (derived from Name, unique, filesystem-safe).
	Slug string `json:"slug"`
	Name string `json:"name"`
	// GamePath is the FM user-data folder this profile belongs to ("" if custom).
	GamePath  string    `json:"game_path"`
	Settings  Settings  `json:"settings"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Clone returns a deep copy (Overrides map included).
func (p *Profile) Clone() *Profile {
	if p == nil {
		return nil
	}
	np := *p
	np.Settings = cloneSettings(p.Settings)
	return &np
}

// Store manages profiles in one directory. All methods are safe for
// concurrent use.
type Store struct {
	mu       sync.Mutex
	dir      string
	profiles map[string]*Profile // by slug
}

// DefaultDir returns <os.UserConfigDir()>/<brand.ConfigDirName>, creating it.
func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, brand.ConfigDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// LegacyDirs returns pre-2.0 config dirs that may exist (~/.jaqen,
// ~/Library/Application Support/jaqen, %APPDATA%\jaqen).
func LegacyDirs() []string {
	home, homeErr := os.UserHomeDir()
	var candidates []string

	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			candidates = append(candidates, filepath.Join(appData, brand.LegacyConfigDirName))
		} else if homeErr == nil && home != "" {
			candidates = append(candidates, filepath.Join(home, "AppData", "Roaming", brand.LegacyConfigDirName))
		}
	case "darwin":
		if homeErr == nil && home != "" {
			candidates = append(candidates, filepath.Join(home, "Library", "Application Support", brand.LegacyConfigDirName))
		}
	default:
		if homeErr == nil && home != "" {
			candidates = append(candidates, filepath.Join(home, "."+brand.LegacyConfigDirName))
		}
	}

	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			out = append(out, c)
		}
	}
	return out
}

// Open loads all profiles from dir (creating it). On first run it migrates
// profiles from LegacyDirs() (old JSON shape: name/game_path/config{preserve,
// xml_path, rtf_path, img_path, fm_version, allow_duplicate, mapping_override})
// and writes a ".migrated" marker so it only happens once.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	marker := filepath.Join(dir, migratedMarker)
	if _, err := os.Stat(marker); os.IsNotExist(err) {
		for _, legacy := range LegacyDirs() {
			if _, mErr := MigrateLegacy(legacy, dir); mErr != nil {
				return nil, mErr
			}
		}
		_ = os.WriteFile(marker, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644)
	}

	s := &Store{dir: dir}
	if err := s.reload(); err != nil {
		return nil, err
	}
	return s, nil
}

// reload re-reads every *.json profile file (except state.json) from disk.
// Callers must hold no lock; reload manages its own locking of the result.
func (s *Store) reload() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}

	profiles := make(map[string]*Profile)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") || name == stateFileName {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, name))
		if err != nil {
			continue
		}
		var p Profile
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		if p.Slug == "" {
			p.Slug = strings.TrimSuffix(name, ".json")
		}
		if p.Settings.Overrides == nil {
			p.Settings.Overrides = map[string]string{}
		}
		profiles[p.Slug] = &p
	}

	s.mu.Lock()
	s.profiles = profiles
	s.mu.Unlock()
	return nil
}

// Dir returns the store directory.
func (s *Store) Dir() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dir
}

// List returns profiles sorted by name (copies).
func (s *Store) List() []*Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Profile, 0, len(s.profiles))
	for _, p := range s.profiles {
		out = append(out, p.Clone())
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// Get returns a copy of the profile with slug.
func (s *Store) Get(slug string) (*Profile, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.profiles[slug]
	if !ok {
		return nil, false
	}
	return p.Clone(), true
}

// FindByGamePath returns the profile bound to an FM install path.
func (s *Store) FindByGamePath(gamePath string) (*Profile, bool) {
	if gamePath == "" {
		return nil, false
	}
	clean := filepath.Clean(gamePath)

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.profiles {
		if p.GamePath != "" && filepath.Clean(p.GamePath) == clean {
			return p.Clone(), true
		}
	}
	return nil, false
}

// NameError is returned by ValidateName / Create / Rename.
type NameError struct{ Reason string }

func (e *NameError) Error() string { return e.Reason }

// ValidateName trims and checks: non-empty, <= 64 runes, no control chars.
func ValidateName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", &NameError{Reason: "name must not be empty"}
	}
	if utf8.RuneCountInString(trimmed) > 64 {
		return "", &NameError{Reason: "name must be 64 characters or fewer"}
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			return "", &NameError{Reason: "name must not contain control characters"}
		}
	}
	return trimmed, nil
}

var slugInvalidRE = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts a name into a safe file stem (lowercase, [a-z0-9-]).
func Slugify(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	slug := slugInvalidRE.ReplaceAllString(lower, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "profile"
	}
	return slug
}

// uniqueSlug appends "-2", "-3", ... to base until taken(candidate) is false.
func uniqueSlug(base string, taken func(string) bool) string {
	if !taken(base) {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !taken(candidate) {
			return candidate
		}
	}
}

// Create makes and saves a new profile. Settings are copied from base when
// non-nil, else DefaultSettings(). Name must be unique (case-insensitive).
func Create(s *Store, name, gamePath string, base *Settings) (*Profile, error) {
	trimmed, err := ValidateName(name)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	for _, p := range s.profiles {
		if strings.EqualFold(p.Name, trimmed) {
			s.mu.Unlock()
			return nil, &NameError{Reason: fmt.Sprintf("a profile named %q already exists", trimmed)}
		}
	}
	slug := uniqueSlug(Slugify(trimmed), func(candidate string) bool {
		_, ok := s.profiles[candidate]
		return ok
	})

	var settings Settings
	if base != nil {
		settings = cloneSettings(*base)
	} else {
		settings = DefaultSettings()
	}

	now := time.Now()
	p := &Profile{
		Slug:      slug,
		Name:      trimmed,
		GamePath:  gamePath,
		Settings:  settings,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.profiles[slug] = p
	s.mu.Unlock()

	if err := s.Save(p); err != nil {
		return nil, err
	}
	return p.Clone(), nil
}

// writeAtomic writes data to path via a temp file + rename in the same
// directory, so a crash never leaves a half-written file.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil {
		os.Remove(tmpName)
		return werr
	}
	if cerr != nil {
		os.Remove(tmpName)
		return cerr
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// Save writes p atomically (temp + rename) and updates UpdatedAt.
func (s *Store) Save(p *Profile) error {
	if p == nil {
		return errors.New("profile: nil profile")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if p.Slug == "" {
		p.Slug = uniqueSlug(Slugify(p.Name), func(candidate string) bool {
			_, ok := s.profiles[candidate]
			return ok
		})
	}
	p.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(s.dir, p.Slug+".json")
	if err := writeAtomic(path, data); err != nil {
		return err
	}

	if s.profiles == nil {
		s.profiles = make(map[string]*Profile)
	}
	s.profiles[p.Slug] = p.Clone()
	return nil
}

// Rename changes the display name (slug stays).
func (s *Store) Rename(slug, newName string) error {
	trimmed, err := ValidateName(newName)
	if err != nil {
		return err
	}

	s.mu.Lock()
	p, ok := s.profiles[slug]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("profile: %q not found", slug)
	}
	for otherSlug, other := range s.profiles {
		if otherSlug != slug && strings.EqualFold(other.Name, trimmed) {
			s.mu.Unlock()
			return &NameError{Reason: fmt.Sprintf("a profile named %q already exists", trimmed)}
		}
	}
	updated := p.Clone()
	updated.Name = trimmed
	s.mu.Unlock()

	return s.Save(updated)
}

// Delete removes the profile file.
func (s *Store) Delete(slug string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.profiles[slug]; !ok {
		return fmt.Errorf("profile: %q not found", slug)
	}
	path := filepath.Join(s.dir, slug+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	delete(s.profiles, slug)
	return nil
}

// State is app-wide (not per profile) persisted state.
type State struct {
	LastProfile     string    `json:"last_profile"`
	WindowWidth     float32   `json:"window_width"`
	WindowHeight    float32   `json:"window_height"`
	Theme           string    `json:"theme"`    // "system", "light", "dark"
	Language        string    `json:"language"` // "", "en", "de"
	LastUpdateCheck time.Time `json:"last_update_check"`
	SkippedVersion  string    `json:"skipped_version"`
	WizardDone      bool      `json:"wizard_done"`
}

// LoadState reads state.json (zero State if missing).
func (s *Store) LoadState() State {
	s.mu.Lock()
	dir := s.dir
	s.mu.Unlock()

	data, err := os.ReadFile(filepath.Join(dir, stateFileName))
	if err != nil {
		return State{}
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}
	}
	return st
}

// SaveState writes state.json atomically.
func (s *Store) SaveState(st State) error {
	s.mu.Lock()
	dir := s.dir
	s.mu.Unlock()

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, stateFileName), data)
}

// legacyConfig is the old ("jaqen") per-profile config shape. Pointer fields
// are nil when the user never touched that setting.
type legacyConfig struct {
	Preserve        *bool              `json:"preserve"`
	XMLPath         *string            `json:"xml_path"`
	RTFPath         *string            `json:"rtf_path"`
	IMGPath         *string            `json:"img_path"`
	FMVersion       *string            `json:"fm_version"`
	AllowDuplicate  *bool              `json:"allow_duplicate"`
	MappingOverride *map[string]string `json:"mapping_override"`
}

// legacyProfile is the old ("jaqen") profile JSON shape.
type legacyProfile struct {
	Name      string       `json:"name"`
	GamePath  string       `json:"game_path"`
	Config    legacyConfig `json:"config"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// MigrateLegacy imports every *.json profile under fromDir/profiles into
// toDir, translating the old ("jaqen") shape into the current one. It is
// side-effect free when fromDir/profiles does not exist. It returns the
// number of profiles migrated.
func MigrateLegacy(fromDir, toDir string) (int, error) {
	legacyProfilesDir := filepath.Join(fromDir, "profiles")
	entries, err := os.ReadDir(legacyProfilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	if err := os.MkdirAll(toDir, 0o755); err != nil {
		return 0, err
	}

	taken := make(map[string]bool)
	if existing, err := os.ReadDir(toDir); err == nil {
		for _, e := range existing {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") && e.Name() != stateFileName {
				taken[strings.TrimSuffix(e.Name(), ".json")] = true
			}
		}
	}

	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(legacyProfilesDir, e.Name()))
		if err != nil {
			continue
		}
		var lp legacyProfile
		if err := json.Unmarshal(data, &lp); err != nil {
			continue
		}

		settings := DefaultSettings()
		if lp.Config.Preserve != nil {
			settings.Preserve = *lp.Config.Preserve
		}
		if lp.Config.AllowDuplicate != nil {
			settings.AllowDuplicates = *lp.Config.AllowDuplicate
		}
		if lp.Config.FMVersion != nil && *lp.Config.FMVersion != "" {
			settings.FMVersion = *lp.Config.FMVersion
		}
		if lp.Config.XMLPath != nil {
			settings.ConfigXML = *lp.Config.XMLPath
		}
		if lp.Config.RTFPath != nil {
			settings.RTFPath = *lp.Config.RTFPath
		}
		if lp.Config.IMGPath != nil {
			settings.PackDir = *lp.Config.IMGPath
		}
		if lp.Config.MappingOverride != nil {
			for k, v := range *lp.Config.MappingOverride {
				settings.Overrides[strings.ToUpper(k)] = v
			}
		}

		name := strings.TrimSpace(lp.Name)
		if name == "" {
			name = strings.TrimSuffix(e.Name(), ".json")
		}
		slug := uniqueSlug(Slugify(name), func(candidate string) bool { return taken[candidate] })
		taken[slug] = true

		created := lp.CreatedAt
		if created.IsZero() {
			created = time.Now()
		}
		updated := lp.UpdatedAt
		if updated.IsZero() {
			updated = created
		}

		np := &Profile{
			Slug:      slug,
			Name:      name,
			GamePath:  lp.GamePath,
			Settings:  settings,
			CreatedAt: created,
			UpdatedAt: updated,
		}
		out, err := json.MarshalIndent(np, "", "  ")
		if err != nil {
			continue
		}
		if err := writeAtomic(filepath.Join(toDir, slug+".json"), out); err != nil {
			continue
		}
		count++
	}

	return count, nil
}

// Autosaver coalesces rapid Save calls: Trigger copies the profile and writes
// it after delay; further triggers within the delay replace the pending copy.
// Flush writes immediately. Errors go to onErr (may be nil).
type Autosaver struct {
	mu      sync.Mutex
	store   *Store
	delay   time.Duration
	pending *Profile
	timer   *time.Timer
	onErr   func(error)
}

// NewAutosaver creates an Autosaver that saves into store after delay of
// inactivity following the last Trigger.
func NewAutosaver(store *Store, delay time.Duration, onErr func(error)) *Autosaver {
	return &Autosaver{store: store, delay: delay, onErr: onErr}
}

// Trigger schedules p (a clone) to be saved after the debounce delay,
// replacing any not-yet-written pending save.
func (a *Autosaver) Trigger(p *Profile) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pending = p.Clone()
	if a.timer != nil {
		a.timer.Stop()
	}
	a.timer = time.AfterFunc(a.delay, a.fire)
}

func (a *Autosaver) fire() {
	a.mu.Lock()
	p := a.pending
	a.pending = nil
	a.timer = nil
	a.mu.Unlock()

	if p == nil {
		return
	}
	if err := a.store.Save(p); err != nil && a.onErr != nil {
		a.onErr(err)
	}
}

// Flush writes any pending profile synchronously and cancels the timer.
func (a *Autosaver) Flush() {
	a.mu.Lock()
	p := a.pending
	a.pending = nil
	if a.timer != nil {
		a.timer.Stop()
		a.timer = nil
	}
	a.mu.Unlock()

	if p == nil {
		return
	}
	if err := a.store.Save(p); err != nil && a.onErr != nil {
		a.onErr(err)
	}
}
