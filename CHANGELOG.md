# Changelog

## 2.0.0 – 2026-09-18

Rebranded from *Jaqen NewGen Tool* to **FM NewGen Faces** and rewritten around a headless core.

### Added
- Live setup checklist with fix-it actions; face pack validation panel.
- Dry-run preview, post-run summary with next steps in FM.
- Timestamped config.xml backups with one-click undo / CLI restore.
- Unmapped nations resolver (inline overrides), validated override editor with TOML import/export.
- Review table with thumbnails, search and per-player reroll.
- First-run wizard, drag and drop, RTF file watcher, keyboard shortcuts, window state memory, light/dark theme.
- English and German UI.
- Headless CLI: `assign`, `check`, `detect`, `restore`, `profiles`, `version`.
- Update check against GitHub releases; app version baked into builds and bug reports.
- Views and filters are embedded in the binary and installed into every FM installation.
- Per-OS release workflow and CI.
- Automatic migration of 1.x profiles.

### Fixed
- Existing config.xml records that were not newgen portraits collapsed into one entry and were lost.
- Concurrent map access from auto-save could crash the app.
- config.xml auto-generation and view/filter distribution never ran on a fresh install.
- "Allow duplicates" off did not exclude already-assigned images.
- Players silently skipped when a group ran out of images while "Success" was shown.
- Non-image files (Thumbs.db, .DS_Store) were treated as faces.
- Startup recursed through the whole Steam library, twice, blocking the window.
- Every keystroke in the folder field walked the face pack and leaked a log file handle.
- One unknown nation or malformed RTF line aborted the whole run.
- Overrides leaked across profiles and lowercase keys were ignored.
- Relative image paths were computed from the XML file instead of its directory; backslashes on Windows.
- The last image of every folder could never be picked.
- FM 2025/2026 installs were detected but silently ignored.
- No backup before overwriting config.xml; file handle not closed; missing XML declaration.
- Bug reports leaked the OS username.
- Profile names were not validated.

### Removed
- `format` command (profiles are JSON now; `jaqen.toml` is no longer used).
