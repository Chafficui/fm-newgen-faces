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

// newTheme returns a theme for the given setting ("light", "dark", "system").
// Light is the default for a fresh install (an empty setting); "system"
// follows the OS preference.
func newTheme(setting string) fyne.Theme {
	base := theme.DefaultTheme()
	switch setting {
	case "dark":
		return &variantTheme{Theme: base, variant: theme.VariantDark}
	case "system":
		return base
	default:
		return &variantTheme{Theme: base, variant: theme.VariantLight}
	}
}

// Color forces the wrapped theme's variant regardless of what the caller asks for.
func (t *variantTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return t.Theme.Color(name, t.variant)
}
