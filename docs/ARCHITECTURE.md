# FM NewGen Faces – architecture (v2)

v2 is a rewrite of the 1.x ("Jaqen") tool around a headless core that both the
GUI and the CLI drive. Nothing in `internal/core` imports Fyne.

```
main.go                      → cmd.Execute()
cmd/                         cobra commands: (default) gui, assign, check, detect, restore, profiles, version
gui/                         Fyne app shell: window, profile bar, checklist, settings, run flow
gui/widgets/                 reusable components (checklist, pack table, preview, summary, review table, …)
assets/                      embedded FM view/filter files + icon
internal/brand/              product names, URLs, Version (ldflags)
internal/i18n/               T()/N() over embedded locales/en.json, de.json
internal/bugreport/          redacted bug-report markdown
internal/core/ethnic/        14 ethnic groups, nation table, per-run Resolver with overrides
internal/core/fmversion/     FM release table (Year, Label, IDPrefix) + FromPath
internal/core/rtf/           tolerant parser of the FM "SCRIPT FACES" export
internal/core/facepack/      pack scan (per-folder counts/warnings), Pool with exclusion + fair random
internal/core/fmconfig/      config.xml load/save (foreign records preserved, atomic write, backups)
internal/core/assign/        Plan (dry run) → Apply → Reroll
internal/core/fminstall/     bounded FM install detection, embedded view/filter distribution
internal/core/profile/       JSON profiles, legacy migration, app State, debounced Autosaver
internal/core/update/        GitHub latest-release check
```

## Pipeline

1. `facepack.Scan(packDir)` → `Pack` (never fails on a missing folder; reports it).
2. `ethnic.NewResolver(profile.Overrides)` → `Resolver` (no global state).
3. `rtf.Parse(rtfPath, resolver)` → `Result` (players, unmapped nations, malformed rows, duplicates).
4. `fmconfig.LoadOrNew(configXML, version)` → `Config` (foreign records kept verbatim).
5. `assign.Build(result, config, pack, options)` → `Plan` (dry run shown to the user).
6. `assign.Apply(plan, config, pack, progress)` → `Result` (config mutated in memory).
7. `config.Save(SaveOptions{BackupDir})` → atomic write + backup.

## Rules that the v1 code got wrong (do not regress)

- Never key a mapping on an empty ID; foreign `<record>`s pass through untouched.
- Random draw is `rand.Intn(len)`; every image reachable.
- Only `.png/.jpg/.jpeg` are images; dotfiles/`Thumbs.db` are ignored and reported.
- Relative image paths are computed from the config.xml **directory**, forward slashes.
- "No duplicates" excludes images already mapped in the config before drawing.
- One unknown nation or malformed row never aborts the run.
- Overrides are per run; nation keys are upper-cased.
- Install detection never recurses through game libraries; scan once, off the UI thread.
- Everything the UI does on a path change is debounced and runs off the UI thread.
- Profile names are validated and slugified; saves are atomic and serialised.
- The old `~/.jaqen` profiles are migrated once into the new config dir.
