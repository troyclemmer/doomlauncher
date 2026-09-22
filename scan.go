package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// knownIWADs are base games: they go after -iwad, never after -file.
var knownIWADs = map[string]bool{
	"doom.wad": true, "doom1.wad": true, "doom2.wad": true,
	"plutonia.wad": true, "tnt.wad": true, "doom64.wad": true,
	"heretic.wad": true, "hexen.wad": true, "hexdd.wad": true,
	"strife1.wad": true, "chex.wad": true, "chex3.wad": true,
	"freedoom1.wad": true, "freedoom2.wad": true, "freedm.wad": true,
	"nerve.wad": true, "id1.wad": true,
}

// engineFiles ship with GZDoom/UZDoom and are auto-loaded; they are not campaigns.
var engineFiles = map[string]bool{
	"uzdoom.pk3": true, "gzdoom.pk3": true, "zdoom.pk3": true,
	"brightmaps.pk3": true, "lights.pk3": true, "obituaries.pk3": true,
	"game_support.pk3": true, "game_widescreen_gfx.pk3": true,
	"game_support_data.pk3": true, "soundfonts.pk3": true, "licenses.zip": true,
}

var loadableExts = map[string]bool{
	".wad": true, ".pk3": true, ".pk7": true, ".ipk3": true,
}

// Entry is one selectable item in the campaign dropdown.
type Entry struct {
	Name  string
	Files []string
}

type scanResult struct {
	IWADs   []string
	Entries []Entry
	Mods    []string
}

// scanDir builds the dropdown contents from what is actually on disk,
// merged with the multi-file presets defined in the config.
func scanDir(dir string, presets []Preset) scanResult {
	res := scanResult{Entries: []Entry{{Name: "(none — IWAD only)"}}}

	items, err := os.ReadDir(dir)
	if err != nil {
		return res
	}

	// Files already spoken for by a preset must not also show up standalone.
	claimed := map[string]bool{}
	for _, p := range presets {
		for _, f := range p.Files {
			claimed[strings.ToLower(filepath.Base(f))] = true
		}
	}

	// A .deh sitting next to a .wad of the same name is part of that mod.
	dehs := map[string]string{}
	for _, it := range items {
		if it.IsDir() {
			continue
		}
		n := it.Name()
		if strings.EqualFold(filepath.Ext(n), ".deh") {
			dehs[strings.ToLower(strings.TrimSuffix(n, filepath.Ext(n)))] = n
		}
	}

	var loose []Entry
	for _, it := range items {
		if it.IsDir() {
			continue
		}
		name := it.Name()
		lower := strings.ToLower(name)
		ext := strings.ToLower(filepath.Ext(name))

		if knownIWADs[lower] {
			res.IWADs = append(res.IWADs, name)
			continue
		}
		if !loadableExts[ext] || engineFiles[lower] {
			continue
		}
		if strings.Contains(lower, "brutal") {
			res.Mods = append(res.Mods, name)
			continue
		}
		if claimed[lower] {
			continue
		}

		files := []string{name}
		if deh, ok := dehs[strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))]; ok {
			files = append(files, deh)
		}
		loose = append(loose, Entry{
			Name:  strings.TrimSpace(strings.TrimSuffix(name, filepath.Ext(name))),
			Files: files,
		})
	}

	for _, p := range presets {
		loose = append(loose, Entry{Name: p.Name, Files: p.Files})
	}

	sort.Slice(loose, func(i, j int) bool {
		return strings.ToLower(loose[i].Name) < strings.ToLower(loose[j].Name)
	})
	sort.Slice(res.IWADs, func(i, j int) bool {
		return strings.ToLower(res.IWADs[i]) < strings.ToLower(res.IWADs[j])
	})
	sort.Slice(res.Mods, func(i, j int) bool {
		return strings.ToLower(res.Mods[i]) < strings.ToLower(res.Mods[j])
	})

	res.Entries = append(res.Entries, loose...)
	return res
}

// splitArgs splits the free-form extra-args field the way a shell would,
// so things like -warp 12 or +map "MAP07" survive intact.
func splitArgs(s string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	started := false

	flush := func() {
		if started {
			out = append(out, cur.String())
			cur.Reset()
			started = false
		}
	}

	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
			started = true
		case r == ' ' || r == '\t':
			flush()
		default:
			cur.WriteRune(r)
			started = true
		}
	}
	flush()
	return out
}

// quoteForDisplay renders the command preview the way you would type it.
func quoteForDisplay(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, " \t") {
		return `"` + s + `"`
	}
	return s
}
