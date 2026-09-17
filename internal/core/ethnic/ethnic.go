// Package ethnic defines the 14 face-pack ethnic groups, the default
// nation→ethnic table and a per-run Resolver that applies user overrides
// without mutating global state.
package ethnic

import "strings"

// Ethnic is the name of a face-pack subfolder.
type Ethnic string

const (
	African                     Ethnic = "African"
	Asian                       Ethnic = "Asian"
	Caucasian                   Ethnic = "Caucasian"
	CentralEuropean             Ethnic = "Central European"
	EasternEuropeanCentralAsian Ethnic = "EECA"
	ItalianMediterranean        Ethnic = "Italmed"
	MiddleEastNorthAfrican      Ethnic = "MENA"
	MiddleEastSouthAsian        Ethnic = "MESA"
	SouthAmericanMediterranean  Ethnic = "SAMed"
	Scandinavian                Ethnic = "Scandinavian"
	SouthEastAsian              Ethnic = "Seasian"
	SouthAmerican               Ethnic = "South American"
	SpanishMediterranean        Ethnic = "SpanMed"
	YugoslavGreek               Ethnic = "YugoGreek"
)

// All lists every ethnic group in display order.
var All = []Ethnic{
	African, Asian, Caucasian, CentralEuropean, EasternEuropeanCentralAsian,
	ItalianMediterranean, MiddleEastNorthAfrican, MiddleEastSouthAsian,
	SouthAmericanMediterranean, Scandinavian, SouthEastAsian, SouthAmerican,
	SpanishMediterranean, YugoslavGreek,
}

// String implements fmt.Stringer.
func (e Ethnic) String() string { return string(e) }

// Description returns a human readable long name (e.g. "Middle East North African").
func (e Ethnic) Description() string {
	switch e {
	case EasternEuropeanCentralAsian:
		return "Eastern European / Central Asian"
	case ItalianMediterranean:
		return "Italian Mediterranean"
	case MiddleEastNorthAfrican:
		return "Middle East / North African"
	case MiddleEastSouthAsian:
		return "Middle East / South Asian"
	case SouthAmericanMediterranean:
		return "South American Mediterranean"
	case SouthEastAsian:
		return "South East Asian"
	case SpanishMediterranean:
		return "Spanish Mediterranean"
	case YugoslavGreek:
		return "Yugoslav / Greek"
	default:
		return string(e)
	}
}

// Parse matches s against All, case-insensitively and ignoring surrounding
// whitespace, so folder names like "african" or "south american" resolve.
func Parse(s string) (Ethnic, bool) {
	n := strings.ToLower(strings.TrimSpace(s))
	for _, e := range All {
		if strings.ToLower(string(e)) == n {
			return e, true
		}
	}
	return "", false
}

// IsValid reports whether s is exactly one of All.
func IsValid(s string) bool {
	for _, e := range All {
		if string(e) == s {
			return true
		}
	}
	return false
}

// Names returns All as strings (for dropdowns).
func Names() []string {
	out := make([]string, len(All))
	for i, e := range All {
		out[i] = string(e)
	}
	return out
}
