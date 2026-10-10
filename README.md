# Doom Launcher

A small native Windows launcher for GZDoom-family source ports, written in Go.

It replaces the pile of `.bat` files that accumulate around a Doom folder. Point it
at a directory of WADs and it works out what's there: base games, campaigns,
mods. Pick a campaign, pick a base game, hit Launch. It remembers what you chose.

No installer, no dependencies, no runtime. One `.exe` you drop in your Doom folder.

## What it does

**Auto-detects your folder.** IWADs (DOOM2.WAD, PLUTONIA.WAD, TNT.WAD…) go in one
dropdown, everything else in another. Engine resource files (`gzdoom.pk3`,
`brightmaps.pk3`, `lights.pk3` and friends) are filtered out so they don't clutter
the campaign list.

**Remembers everything**, including the base game *per campaign*. Play a Doom 1
megawad once with DOOM.WAD and picking it again switches the IWAD back for you.

**Multi-file campaigns.** Some releases need several files loaded in a fixed order —
a maps wad, a music wad, a compatibility patch. A preset editor lets you bundle
them with explicit Move up / Move down ordering, because load order matters and
file pickers don't preserve it.

**A mod checkbox.** Auto-detects Brutal Doom (`brutal*.pk3`) and appends it last,
matching the usual convention. The skill dropdown switches between 5 and 8 entries
depending on whether it's loaded, because Brutal Doom replaces the stock skill
definitions with eight of its own.

**Live command-line preview**, so you can see exactly what will run before it runs.

**WAD info panel** — map count, map range, file size, monster totals per difficulty,
plus title/author/date pulled from the accompanying `.txt` if it can find one.
Scanning reads only the WAD directory and `THINGS` lumps, so a 280 MB megawad costs
a few milliseconds.

**Friendly names.** Dropdowns show the title from each wad's readme rather than
its filename — *Struggle - Antaresian Legacy*, not `STRG`. Presets keep the name
you gave them, marked with a trailing `*`. Where two wads share a readme (STRG
and STRG_M28 both match STRG.txt) the duplicate is qualified with its filename,
so no two rows ever read the same.

**Progress tracking.** Every campaign carries a status — *in progress*, *not
started*, *completed* or *ignored* — and the Launch tab has one dropdown per
status, in that order, so what you are playing now is at the top and finished or
shelved wads sink to the bottom. The four are mutually exclusive: picking a
campaign in one blanks the others, and blank in all four means "IWAD only".

Anything the launcher has not seen defaults to *not started*. Launching a
campaign promotes it to *in progress* — but only from *not started*, so marking
something completed sticks even if you replay it. The **Progress tab** lists
everything in a table; select one or more rows and reassign them.

**History tab** logs every launch with its full command line. Double-click a row to
restore those settings.

**Readme tab** shows the wad's text file. Name matching is fuzzy, because it has to
be: `deathless.wad` ships `deathless1.1.txt`, `Sunder 2308.wad` ships
`SunderReadMeV2.TXT`.

**Cheatsheet tab** — a searchable console-command reference, filtering on
descriptions as well as names. Every command was verified to exist in the engine
binary before being listed.

## Install

Download or build `DoomLauncher.exe`, drop it in your Doom folder next to the
engine, and run it. It writes `launcher.json` and `history.jsonl` alongside itself,
falling back to `%APPDATA%\DoomLauncher\` if the folder is read-only.

Configuration is that one JSON file. Edit it by hand if you prefer — the launcher
reads it back.

## Build

Needs Go 1.22+. No C compiler: [lxn/walk](https://github.com/lxn/walk) uses native
Win32 controls through syscalls, so `CGO_ENABLED=0` is fine.

```
go generate ./...          # optional: builds the icon, see below
go build -ldflags "-H=windowsgui -s -w" -o DoomLauncher.exe .
```

`-H=windowsgui` suppresses the console window; `-s -w` strips debug info.

### The icon

The launcher's icon is the Doomguy status-bar face, extracted from **your own**
`DOOM2.WAD` at build time. It isn't committed here, because it's id Software's
artwork and this repository doesn't redistribute it.

```
go run ./tools/wadicon "C:\path\to\DOOM2.WAD" STFST01 doom.ico
go run github.com/akavel/rsrc@v0.10.2 -manifest app.manifest -ico doom.ico -o rsrc.syso
go build -ldflags "-H=windowsgui -s -w" -o DoomLauncher.exe .
```

Any face works — `STFST31` is the grimacing low-health one. Pass a different lump
name as the second argument.

**The manifest step is not optional.** walk requires a ComCtl32 v6 manifest or it
fails to initialise common controls at startup. If you skip `rsrc`, you get an exe
that won't open a window. To build without an icon, run `rsrc` with just
`-manifest app.manifest`.

## Tools

Three command-line utilities live under `tools/`.

### wadstat

Monster counts per difficulty, for any WAD or PK3.

```
go build -o wadstat.exe ./tools/wadstat

