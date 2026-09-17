package gui

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	"github.com/sqweek/dialog"
)

// openURL opens an http(s) URL in the system browser via fyne.App.OpenURL.
func (a *App) openURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("openURL: unsupported scheme %q", u.Scheme)
	}
	return a.fyneApp.OpenURL(u)
}

// openFolder opens path in the platform's file manager.
func openFolder(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// pickFolder shows a native "choose folder" dialog starting at start.
// Returns "" when cancelled.
func pickFolder(start string) string {
	d := dialog.Directory()
	if start != "" {
		d = d.SetStartDir(start)
	}
	path, err := d.Browse()
	if err != nil {
		return ""
	}
	return path
}

// pickFile shows a native "choose file" dialog. ext is a single extension
// without the dot (e.g. "rtf", "xml"). Returns "" when cancelled.
func pickFile(title, ext string) string {
	b := dialog.File().Title(title)
	if ext != "" {
		b = b.Filter(strings.ToUpper(ext)+" files", ext)
	}
	path, err := b.Load()
	if err != nil {
		return ""
	}
	return path
}
