package main

import (
	"path/filepath"
	"strings"
	"testing"

	"doomlauncher/internal/wadinfo"
)

func newTestLauncher(entries []Entry, status map[string]string) *launcher {
	cfg := defaultConfig()
	cfg.WadStatus = status
	l := &launcher{cfg: cfg}
	l.scan = scanResult{Entries: append([]Entry{{Name: "(none)"}}, entries...)}
	l.bucketize()
	return l
}

func TestBucketizeDefaultsToNotStarted(t *testing.T) {
	l := newTestLauncher([]Entry{
		{Name: "sunlust", Files: []string{"sunlust.wad"}},
		{Name: "Valiant", Files: []string{"Valiant.wad"}},
	}, map[string]string{})

	if got := len(l.buckets[1]); got != 2 { // index 1 == StatusTodo
		t.Fatalf("unknown wads should default to Not started, got %d in that bucket", got)
	}
	for _, i := range []int{0, 2, 3} {
		if len(l.buckets[i]) != 0 {
			t.Errorf("bucket %d should be empty, has %d", i, len(l.buckets[i]))
		}
	}
}

func TestBucketizeSortsByStatus(t *testing.T) {
	l := newTestLauncher([]Entry{
		{Name: "sunlust", Files: []string{"a.wad"}},
		{Name: "Valiant", Files: []string{"b.wad"}},
		{Name: "Sunder 2308", Files: []string{"c.wad"}},
		{Name: "chillax", Files: []string{"d.wad"}},
	}, map[string]string{
		"sunlust":     StatusProgress,
		"valiant":     StatusDone,
		"sunder 2308": StatusIgnore,
	})

	want := map[int]string{0: "sunlust", 1: "chillax", 2: "Valiant", 3: "Sunder 2308"}
	for i, name := range want {
		if len(l.buckets[i]) != 1 {
			t.Fatalf("bucket %d (%s): want 1 entry, got %d", i, statusOrder[i], len(l.buckets[i]))
		}
		if l.buckets[i][0].Name != name {
			t.Errorf("bucket %s: got %q want %q", statusOrder[i], l.buckets[i][0].Name, name)
		}
	}
}

// The "(none)" placeholder must never appear in a dropdown - a blank row in
// all four is what means "IWAD only".
func TestBucketsExcludePlaceholder(t *testing.T) {
	l := newTestLauncher([]Entry{{Name: "sunlust", Files: []string{"a.wad"}}}, map[string]string{})
	for i := range l.buckets {
		for _, e := range l.buckets[i] {
			if len(e.Files) == 0 {
				t.Fatalf("placeholder leaked into bucket %d", i)
			}
		}
	}
	if names := l.bucketNames(1); names[0] != "" {
		t.Errorf("first dropdown row should be blank, got %q", names[0])
	}
}

func TestStatusKeyIsCaseInsensitive(t *testing.T) {
	l := newTestLauncher([]Entry{{Name: "Eviternity II", Files: []string{"e.wad"}}},
		map[string]string{"eviternity ii": StatusDone})
	if got := l.statusOf("EVITERNITY II"); got != StatusDone {
		t.Errorf("got %q want %q", got, StatusDone)
	}
}

func TestUnknownStatusFallsBack(t *testing.T) {
	l := newTestLauncher([]Entry{{Name: "x", Files: []string{"x.wad"}}},
		map[string]string{"x": "banana"})
	if got := l.statusOf("x"); got != StatusTodo {
		t.Errorf("a corrupt status should fall back to Not started, got %q", got)
	}
}

func TestPromoteOnLaunchOnlyFromNotStarted(t *testing.T) {
	cases := []struct {
		name    string
		start   string
		changed bool
		want    string
	}{
		{"fresh wad gets started", StatusTodo, true, StatusProgress},
		{"already playing stays", StatusProgress, false, StatusProgress},
		{"completed is not reopened", StatusDone, false, StatusDone},
		{"ignored stays ignored", StatusIgnore, false, StatusIgnore},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := newTestLauncher(
				[]Entry{{Name: "sunlust", Files: []string{"a.wad"}}},
				map[string]string{"sunlust": c.start})
			if got := l.promoteOnLaunch("sunlust"); got != c.changed {
				t.Errorf("changed = %v, want %v", got, c.changed)
			}
			if got := l.statusOf("sunlust"); got != c.want {
				t.Errorf("status = %q, want %q", got, c.want)
			}
		})
	}
}

