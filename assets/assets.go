// Package assets embeds files that ship inside the binary: the Football
// Manager view and filter files that are distributed to each install, and the
// application icon.
package assets

import "embed"

// FS holds views/ and filters/ (FM .fmf files) and icon.png.
//
//go:embed views filters icon.png
var FS embed.FS

// Icon returns the embedded application icon (a 256x256 PNG).
func Icon() []byte {
	data, err := FS.ReadFile("icon.png")
	if err != nil {
		// icon.png is embedded at build time; a missing file is a build bug,
		// not a runtime condition callers should have to handle.
		panic("assets: icon.png missing from embedded FS: " + err.Error())
	}
	return data
}
