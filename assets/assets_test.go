package assets

import (
	"bytes"
	"image/png"
	"testing"
)

func TestIcon(t *testing.T) {
	data := Icon()
	if len(data) == 0 {
		t.Fatal("Icon() returned no data")
	}
	if len(data) > 30*1024 {
		t.Errorf("Icon() is %d bytes, want <= 30KB", len(data))
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Icon() is not a valid PNG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 256 || bounds.Dy() != 256 {
		t.Errorf("Icon() size = %dx%d, want 256x256", bounds.Dx(), bounds.Dy())
	}
}

func TestFSHasViewsAndFilters(t *testing.T) {
	if _, err := FS.ReadDir("views"); err != nil {
		t.Errorf("FS missing views/: %v", err)
	}
	if _, err := FS.ReadDir("filters"); err != nil {
		t.Errorf("FS missing filters/: %v", err)
	}
	viewsEntries, _ := FS.ReadDir("views")
	if len(viewsEntries) == 0 {
		t.Error("views/ is empty")
	}
	filtersEntries, _ := FS.ReadDir("filters")
	if len(filtersEntries) == 0 {
		t.Error("filters/ is empty")
	}
}
