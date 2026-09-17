// Package fmconfig reads and writes the Football Manager graphics config.xml:
//
//	<record>
//	  <boolean id="preload" value="false"/>
//	  <boolean id="amap" value="false"/>
//	  <list id="maps">
//	    <record from="faces/African/abc" to="graphics/pictures/person/r-2000134233/portrait"/>
//	  </list>
//	</record>
//
// Records whose "to" path is not a newgen portrait for the selected version
// (real players, kits, logos, other versions' prefix) are kept verbatim and
// re-emitted unchanged, so the tool never loses mappings it does not own.
package fmconfig

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"fmnewgenfaces/internal/core/ethnic"
	"fmnewgenfaces/internal/core/fmversion"
)

// Record is one <record from= to=/> element.
type Record struct {
	From string `xml:"from,attr"`
	To   string `xml:"to,attr"`
}

// Config is an in-memory config.xml.
type Config struct {
	Path    string
	Preload bool
	Amap    bool
	Version fmversion.Version

	newgen      map[string]string // uid -> from
	order       []string          // uids in first-seen order for stable output
	passthrough []Record          // untouched foreign records, original order
}

// PortraitPath returns "graphics/pictures/person/<prefix><uid>/portrait".
func PortraitPath(v fmversion.Version, uid string) string {
	return "graphics/pictures/person/" + v.IDPrefix + uid + "/portrait"
}

// ParsePortrait extracts the uid from a "to" path IF it is a newgen portrait
// for v (correct prefix, digits only). Any other path returns ok=false.
func ParsePortrait(v fmversion.Version, to string) (uid string, ok bool) {
	clean := strings.ReplaceAll(to, "\\", "/")
	clean = strings.TrimPrefix(clean, "/")

	pattern := "^graphics/pictures/person/" + regexp.QuoteMeta(v.IDPrefix) + `(\d+)/portrait$`
	re := regexp.MustCompile(pattern)

	m := re.FindStringSubmatch(clean)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// New returns an empty config (preload=false, amap=false) for path.
func New(path string, v fmversion.Version) *Config {
	return &Config{
		Path:    path,
		Preload: false,
		Amap:    false,
		Version: v,
		newgen:  make(map[string]string),
	}
}

// xmlDoc mirrors the config.xml shape for decoding.
type xmlDoc struct {
	XMLName xml.Name `xml:"record"`
	Boolean []struct {
		ID    string `xml:"id,attr"`
		Value string `xml:"value,attr"`
	} `xml:"boolean"`
	List struct {
		ID     string   `xml:"id,attr"`
		Record []Record `xml:"record"`
	} `xml:"list"`
}

// Load parses path. A missing file returns an error satisfying
// errors.Is(err, fs.ErrNotExist); malformed XML returns a descriptive error.
func Load(path string, v fmversion.Version) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw xmlDoc
	if err := xml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("fmconfig: parse %s: %w", path, err)
	}

	cfg := New(path, v)
	for _, b := range raw.Boolean {
		switch b.ID {
		case "preload":
			cfg.Preload = strings.EqualFold(b.Value, "true")
		case "amap":
			cfg.Amap = strings.EqualFold(b.Value, "true")
		}
	}

	for _, r := range raw.List.Record {
		if uid, ok := ParsePortrait(v, r.To); ok {
			if _, exists := cfg.newgen[uid]; !exists {
				cfg.order = append(cfg.order, uid)
			}
			cfg.newgen[uid] = r.From
			continue
		}
		cfg.passthrough = append(cfg.passthrough, r)
	}

	return cfg, nil
}

// LoadOrNew loads path, or returns New when the file does not exist.
func LoadOrNew(path string, v fmversion.Version) (*Config, error) {
	cfg, err := Load(path, v)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return New(path, v), nil
		}
		return nil, err
	}
	return cfg, nil
}

// Has reports whether uid already has a newgen mapping.
func (c *Config) Has(uid string) bool {
	_, ok := c.newgen[uid]
	return ok
}

// Get returns the "from" path for uid.
func (c *Config) Get(uid string) (string, bool) {
	v, ok := c.newgen[uid]
	return v, ok
}

// Set maps uid to from (a forward-slash path relative to the config dir).
func (c *Config) Set(uid, from string) {
	if c.newgen == nil {
		c.newgen = make(map[string]string)
	}
	if _, exists := c.newgen[uid]; !exists {
		c.order = append(c.order, uid)
	}
	c.newgen[uid] = from
}

