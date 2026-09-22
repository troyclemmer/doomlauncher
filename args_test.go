package main

import (
	"fmt"
	"testing"
)

// The Extermination Day entry must reproduce the hand-written .bat exactly:
//
//	uzdoom.exe -file "EdayTest001.pk3" "EdayMusic001.wad" "EdayMapInfoPatchV2.wad" "brutal22test6.pk3"
func TestExterminationDayOrdering(t *testing.T) {
	dir := doomDir(t)
	cfg := defaultConfig()
	r := scanDir(dir, cfg.Presets)

	var eday Entry
	for _, e := range r.Entries {
		if e.Name == "Extermination Day" {
			eday = e
		}
	}
	if eday.Name == "" {
		t.Fatal("Extermination Day entry missing")
	}

	args := buildArgs(launchSpec{
		IWAD:   "DOOM2.WAD",
		Files:  eday.Files,
		Mod:    "brutal22test6.pk3",
		Brutal: true,
	})
	fmt.Println("launcher produces:", commandLine("uzdoom.exe", args))

	want := []string{
		"-iwad", "DOOM2.WAD",
		"-file",
		"EdayTest001.pk3",
		"EdayMusic001.wad",
		"EdayMapInfoPatchV2.wad",
		"brutal22test6.pk3",
	}
	if len(args) != len(want) {
		t.Fatalf("got %q\nwant %q", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("position %d: got %q want %q\nfull: %q", i, args[i], want[i], args)
		}
	}
}

// Unchecking Brutal Doom must drop only the mod, leaving campaign order intact.
func TestOrderingWithoutMod(t *testing.T) {
	args := buildArgs(launchSpec{
		IWAD:   "DOOM2.WAD",
		Files:  []string{"a.pk3", "b.wad", "c.wad"},
		Mod:    "brutal22test6.pk3",
		Brutal: false,
	})
	want := "uzdoom.exe -iwad DOOM2.WAD -file a.pk3 b.wad c.wad"
	if got := commandLine("uzdoom.exe", args); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSkillLabels(t *testing.T) {
	cases := []struct {
		skill  int
		brutal bool
		want   string
	}{
		{0, true, "default"},
		{4, false, "Ultra-Violence"},
		{4, true, "Ultra-Violence"},
		{6, true, "Black Metal"},
		{8, true, "Realism Nightmare"},
		{6, false, "6"}, // unreachable without the mod loaded
	}
	for _, c := range cases {
		if got := skillLabel(c.skill, c.brutal); got != c.want {
			t.Errorf("skillLabel(%d, %v) = %q, want %q", c.skill, c.brutal, got, c.want)
		}
	}
}
