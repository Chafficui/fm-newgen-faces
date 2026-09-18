// Package fmversion knows which Football Manager releases exist and how each
// one names newgen portraits inside config.xml.
package fmversion

import (
	"regexp"
	"strconv"
	"strings"
)

// Version describes one FM release.
type Version struct {
	// Year is the canonical key stored in profiles, e.g. "2024".
	Year string
	// Label is the short community name, e.g. "FM24".
	Label string
	// IDPrefix is prepended to the newgen UID inside the config.xml "to" path.
	// FM 2024 introduced "r-" (graphics/pictures/person/r-<uid>/portrait);
	// earlier versions use the bare UID.
	IDPrefix string
}

// Known lists supported versions, newest first. FM 2025 was cancelled by SI
// and FM26 is not supported; FromPath still recognises newer folders so a
// detected install is never mis-parsed, but they are not offered here.
var Known = []Version{
	{Year: "2024", Label: "FM24", IDPrefix: "r-"},
	{Year: "2023", Label: "FM23", IDPrefix: ""},
	{Year: "2022", Label: "FM22", IDPrefix: ""},
	{Year: "2021", Label: "FM21", IDPrefix: ""},
	{Year: "2020", Label: "FM20", IDPrefix: ""},
}

// Default is the newest known version.
func Default() Version { return Known[0] }

// Lookup finds a version by Year ("2024") or Label ("FM24", case-insensitive).
func Lookup(key string) (Version, bool) {
	k := strings.ToUpper(strings.TrimSpace(key))
	for _, v := range Known {
		if v.Year == k || strings.ToUpper(v.Label) == k {
			return v, true
		}
	}
	return Version{}, false
}

// Years returns the Year keys, newest first (for dropdowns).
func Years() []string {
	out := make([]string, len(Known))
	for i, v := range Known {
		out[i] = v.Year
	}
	return out
}

// Display returns "FM24 (2024)".
func (v Version) Display() string { return v.Label + " (" + v.Year + ")" }

var (
	// "Football Manager 2024" (FM20–FM24) and "Football Manager 26" (newer
	// releases, where SI dropped the century from the product name).
	yearRe  = regexp.MustCompile(`(?i)football\s*manager\s*(20\d\d|\d\d)\b`)
	shortRe = regexp.MustCompile(`(?i)\bfm\s*(\d\d)(\d\d)?\b`)
)

// FromPath extracts a version from any path segment such as
// ".../Football Manager 2024/graphics", ".../Football Manager 26/..." or
// ".../FM24/...". Unknown years that still look like a Football Manager
// folder are returned as an ad-hoc Version with the newest known IDPrefix,
// so a new release is never silently dropped.
func FromPath(p string) (Version, bool) {
	if m := yearRe.FindStringSubmatch(p); m != nil {
		year := m[1]
		if len(year) == 2 {
			year = "20" + year
		}
		return fromYear(year), true
	}
	if m := shortRe.FindStringSubmatch(p); m != nil {
		if m[2] != "" { // FM2024
			return fromYear(m[1] + m[2]), true
		}
		return fromYear("20" + m[1]), true // FM24
	}
	return Version{}, false
}

func fromYear(year string) Version {
	if v, ok := Lookup(year); ok {
		return v
	}
	y, _ := strconv.Atoi(year)
	prefix := Default().IDPrefix
	if y < 2024 {
		prefix = ""
	}
	return Version{Year: year, Label: "FM" + year[2:], IDPrefix: prefix}
}