wadstat.exe sunlust.wad          # every map, totals
wadstat.exe sunlust.wad MAP28    # one map, broken down by monster type
```

Drag WAD files — or a folder of them — onto the exe and it works too. It detects
whether it owns its console (via `GetConsoleProcessList`) and pauses before closing
when launched from Explorer, so results don't flash past.

Reports three columns, because a classic-format map stores exactly three skill bits
per thing: skills 1–2 share one, skill 3 has its own, skills 4–5 share one. Five (or
eight) difficulty levels can only ever produce three distinct monster line-ups.

### wadicon

Builds a `.ico` from a sprite inside an IWAD — decodes Doom's column-major picture
format, applies `PLAYPAL`, and renders seven sizes from 16 to 256 px onto a dark
rounded backdrop.

### mkicon

Extracts an existing icon out of any Windows PE binary by walking its `.rsrc`
resource tree. Written to lift the engine's own icon before deciding the Doomguy
face was better.

## Layout

```
main.go               window, tabs, event wiring
status.go             per-campaign play status and the four Launch dropdowns
config.go             launcher.json load/save
scan.go               folder scanning, IWAD/mod/campaign classification
args.go               command-line construction
preset.go             multi-file campaign editor dialog
history.go            launch log
cheats.go             console command reference
wadbox.go             WAD info panel, background scanning and caching
internal/wadinfo/     WAD and PK3 parsing, readme discovery
tools/                wadstat, wadicon, mkicon
```

`internal/wadinfo` is shared by the launcher and `wadstat`, so there's one parser
rather than two that drift apart.

## Notes on the WAD parsing

A few things that aren't obvious and cost me time:

**A map marker is any lump followed by `THINGS` or `TEXTMAP`** — not a lump matching
`MAP\d\d|E\dM\d`. Map names are arbitrary; Extermination Day's are `EDAY01`–`EDAY44`.
Pattern-matching names silently misses whole megawads.

**Map format is per-map, not per-WAD.** A single file can mix classic and UDMF maps,
so format has to be detected inside each map's lump group.

**PK3 maps aren't always under `maps/`.** They can be whole WADs embedded anywhere in
the archive.

**In the `SECTORS` lump, tag is the last field and special is second-to-last.**
Swapping them makes every sector look untagged, and tagged-sector lookups return
nothing at all.

## Testing

```
go test ./...
```

Tests read real WADs rather than fixtures. They look for a Doom folder and skip
cleanly if there isn't one — set `DOOM_DIR` to point at yours.

Tests that *write* use `t.TempDir()` and never a real Doom folder. An early
version of the config round-trip test wrote into the live directory and deleted
the user's `launcher.json` on the way out; `TestConfigRoundTrip` now asserts that
its path cannot escape the temp dir.

## Why walk

Fyne and most Go GUI toolkits need cgo, and therefore a C compiler. walk is pure Go
over the Win32 API: it builds with nothing but the Go toolchain, produces a single
~6 MB exe with no runtime dependency, and looks like a native Windows application
because it is one. Windows-only, which for a Doom launcher is fine.

## License

MIT. Doom, Brutal Doom and any game assets referenced here belong to their
respective owners; none are included in this repository.