func TestPromoteOnLaunchIgnoresEmptyName(t *testing.T) {
	l := newTestLauncher(nil, map[string]string{})
	if l.promoteOnLaunch("") {
		t.Error("launching with no campaign should not record a status")
	}
	if len(l.cfg.WadStatus) != 0 {
		t.Errorf("WadStatus should stay empty, got %v", l.cfg.WadStatus)
	}
}

// Dropdown labels must be unique, or you get two identical rows and no way to
// tell them apart. STRG.wad and STRG_M28.wad both match STRG.txt, so this is a
// real case rather than a hypothetical one.
func TestDisplayNamesAreUnique(t *testing.T) {
	dir := doomDir(t)
	cfg := defaultConfig()
	l := &launcher{gameDir: dir, cfg: cfg}
	l.scan = scanDir(dir, cfg.Presets)
	l.nameEntries()

	seen := map[string]string{}
	for _, e := range l.scan.Entries {
		if len(e.Files) == 0 {
			continue
		}
		key := strings.ToLower(e.Display)
		if prev, dup := seen[key]; dup {
			t.Errorf("duplicate dropdown label %q used by both %q and %q", e.Display, prev, e.Name)
		}
		seen[key] = e.Name
		t.Logf("%-34s <- %s", e.Display, e.Name)
	}
}

// Base games should be selectable campaigns in their own right, carrying a
// status like any megawad, and must not add a -file argument.
func TestBaseGamesAreTrackableEntries(t *testing.T) {
	dir := doomDir(t)
	cfg := defaultConfig()
	l := &launcher{gameDir: dir, cfg: cfg}
	l.scan = scanDir(dir, cfg.Presets)
	l.nameEntries()
	l.bucketize()

	found := map[string]Entry{}
	for _, e := range l.scan.Entries {
		if e.IWAD != "" {
			found[e.Name] = e
		}
	}
	for _, want := range []string{"Doom II: Hell on Earth", "The Plutonia Experiment", "TNT: Evilution"} {
		e, ok := found[want]
		if !ok {
			t.Errorf("missing base-game entry %q", want)
			continue
		}
		if len(e.Files) != 0 {
			t.Errorf("%s must not load any -file, got %v", want, e.Files)
		}
		args := buildArgs(launchSpec{IWAD: e.IWAD, Files: e.Files})
		if got := commandLine("uzdoom.exe", args); got != "uzdoom.exe -iwad "+e.IWAD {
			t.Errorf("%s produced %q", want, got)
		}
	}

	inBuckets := 0
	for i := range l.buckets {
		for _, e := range l.buckets[i] {
			if e.IWAD != "" {
				inBuckets++
			}
		}
	}
	if inBuckets != len(found) {
		t.Errorf("%d base games exist but %d reached the dropdowns", len(found), inBuckets)
	}
}

// The old way of playing a base game - leave every dropdown blank and just
// pick the IWAD - must keep working alongside the new base-game entries.
func TestBlankSelectionStillPlaysTheBaseGame(t *testing.T) {
	args := buildArgs(launchSpec{IWAD: "PLUTONIA.WAD"}) // no campaign selected
	if got := commandLine("uzdoom.exe", args); got != "uzdoom.exe -iwad PLUTONIA.WAD" {
		t.Fatalf("blank selection produced %q", got)
	}
	// and it is identical to picking the base-game entry
	viaEntry := buildArgs(launchSpec{IWAD: "PLUTONIA.WAD", Files: nil})
	if commandLine("uzdoom.exe", viaEntry) != commandLine("uzdoom.exe", args) {
		t.Error("the two routes to a base game should produce the same command")
	}
}

