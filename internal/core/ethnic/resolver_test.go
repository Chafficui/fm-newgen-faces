package ethnic

import (
	"errors"
	"sort"
	"testing"
)

func TestResolveEthnicValueBranches(t *testing.T) {
	r, err := NewResolver(nil)
	if err != nil {
		t.Fatalf("NewResolver(nil) returned error: %v", err)
	}

	tests := []struct {
		name        string
		nat1, nat2  string
		ethnicValue int
		want        Ethnic
	}{
		// value 0: Scandinavian > Caucasian > CentralEuropean fallback.
		{"0 scandinavian", "DEN", "", 0, Scandinavian},
		{"0 caucasian", "ENG", "", 0, Caucasian},
		{"0 fallback central european", "GER", "", 0, CentralEuropean},

		// value 1: any of a long list of ethnics -> SouthAmerican, else e1 fallback.
		{"1 in set -> south american", "ENG", "", 1, SouthAmerican},
		{"1 fallback to e1", "ITA", "", 1, ItalianMediterranean},
		{"1 fallback to e1 spanmed", "ESP", "", 1, SpanishMediterranean},

		// value 2: MESA else MENA.
		{"2 mesa", "IND", "", 2, MiddleEastSouthAsian},
		{"2 fallback mena", "GER", "", 2, MiddleEastNorthAfrican},

		// value 3,6,8,9: African always.
		{"3 african", "RSA", "", 3, African},
		{"6 african", "GER", "", 6, African},
		{"8 african", "GER", "", 8, African},
		{"9 african", "GER", "", 9, African},

		// value 7: special-cased on e1.
		{"7 samed", "ARG", "", 7, SouthAmericanMediterranean},
		{"7 south american", "BRA", "", 7, SouthAmerican},
		{"7 fallback african", "GER", "", 7, African},

		// value 4: always MESA.
		{"4 always mesa", "GER", "", 4, MiddleEastSouthAsian},

		// value 5: always SouthEastAsian.
		{"5 always seasian", "GER", "", 5, SouthEastAsian},

		// value 10: SouthAmerican else Asian.
		{"10 south american", "BRA", "", 10, SouthAmerican},
		{"10 fallback asian", "GER", "", 10, Asian},

		// example from the task description.
		{"example ger/rsa value 3", "GER", "RSA", 3, African},

		// unknown nat2 is looked up silently and never errors.
		{"unknown nat2 silent", "GER", "ZZZ", 0, CentralEuropean},

		// nat2 can push ethnicValue 0/1/2 branches too.
		{"0 nat2 scandinavian", "GER", "DEN", 0, Scandinavian},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.Resolve(tc.nat1, tc.nat2, tc.ethnicValue)
			if err != nil {
				t.Fatalf("Resolve(%q,%q,%d) unexpected error: %v", tc.nat1, tc.nat2, tc.ethnicValue, err)
			}
			if got != tc.want {
				t.Errorf("Resolve(%q,%q,%d) = %q, want %q", tc.nat1, tc.nat2, tc.ethnicValue, got, tc.want)
			}
		})
	}
}

func TestResolveUnknownNation(t *testing.T) {
	r, err := NewResolver(nil)
	if err != nil {
		t.Fatalf("NewResolver(nil) returned error: %v", err)
	}

	_, err = r.Resolve("ZZZ", "", 0)
	if err == nil {
		t.Fatal("expected error for unknown primary nation")
	}
	var unknownNation *UnknownNationError
	if !errors.As(err, &unknownNation) {
		t.Fatalf("expected *UnknownNationError, got %T: %v", err, err)
	}
	if unknownNation.Code != "ZZZ" {
		t.Errorf("Code = %q, want ZZZ", unknownNation.Code)
	}
}