// Delete removes the newgen mapping for uid.
func (c *Config) Delete(uid string) {
	if _, exists := c.newgen[uid]; !exists {
		return
	}
	delete(c.newgen, uid)
	for i, id := range c.order {
		if id == uid {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

// Newgens returns uid -> from for all newgen mappings (copy).
func (c *Config) Newgens() map[string]string {
	out := make(map[string]string, len(c.newgen))
	for k, v := range c.newgen {
		out[k] = v
	}
	return out
}

// AssignedImages returns every "from" path currently mapped to a newgen.
func (c *Config) AssignedImages() []string {
	out := make([]string, 0, len(c.order))
	for _, uid := range c.order {
		out = append(out, c.newgen[uid])
	}
	return out
}

// Passthrough returns the foreign records kept verbatim (copy).
func (c *Config) Passthrough() []Record {
	out := make([]Record, len(c.passthrough))
	copy(out, c.passthrough)
	return out
}

// Count returns the number of newgen mappings.
func (c *Config) Count() int { return len(c.newgen) }

// ImagePath builds the "from" value for an image: the path of
// <packRoot>/<ethnic>/<image> relative to the DIRECTORY containing configPath,
// with forward slashes and no leading "./". When the pack root IS the config
// directory this is "<ethnic>/<image>".
func ImagePath(configPath, packRoot string, e ethnic.Ethnic, image string) (string, error) {
	configDir := filepath.Dir(configPath)
	full := filepath.Join(packRoot, string(e), image)

	rel, err := filepath.Rel(configDir, full)
	if err != nil {
		// Different volumes (Windows drive letters / UNC): a relative path is
		// impossible. Fall back to the absolute path rather than aborting
		// the run; pipeline.Check warns about this setup up front.
		abs, aerr := filepath.Abs(full)
		if aerr != nil {
			return "", err
		}
		return filepath.ToSlash(abs), nil
	}

	rel = filepath.ToSlash(rel)
	rel = strings.TrimPrefix(rel, "./")
	return rel, nil
}

// SaveOptions controls Save.
type SaveOptions struct {
	// BackupDir, when non-empty, receives a timestamped copy of the previous
	// file before it is replaced (see backup.go). Ignored if no file exists.
	BackupDir string
	// KeepBackups prunes older backups beyond this count (0 = keep all).
	KeepBackups int
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	// xml.EscapeText escapes &, <, >, ' and " which is safe for both text
	// content and attribute values.
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// marshal renders the config as XML text: declaration, tab indentation,
// forward slashes, passthrough records first (original order) then newgen
// records in first-seen order.
func (c *Config) marshal() []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString("<record>\n")
	fmt.Fprintf(&b, "\t<boolean id=\"preload\" value=\"%t\"/>\n", c.Preload)
	fmt.Fprintf(&b, "\t<boolean id=\"amap\" value=\"%t\"/>\n", c.Amap)
	b.WriteString("\t<list id=\"maps\">\n")

	for _, r := range c.passthrough {
		fmt.Fprintf(&b, "\t\t<record from=\"%s\" to=\"%s\"/>\n", xmlEscape(r.From), xmlEscape(r.To))
	}
	for _, uid := range c.order {
		from := filepath.ToSlash(c.newgen[uid])
		to := PortraitPath(c.Version, uid)
		fmt.Fprintf(&b, "\t\t<record from=\"%s\" to=\"%s\"/>\n", xmlEscape(from), xmlEscape(to))
	}

	b.WriteString("\t</list>\n")
	b.WriteString("</record>")
	return []byte(b.String())
}

// Save writes the file atomically (temp file + rename) with an XML
// declaration, tab indentation and forward slashes. It returns the backup
// path if one was made.
func (c *Config) Save(opts SaveOptions) (backupPath string, err error) {
	dir := filepath.Dir(c.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	if opts.BackupDir != "" {
		if _, statErr := os.Stat(c.Path); statErr == nil {
			bp, backupErr := backupFile(c.Path, opts.BackupDir)
			if backupErr != nil {
				return "", backupErr
			}
			backupPath = bp
			if opts.KeepBackups > 0 {
				if err := pruneBackups(opts.BackupDir, opts.KeepBackups); err != nil {
					return backupPath, err
				}
			}
		}
	}

	data := c.marshal()

	tmpPath := c.Path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return backupPath, err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return backupPath, err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return backupPath, err
	}
	if err := os.Rename(tmpPath, c.Path); err != nil {
		return backupPath, err
	}

	return backupPath, nil
}

// Generate writes a default empty config.xml at path unless it exists.
func Generate(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	content := `<?xml version="1.0" encoding="UTF-8"?>
<record>
	<boolean id="preload" value="false"/>
	<boolean id="amap" value="false"/>
	<list id="maps">
	</list>
</record>`

	dir := filepath.Dir(path)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return os.WriteFile(path, []byte(content), 0o644)
}

// Backup describes a stored copy of a config.xml.
type Backup struct {
	Path string
	Time time.Time
	Size int64
}

// BackupDirFor returns <baseDir>/<hash of configPath>, so each config.xml
// has its own backup folder.
func BackupDirFor(baseDir, configPath string) string {
	abs, err := filepath.Abs(configPath)
	if err != nil {
		abs = configPath
	}
	clean := filepath.Clean(abs)

	sum := sha1.Sum([]byte(clean))
	hexStr := hex.EncodeToString(sum[:])[:12]

	return filepath.Join(baseDir, hexStr)
}

// nowFunc is the clock used to name backup files. It is a variable so tests
// can produce deterministic, non-colliding timestamps without sleeping.
var nowFunc = time.Now

const backupTimeLayout = "20060102-150405"

var backupNameRe = regexp.MustCompile(`^config-(\d{8}-\d{6})(?:-\d+)?\.xml$`)

func backupFile(srcPath, backupDir string) (string, error) {
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", err
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", err
	}

	// Same-second saves (e.g. two quick rerolls) must not overwrite each
	// other's backup, so disambiguate with a counter suffix when needed.
	stem := "config-" + nowFunc().Format(backupTimeLayout)
	dest := filepath.Join(backupDir, stem+".xml")
	for i := 2; ; i++ {
		if _, err := os.Stat(dest); err != nil {
			break
		}
		dest = filepath.Join(backupDir, fmt.Sprintf("%s-%d.xml", stem, i))
	}
	tmp := dest + ".tmp"

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return "", err
	}

	return dest, nil
}

// backupSeq returns the "-N" collision counter of a backup file name (1 if none).
func backupSeq(path string) int {
	m := backupNameSeqRe.FindStringSubmatch(filepath.Base(path))
	if m == nil || m[1] == "" {
		return 1
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

var backupNameSeqRe = regexp.MustCompile(`^config-\d{8}-\d{6}(?:-(\d+))?\.xml$`)

func pruneBackups(dir string, keep int) error {
	backups, err := ListBackups(dir)
	if err != nil {
		return err
	}
	if len(backups) <= keep {
		return nil
	}
	for _, b := range backups[keep:] {
		if err := os.Remove(b.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

// ListBackups returns backups in dir, newest first.
func ListBackups(dir string) ([]Backup, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var backups []Backup
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := backupNameRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		t, err := time.ParseInLocation(backupTimeLayout, m[1], time.Local)
		if err != nil {
			continue
		}
		var size int64
		if info, err := e.Info(); err == nil {
			size = info.Size()
		}
		backups = append(backups, Backup{
			Path: filepath.Join(dir, e.Name()),
			Time: t,
			Size: size,
		})
	}

	sort.Slice(backups, func(i, j int) bool {
		if !backups[i].Time.Equal(backups[j].Time) {
			return backups[i].Time.After(backups[j].Time)
		}
		// Same second: the disambiguating counter suffix marks the later one.
		return backupSeq(backups[i].Path) > backupSeq(backups[j].Path)
	})

	return backups, nil
}

// Restore copies backupPath over configPath (atomically).
func Restore(backupPath, configPath string) error {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return err
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// A restore must itself be undoable: keep the current file as a backup
	// in the same backup folder before overwriting it.
	if _, err := os.Stat(configPath); err == nil {
		if _, err := backupFile(configPath, filepath.Dir(backupPath)); err != nil {
			return fmt.Errorf("backup current config before restore: %w", err)
		}
	}

	tmp := configPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, configPath); err != nil {
		os.Remove(tmp)
		return err
	}

	return nil
}