func TestSupportsDoomMods(t *testing.T) {
	for _, c := range []struct {
		iwad string
		want bool
	}{
		{"DOOM2.WAD", true}, {"doom.wad", true}, {"PLUTONIA.WAD", true},
		{"TNT.WAD", true}, {"freedoom2.wad", true},
		{"HERETIC.WAD", false}, {"HEXEN.WAD", false}, {"STRIFE1.WAD", false},
		{"DOOM64.WAD", false}, {"CHEX3.WAD", false}, {"", false},
	} {
		if got := supportsDoomMods(c.iwad); got != c.want {
			t.Errorf("supportsDoomMods(%q) = %v, want %v", c.iwad, got, c.want)
		}
	}
}

// The mod must not be added to the command line for a base game it cannot
// work with, even if Brutal were somehow still ticked.
func TestNoModArgsForIncompatibleBaseGame(t *testing.T) {
	spec := launchSpec{IWAD: "HERETIC.WAD", Mod: "brutal22test6.pk3", Brutal: false}
	if got := commandLine("uzdoom.exe", buildArgs(spec)); got != "uzdoom.exe -iwad HERETIC.WAD" {
		t.Errorf("got %q", got)
	}
	// and it is still added for one that can
	spec = launchSpec{IWAD: "DOOM2.WAD", Mod: "brutal22test6.pk3", Brutal: true}
	want := "uzdoom.exe -iwad DOOM2.WAD -file brutal22test6.pk3"
	if got := commandLine("uzdoom.exe", buildArgs(spec)); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// Era inference is what stops SIGIL silently launching on Doom 2.
func TestEraInferenceOnRealWads(t *testing.T) {
	dir := doomDir(t)
	for _, c := range []struct{ file, want string }{
		{"sigil.wad", "doom1"},
		{"sigil2.wad", "doom1"},
		{"nerve.wad", "doom2"},
		{"masterlevels.wad", "doom2"},
		{"sunlust.wad", "doom2"},
		{"DOOM.WAD", "doom1"},
		{"DOOM2.WAD", "doom2"},
	} {
		info, err := wadinfo.Scan(filepath.Join(dir, c.file))
		if err != nil {
			t.Logf("skipping %s: %v", c.file, err)
			continue
		}
		if got := info.Era(); got != c.want {
			t.Errorf("%s: Era() = %q, want %q", c.file, got, c.want)
		}
	}
}

// nerve, sigil and the Master Levels are add-ons, not base games: listing them
// as IWADs would offer them in the base-game dropdown, where they cannot work.
func TestAddOnsAreNotBaseGames(t *testing.T) {
	for _, f := range []string{"nerve.wad", "hexdd.wad", "sigil.wad", "masterlevels.wad", "heretic_fr.wad", "heretic_ex.wad", "hexen_ex.wad"} {
		if knownIWADs[f] {
			t.Errorf("%s is an add-on PWAD but is listed as a base game", f)
		}
	}
	for _, f := range []string{"doom2.wad", "heretic.wad", "hexen.wad", "strife1.wad", "sve.wad"} {
		if !knownIWADs[f] {
			t.Errorf("%s is a base game but is not listed", f)
		}
	}
}

// Heretic and Hexen must land in the base-game dropdown, and their add-ons
// in the campaign dropdowns - not the other way round.
func TestHereticHexenClassification(t *testing.T) {
	dir := doomDir(t)
	r := scanDir(dir, nil)
	isIWAD := map[string]bool{}
	for _, w := range r.IWADs {
		isIWAD[strings.ToLower(w)] = true
	}
	for _, w := range []string{"heretic.wad", "hexen.wad"} {
		if !isIWAD[w] {
			t.Errorf("%s should be offered as a base game", w)
		}
	}
	for _, w := range []string{"hexdd.wad", "heretic_fr.wad", "hexen_vog.wad"} {
		if isIWAD[w] {
			t.Errorf("%s is an add-on but was offered as a base game", w)
		}
	}
}
