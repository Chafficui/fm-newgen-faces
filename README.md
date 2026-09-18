<div align="center">

# FM NewGen Faces

**Give every newgen in your Football Manager save a real face – in one click.**

Free, open source, cross-platform (Windows · macOS · Linux). Works with any newgen face pack that uses the standard 14 ethnic folders (NewGAN-style packs).

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/chafficui/fm-newgen-faces?label=download)](https://github.com/chafficui/fm-newgen-faces/releases/latest)
[![CI](https://github.com/chafficui/fm-newgen-faces/actions/workflows/ci.yml/badge.svg)](https://github.com/chafficui/fm-newgen-faces/actions/workflows/ci.yml)

[Download](https://github.com/chafficui/fm-newgen-faces/releases/latest) · [Tutorial video](https://youtu.be/aHnrpfH--ic) · [Report a problem](https://github.com/chafficui/fm-newgen-faces/issues)

</div>

## What it does

Football Manager generates "newgens" (regens) with blank faces. FM NewGen Faces reads the list of newgens from your save, picks a matching portrait for each one from your face pack based on nationality and ethnicity, and writes the `config.xml` that FM uses to show the pictures. Run it once per season (or whenever you like) and every new youth intake gets faces too.

**Version 2** is a rewrite of the tool formerly known as *Jaqen NewGen Tool*, with a guided setup, a dry-run preview, backups with undo, a face review table, a headless CLI and a long list of fixes. Your old profiles are migrated automatically on first start.

## Quick start

1. **Install a face pack.** Extract it anywhere under your FM graphics folder, e.g.
   `Documents/Sports Interactive/Football Manager 2024/graphics/newgen-faces/`.
   The folder must contain the 14 ethnic subfolders (`African`, `Asian`, `Caucasian`, …).
2. **Start FM NewGen Faces.** It finds your FM installations, installs the required view and filter into FM, and creates a profile per installation. The setup checklist tells you exactly what is still missing.
3. **Export the newgen list from FM** (once per run):
   Scouting → Players in Range → import the view **SCRIPT FACES player search** → apply the filter **is newgen search filter** → select all (Ctrl+A) → print to text file (Ctrl+P) → save as `newgen.rtf` in your face pack folder. The app notices the new file by itself.
4. **Click Preview**, check the numbers, then **Assign faces**. A backup of the previous `config.xml` is kept; *Undo* restores it.
5. **In FM:** Preferences → Interface → *Clear Cache* and *Reload Skin* (or restart FM). Done.

## Features

- **Setup checklist** – face pack, config.xml, RTF export and FM version are checked live, each with a fix-it action.
- **Face pack validation** – per-group image counts, missing/empty folders, ignored files and nested folders are shown before you run.
- **Dry-run preview** – how many players get a new face, how many are preserved, supply vs demand per group.
- **Unmapped nations resolver** – unknown nation codes are listed with a dropdown; one click saves the override and continues. A single unknown code never blocks the run.
- **Backups and undo** – every write keeps a timestamped copy of `config.xml`.
- **Review table** – see every assigned face with a thumbnail, search, and *Reroll* a face you do not like.
- **Preserve / incremental mode** – existing faces are kept; only new newgens get one. The summary tells you how many.
- **No duplicates mode** – images already in use are excluded before drawing.
- **Profiles per FM installation** with auto-selection, validated names and atomic saves.
- **Drag and drop** the RTF or the face pack folder onto the window; an **RTF watcher** picks up new exports.
- **Headless CLI** for scripting: `fm-newgen-faces assign --profile "FM 2024"`, `check`, `detect`, `restore`.
- **First-run wizard**, light/dark theme, English/German UI, keyboard shortcuts, update check, redacted bug reports.

## Command line

```
fm-newgen-faces                 # launches the GUI
fm-newgen-faces check           # validate the current profile's setup
fm-newgen-faces assign --profile "FM 2024" [--dry-run] [--no-preserve] [--no-duplicates]
fm-newgen-faces assign --pack ./faces --rtf ./newgen.rtf --config ./faces/config.xml --fm 2024
fm-newgen-faces detect          # list Football Manager installations found
fm-newgen-faces restore --list  # list config.xml backups; --to <backup> restores one
fm-newgen-faces profiles        # list profiles
fm-newgen-faces version
```

## Ethnic groups and overrides

Nations are mapped to the 14 face-pack groups (`African`, `Asian`, `Caucasian`, `Central European`, `EECA`, `Italmed`, `MENA`, `MESA`, `SAMed`, `Scandinavian`, `Seasian`, `South American`, `SpanMed`, `YugoGreek`) using FM's own ethnicity value plus the player's nations. You can override any nation in *Settings → Overrides*, or import/export a block like:

```toml
[mapping_override]
AFG = "MESA"
ENG = "Caucasian"
```

## Supported Football Manager versions

FM20 – FM24 and FM26. FM24 and later use the `r-<id>` portrait naming; earlier versions use the bare id. If a future release changes the naming, it is a one-line change in `internal/core/fmversion`.

## Where files live

| What | Location |
|---|---|
| Profiles, app state, log, backups | Linux `~/.config/fm-newgen-faces`, macOS `~/Library/Application Support/fm-newgen-faces`, Windows `%AppData%\fm-newgen-faces` |
| Views / filters installed into FM | `<FM user folder>/views`, `<FM user folder>/filters` |
| Legacy 1.x profiles | migrated once from `~/.jaqen` (or the OS equivalent) |

## Build from source

Requires Go 1.24+ and, on Linux, the Fyne build dependencies (`libgl1-mesa-dev libx11-dev libxcursor-dev libxrandr-dev libxinerama-dev libxi-dev libxxf86vm-dev libgtk-3-dev pkg-config`).

```bash
git clone https://github.com/chafficui/fm-newgen-faces.git
cd fm-newgen-faces
make build     # → ./fm-newgen-faces
make check     # gofmt, vet, tests
```

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for how the code is organised.

## Credits

- Original tool: [Jaqen](https://github.com/imfulee/jaqen) by [@imfulee](https://github.com/imfulee)
- Views and filters: [NewGAN-Manager](https://github.com/Maradonna90/NewGAN-Manager) by [@Maradonna90](https://github.com/Maradonna90)

Football Manager is a trademark of Sports Interactive / SEGA. This is an independent community tool.

## License

GPL v3 – see [LICENSE](LICENSE).
