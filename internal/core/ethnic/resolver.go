package ethnic

import (
	"sort"
	"strings"
)

// UnknownNationError is returned by Resolve when the primary nation code has
// no entry in the table (and no override).
type UnknownNationError struct{ Code string }

func (e *UnknownNationError) Error() string { return "unknown nation code: " + e.Code }

// UnknownEthnicValueError is returned when the RTF ethnicity column holds a
// value outside 0..10.
type UnknownEthnicValueError struct{ Value int }

func (e *UnknownEthnicValueError) Error() string { return "unknown ethnic value" }

// OverrideError lists overrides whose ethnic value is not valid. Keys are the
// (already upper-cased) nation codes.
type OverrideError struct{ Invalid map[string]string }

func (e *OverrideError) Error() string { return "invalid mapping overrides" }

// Resolver is a per-run copy of DefaultNationTable with overrides applied.
// It is safe for concurrent reads; it is never mutated after creation.
type Resolver struct {
	table map[string]Ethnic
}

// NewResolver copies DefaultNationTable and applies overrides. Keys are
// trimmed and upper-cased; values may be given in any case that Parse accepts.
// Invalid values are collected into an *OverrideError; valid ones are still
// applied and a usable Resolver is returned alongside the error.
func NewResolver(overrides map[string]string) (*Resolver, error) {
	table := make(map[string]Ethnic, len(DefaultNationTable))
	for code, e := range DefaultNationTable {
		table[code] = e
	}

	var invalid map[string]string
	for rawKey, rawVal := range overrides {
		key := strings.ToUpper(strings.TrimSpace(rawKey))
		e, ok := Parse(rawVal)
		if !ok {
			if invalid == nil {
				invalid = make(map[string]string)
			}
			invalid[key] = rawVal
			continue
		}
		table[key] = e
	}

	r := &Resolver{table: table}
	if len(invalid) > 0 {
		return r, &OverrideError{Invalid: invalid}
	}
	return r, nil
}

// Lookup returns the ethnic group for a nation code (upper-cased, trimmed).
func (r *Resolver) Lookup(code string) (Ethnic, bool) {
	e, ok := r.table[strings.ToUpper(strings.TrimSpace(code))]
	return e, ok
}

// Resolve applies the FM ethnicity rules (ethnicValue 0..10, see legacy
// getEthnic) to a player's primary and secondary nation. nat2 may be empty.
// Returns *UnknownNationError when nat1 is unknown, *UnknownEthnicValueError
// for an out-of-range ethnicValue.
func (r *Resolver) Resolve(nat1, nat2 string, ethnicValue int) (Ethnic, error) {
	e1, ok := r.Lookup(nat1)
	if !ok {
		return "", &UnknownNationError{Code: strings.ToUpper(strings.TrimSpace(nat1))}
	}
	// Legacy getEthnic looked nat2 up silently: an unknown/empty nat2 simply
	// means "no ethnic" for that slot, it never errors.
	e2, _ := r.Lookup(nat2)

	has := func(e Ethnic) bool { return e1 == e || e2 == e }

	switch ethnicValue {
	case 0:
		if has(Scandinavian) {
			return Scandinavian, nil
		}
		if has(Caucasian) {
			return Caucasian, nil
		}
		return CentralEuropean, nil
	case 1:
		if has(Scandinavian) ||
			has(SouthEastAsian) ||
			has(CentralEuropean) ||
			has(Caucasian) ||
			has(African) ||
			has(Asian) ||
			has(MiddleEastNorthAfrican) ||
			has(MiddleEastSouthAsian) ||
			has(EasternEuropeanCentralAsian) {
			return SouthAmerican, nil
		}
		if e1 != "" {
			return e1, nil
		}
		return e2, nil
	case 2:
		if has(MiddleEastSouthAsian) {
			return MiddleEastSouthAsian, nil
		}
		return MiddleEastNorthAfrican, nil
	case 3, 6, 7, 8, 9:
		if ethnicValue == 7 {
			if e1 == SouthAmericanMediterranean {
				return SouthAmericanMediterranean, nil
			}
			if e1 == SouthAmerican {
				return SouthAmerican, nil
			}
		}
		return African, nil
	case 4:
		return MiddleEastSouthAsian, nil
	case 5:
		return SouthEastAsian, nil
	case 10:
		if e1 == SouthAmerican {
			return SouthAmerican, nil
		}
		return Asian, nil
	default:
		return "", &UnknownEthnicValueError{Value: ethnicValue}
	}
}

// Codes returns all known nation codes, sorted, for autocomplete.
func (r *Resolver) Codes() []string {
	codes := make([]string, 0, len(r.table))
	for c := range r.table {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	return codes
}

// DefaultFor returns the table entry from DefaultNationTable only (ignoring
// overrides), so an override editor can show "currently: X".
func DefaultFor(code string) (Ethnic, bool) {
	e, ok := DefaultNationTable[strings.ToUpper(strings.TrimSpace(code))]
	return e, ok
}
