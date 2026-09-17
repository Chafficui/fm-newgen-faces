package gui

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"fmnewgenfaces/gui/widgets"
	"fmnewgenfaces/internal/brand"
)

// teeWriter fans out log lines to the log file and to the on-screen LogView.
// The LogView's colour is picked from a "WARN "/"ERROR " marker in the line;
// everything else is treated as info.
type teeWriter struct {
	file *os.File
	view *widgets.LogView
}

func (w *teeWriter) Write(p []byte) (int, error) {
	if w.file != nil {
		_, _ = w.file.Write(p)
	}
	if w.view != nil {
		line := strings.TrimRight(string(p), "\n")
		level := widgets.LogInfo
		switch {
		case strings.Contains(line, "ERROR "):
			level = widgets.LogError
		case strings.Contains(line, "WARN "):
			level = widgets.LogWarn
		}
		w.view.Append(level, line)
	}
	return len(p), nil
}

// openLogger opens <dir>/<brand.LogFileName> once, in append mode, and
// returns a *log.Logger that writes to it. The returned file must be closed
// on shutdown. tee.view is nil until the LogView widget exists; setLogView
// attaches it later so early startup messages still reach the file.
func openLogger(dir string) (*log.Logger, *os.File, *teeWriter, error) {
	path := filepath.Join(dir, brand.LogFileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, nil, err
	}
	tee := &teeWriter{file: f}
	logger := log.New(tee, "", log.LstdFlags)
	return logger, f, tee, nil
}

// setLogView attaches the on-screen log panel so future lines also appear there.
func (a *App) setLogView(v *widgets.LogView) {
	a.logView = v
	if a.logTee != nil {
		a.logTee.view = v
	}
}

func (a *App) logf(format string, args ...any) {
	if a.logger == nil {
		return
	}
	a.logger.Print(fmt.Sprintf(format, args...))
}

func (a *App) warnf(format string, args ...any) {
	if a.logger == nil {
		return
	}
	a.logger.Print("WARN " + fmt.Sprintf(format, args...))
}

func (a *App) errorf(format string, args ...any) {
	if a.logger == nil {
		return
	}
	a.logger.Print("ERROR " + fmt.Sprintf(format, args...))
}

// logTail returns the last n lines of the log file (best-effort, empty on error).
func (a *App) logTail(n int) string {
	path := filepath.Join(a.store.Dir(), brand.LogFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
