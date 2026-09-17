// Package bugreport builds the markdown a user pastes into a GitHub issue,
// with the home directory redacted and the app version included.
package bugreport

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"

	"fmnewgenfaces/internal/brand"
	"fmnewgenfaces/internal/core/profile"
)

// Info is everything the report needs.
type Info struct {
	Version   string
	Profiles  []*profile.Profile
	Current   *profile.Profile
	LogTail   string // last N lines of the log file, may be empty
	Checklist string // rendered setup checklist, may be empty
}

// Redact replaces the user's home directory (and %USERPROFILE% on Windows)
// with "~" everywhere in s.
func Redact(s string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		s = replaceHomeVariants(s, home)
	}
	if up := os.Getenv("USERPROFILE"); up != "" {
		s = replaceHomeVariants(s, up)
	}
	s = strings.ReplaceAll(s, "%USERPROFILE%", "~")
	return s
}

// replaceHomeVariants replaces both slash styles of home in s with "~",
// case-insensitively on Windows (where paths are case-insensitive).
func replaceHomeVariants(s, home string) string {
	variants := map[string]bool{
		home:                               true,
		filepath.ToSlash(home):             true,
		strings.ReplaceAll(home, "/", `\`): true,
	}
	for variant := range variants {
		if variant == "" {
			continue
		}
		if runtime.GOOS == "windows" {
			s = replaceCaseInsensitive(s, variant, "~")
		} else {
			s = strings.ReplaceAll(s, variant, "~")
		}
	}
	return s
}

func replaceCaseInsensitive(s, old, replacement string) string {
	if old == "" {
		return s
	}
	re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(old))
	return re.ReplaceAllString(s, replacement)
}

// Build renders the report (already redacted).
func Build(info Info) string {
	var b strings.Builder

	b.WriteString("## Bug Report\n\n")
	fmt.Fprintf(&b, "**%s version:** %s\n\n", brand.AppName, valueOr(info.Version, "unknown"))
	fmt.Fprintf(&b, "**OS/Arch:** %s/%s\n\n", runtime.GOOS, runtime.GOARCH)

	b.WriteString("### Profiles\n\n")
	if info.Current != nil {
		fmt.Fprintf(&b, "**Current profile:** %s\n\n", info.Current.Name)
	}
	if len(info.Profiles) == 0 {
		b.WriteString("_No profiles found._\n\n")
	}
	for _, p := range info.Profiles {
		fmt.Fprintf(&b, "#### %s\n", p.Name)
		fmt.Fprintf(&b, "- Game path: `%s`\n", Redact(p.GamePath))
		fmt.Fprintf(&b, "- Pack dir: `%s`\n", Redact(p.Settings.PackDir))
		fmt.Fprintf(&b, "- Config XML: `%s`\n", Redact(p.Settings.ConfigXML))
		fmt.Fprintf(&b, "- RTF path: `%s`\n", Redact(p.Settings.RTFPath))
		fmt.Fprintf(&b, "- FM version: %s\n", p.Settings.FMVersion)
		fmt.Fprintf(&b, "- Preserve: %v\n", p.Settings.Preserve)
		fmt.Fprintf(&b, "- Allow duplicates: %v\n", p.Settings.AllowDuplicates)
		if len(p.Settings.Overrides) > 0 {
			b.WriteString("- Overrides:\n")
			keys := make([]string, 0, len(p.Settings.Overrides))
			for k := range p.Settings.Overrides {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(&b, "  - %s -> %s\n", k, p.Settings.Overrides[k])
			}
		}
		b.WriteString("\n")
	}

	if info.Checklist != "" {
		b.WriteString("### Checklist\n\n")
		b.WriteString(Redact(info.Checklist))
		b.WriteString("\n\n")
	}

	b.WriteString("### Log tail\n\n")
	if info.LogTail != "" {
		b.WriteString("```\n")
		b.WriteString(Redact(info.LogTail))
		b.WriteString("\n```\n\n")
	} else {
		b.WriteString("_No log available._\n\n")
	}

	b.WriteString("### Description / Steps to reproduce\n\n")
	b.WriteString("<!-- Please describe what happened and what you expected to happen. -->\n\n")
	b.WriteString("1. \n2. \n3. \n")

	return b.String()
}

func valueOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// maxIssueBodyBytes keeps the encoded URL well under common browser/server
// limits.
const maxIssueBodyBytes = 6000

// IssueURL returns brand.IssuesURL with a prefilled title and body query
// (body truncated to stay under 6 KB).
func IssueURL(title, body string) string {
	if len(body) > maxIssueBodyBytes {
		const note = "\n\n_...truncated, see the full report in the app..._"
		cut := maxIssueBodyBytes - len(note)
		if cut < 0 {
			cut = 0
		}
		body = truncateUTF8(body, cut) + note
	}
	return fmt.Sprintf("%s?title=%s&body=%s", brand.IssuesURL, url.QueryEscape(title), url.QueryEscape(body))
}

// truncateUTF8 cuts s to at most n bytes without splitting a multi-byte rune.
func truncateUTF8(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if n >= len(s) {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
