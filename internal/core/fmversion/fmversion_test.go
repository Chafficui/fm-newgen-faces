package fmversion

import "testing"

func TestFromPath(t *testing.T) {
	cases := map[string]string{
		`C:\Users\x\Documents\Sports Interactive\Football Manager 2024\graphics\faces`: "2024",
		`/home/x/Documents/Sports Interactive/Football Manager 26/graphics`:            "2026",
		`/x/Football Manager 2023/`:       "2023",
		`/x/FM26/graphics`:                "2026",
		`/x/fm2022/graphics`:              "2022",
		`/x/Football Manager 27/graphics`: "2027",
	}
	for p, want := range cases {
		v, ok := FromPath(p)
		if !ok || v.Year != want {
			t.Errorf("FromPath(%q) = %+v, %v; want year %s", p, v, ok, want)
		}
	}
	if _, ok := FromPath("/x/graphics/faces"); ok {
		t.Error("expected no version for a path without an FM folder")
	}
	if v, _ := FromPath("/x/Football Manager 27/"); v.IDPrefix != "r-" || v.Label != "FM27" {
		t.Errorf("unknown future year should inherit the newest prefix: %+v", v)
	}
	if v, _ := FromPath("/x/Football Manager 2021/"); v.IDPrefix != "" {
		t.Errorf("FM21 must not use the r- prefix: %+v", v)
	}
}

func TestLookup(t *testing.T) {
	if v, ok := Lookup("fm24"); !ok || v.Year != "2024" {
		t.Errorf("Lookup(fm24) = %+v %v", v, ok)
	}
	if _, ok := Lookup("2026"); ok {
		t.Error("FM26 is not a supported version and must not be listed")
	}
	if _, ok := Lookup("2025"); ok {
		t.Error("FM 2025 was never released and must not be listed")
	}
}
