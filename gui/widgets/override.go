package widgets

import (
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func grayText(text string) fyne.CanvasObject {
	return widget.NewRichText(&widget.TextSegment{
		Text:  text,
		Style: widget.RichTextStyle{ColorName: theme.ColorNameDisabled},
	})
}

func newOverrideEditor(cfg OverrideEditorConfig) *OverrideEditor {
	e := &OverrideEditor{cfg: cfg}
	e.list = container.NewVBox()

	addBtn := widget.NewButtonWithIcon(T("widgets.override.add"), theme.ContentAddIcon(), func() { e.showAddDialog() })
	importBtn := widget.NewButton(T("widgets.override.import"), func() { e.showImportDialog() })
	exportBtn := widget.NewButton(T("widgets.override.export"), func() { e.showExportDialog() })
	toolbar := container.NewHBox(addBtn, importBtn, exportBtn)

	scroll := container.NewVScroll(e.list)
	e.CanvasObject = container.NewBorder(toolbar, nil, nil, nil, scroll)
	e.refresh()
	return e
}

func (e *OverrideEditor) buildRow(code, val string) fyne.CanvasObject {
	def := ""
	if e.cfg.DefaultFor != nil {
		def = e.cfg.DefaultFor(code)
	}

	codeLabel := widget.NewLabelWithStyle(code, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	valLabel := widget.NewLabel(val)

	var noteObj fyne.CanvasObject
	if def != "" {
		noteObj = grayText(T("widgets.override.default", def))
	} else {
		noteObj = grayText(T("widgets.override.unknown_code_note"))
	}

	info := container.NewVBox(container.NewHBox(codeLabel, widget.NewLabel("→"), valLabel), noteObj)

	codeCopy := code
	delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() { e.deleteOverride(codeCopy) })

	return container.NewBorder(nil, nil, nil, delBtn, info)
}

func (e *OverrideEditor) refresh() {
	overrides := map[string]string{}
	if e.cfg.Get != nil {
		overrides = e.cfg.Get()
	}

	codes := make([]string, 0, len(overrides))
	for c := range overrides {
		codes = append(codes, c)
	}
	sort.Strings(codes)

	objects := make([]fyne.CanvasObject, 0, len(codes)*2)
	for _, c := range codes {
		objects = append(objects, e.buildRow(c, overrides[c]), widget.NewSeparator())
	}
	if len(codes) == 0 {
		objects = append(objects, widget.NewLabel(T("widgets.override.empty")))
	}

	e.list.Objects = objects
	e.list.Refresh()
}

func (e *OverrideEditor) deleteOverride(code string) {
	overrides := map[string]string{}
	if e.cfg.Get != nil {
		overrides = e.cfg.Get()
	}
	updated := make(map[string]string, len(overrides))
	for k, v := range overrides {
		if k != code {
			updated[k] = v
		}
	}
	if e.cfg.Set != nil {
		e.cfg.Set(updated)
	}
	e.refresh()
}

func (e *OverrideEditor) showAddDialog() {
	known := make(map[string]bool, len(e.cfg.KnownCodes))
	for _, c := range e.cfg.KnownCodes {
		known[strings.ToUpper(c)] = true
	}

	entry := widget.NewSelectEntry(e.cfg.KnownCodes)
	entry.SetPlaceHolder(T("widgets.override.code_placeholder"))

	warn := widget.NewLabel(T("widgets.override.unknown_code"))
	warn.Wrapping = fyne.TextWrapWord
	warn.Hide()

	entry.OnChanged = func(s string) {
		up := strings.ToUpper(s)
		if up != s {
			entry.SetText(up)
			return
		}
		trimmed := strings.TrimSpace(up)
		if trimmed != "" && !known[trimmed] {
			warn.Show()
		} else {
			warn.Hide()
		}
	}

	sel := widget.NewSelect(e.cfg.EthnicNames, nil)

	form := widget.NewForm(
		widget.NewFormItem(T("widgets.override.code"), entry),
		widget.NewFormItem(T("widgets.override.ethnic"), sel),
	)
	content := container.NewVBox(form, warn)

	dlg := dialog.NewCustomConfirm(T("widgets.override.add_title"), T("widgets.override.save"), T("widgets.override.cancel"), content, func(ok bool) {
		if !ok {
			return
		}
		code := strings.ToUpper(strings.TrimSpace(entry.Text))
		if code == "" || sel.Selected == "" {
			return
		}
		overrides := map[string]string{}
		if e.cfg.Get != nil {
			overrides = e.cfg.Get()
		}
		updated := make(map[string]string, len(overrides)+1)
		for k, v := range overrides {
			updated[k] = v
		}
		updated[code] = sel.Selected
		if e.cfg.Set != nil {
			e.cfg.Set(updated)
		}
		e.refresh()
	}, e.cfg.Window)
	dlg.Resize(fyne.NewSize(420, 260))
	dlg.Show()
}

func (e *OverrideEditor) showImportDialog() {
	entry := widget.NewMultiLineEntry()
	entry.SetPlaceHolder(T("widgets.override.import_placeholder"))
	entry.Wrapping = fyne.TextWrapWord

	scroll := container.NewVScroll(entry)
	scroll.SetMinSize(fyne.NewSize(420, 260))

	dlg := dialog.NewCustomConfirm(T("widgets.override.import_title"), T("widgets.override.import"), T("widgets.override.cancel"), scroll, func(ok bool) {
		if !ok {
			return
		}
		if e.cfg.Import == nil {
			return
		}
		if err := e.cfg.Import(entry.Text); err != nil {
			dialog.ShowError(err, e.cfg.Window)
			return
		}
		e.refresh()
	}, e.cfg.Window)
	dlg.Resize(fyne.NewSize(460, 380))
	dlg.Show()
}

func (e *OverrideEditor) showExportDialog() {
	text := ""
	if e.cfg.Export != nil {
		text = e.cfg.Export()
	}

	entry := widget.NewMultiLineEntry()
	entry.Wrapping = fyne.TextWrapWord
	entry.SetText(text)

	scroll := container.NewVScroll(entry)
	scroll.SetMinSize(fyne.NewSize(420, 260))

	dlg := dialog.NewCustom(T("widgets.override.export_title"), T("widgets.override.close"), scroll, e.cfg.Window)
	dlg.Resize(fyne.NewSize(460, 380))
	dlg.Show()
}