func TestResolveUnknownEthnicValue(t *testing.T) {
	r, err := NewResolver(nil)
	if err != nil {
		t.Fatalf("NewResolver(nil) returned error: %v", err)
	}

	for _, v := range []int{-1, 11, 42} {
		_, err := r.Resolve("GER", "", v)
		if err == nil {
			t.Fatalf("expected error for ethnicValue %d", v)
		}
		var unknownValue *UnknownEthnicValueError
		if !errors.As(err, &unknownValue) {
			t.Fatalf("expected *UnknownEthnicValueError for %d, got %T: %v", v, err, err)
		}
		if unknownValue.Value != v {
			t.Errorf("Value = %d, want %d", unknownValue.Value, v)
		}
	}
}

func TestNewResolverOverrideKeyNormalisation(t *testing.T) {
	r, err := NewResolver(map[string]string{"esp": "african"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := r.Lookup("ESP")
	if !ok || got != African {
		t.Fatalf("Lookup(ESP) = %q, %v, want African, true", got, ok)
	}
	// Lookup itself trims/uppercases too.
	got2, ok2 := r.Lookup(" esp ")
	if !ok2 || got2 != African {
		t.Fatalf("Lookup( esp ) = %q, %v, want African, true", got2, ok2)
	}
}

func TestNewResolverInvalidOverridesStillApplyValid(t *testing.T) {
	r, err := NewResolver(map[string]string{
		"ESP": "not-a-real-ethnic",
		"GER": "African",
	})
	if err == nil {
		t.Fatal("expected an *OverrideError for the invalid value")
	}
	var overrideErr *OverrideError
	if !errors.As(err, &overrideErr) {
		t.Fatalf("expected *OverrideError, got %T: %v", err, err)
	}
	if v, ok := overrideErr.Invalid["ESP"]; !ok || v != "not-a-real-ethnic" {
		t.Errorf("Invalid[ESP] = %q, %v, want not-a-real-ethnic, true", v, ok)
	}
	if _, ok := overrideErr.Invalid["GER"]; ok {
		t.Errorf("GER should not be reported invalid")
	}

	// The resolver returned alongside the error must still be usable, with
	// the valid override applied.
	if r == nil {
		t.Fatal("resolver must not be nil even when overrides contain errors")
	}
	got, ok := r.Lookup("GER")
	if !ok || got != African {
		t.Fatalf("Lookup(GER) = %q, %v, want African, true (valid override applied)", got, ok)
	}
	// The invalid override must not have touched the table's ESP entry.
	got, ok = r.Lookup("ESP")
	if !ok || got != SpanishMediterranean {
		t.Fatalf("Lookup(ESP) = %q, %v, want SpanishMediterranean, true (unaffected by invalid override)", got, ok)
	}
}

func TestNewResolverDoesNotMutateDefaultTable(t *testing.T) {
	before := DefaultNationTable["ESP"]

	_, err := NewResolver(map[string]string{"esp": "African"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after := DefaultNationTable["ESP"]
	if before != after {
		t.Fatalf("DefaultNationTable[ESP] mutated: before=%q after=%q", before, after)
	}
	if after != SpanishMediterranean {
		t.Fatalf("DefaultNationTable[ESP] = %q, want SpanishMediterranean", after)
	}
}

func TestResolverCodesSorted(t *testing.T) {
	r, err := NewResolver(map[string]string{"zzz": "African"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	codes := r.Codes()
	if !sort.StringsAreSorted(codes) {
		t.Fatalf("Codes() not sorted: %v", codes)
	}
	found := false
	for _, c := range codes {
		if c == "ZZZ" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Codes() missing override-added code ZZZ: %v", codes)
	}
}

func TestDefaultForReadsOnlyDefaultTable(t *testing.T) {
	got, ok := DefaultFor("esp")
	if !ok || got != SpanishMediterranean {
		t.Fatalf("DefaultFor(esp) = %q, %v, want SpanishMediterranean, true", got, ok)
	}

	if _, ok := DefaultFor("ZZZ"); ok {
		t.Fatal("DefaultFor(ZZZ) should be false")
	}

	// DefaultFor must ignore any resolver-level overrides.
	if _, err := NewResolver(map[string]string{"esp": "African"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok = DefaultFor("ESP")
	if !ok || got != SpanishMediterranean {
		t.Fatalf("DefaultFor(ESP) after override = %q, %v, want SpanishMediterranean, true", got, ok)
	}
}
