package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// variantTheme wraps a fyne.Theme and forces a specific light/dark variant,
// ignoring the OS preference.
type variantTheme struct {
	fyne.Theme
	variant fyne.ThemeVariant
}

// newTheme returns a theme for the given setting ("system", "light", "dark").
// Unknown values (including "system"/"") delegate to the default theme,
// which follows the OS preference.
func newTheme(setting string) fyne.Theme {
	base := theme.DefaultTheme()
	switch setting {
	case "light":
		return &variantTheme{Theme: base, variant: theme.VariantLight}
	case "dark":
		return &variantTheme{Theme: base, variant: theme.VariantDark}
	default:
		return base
	}
}

// Color forces the wrapped theme's variant regardless of what the caller asks for.
func (t *variantTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return t.Theme.Color(name, t.variant)
}
