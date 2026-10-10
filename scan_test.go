package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestScanRealFolder(t *testing.T) {
	dir := doomDir(t)
	cfg := defaultConfig()
	r := scanDir(dir, cfg.Presets)
	fmt.Println("IWADS:", r.IWADs)
	fmt.Println("MODS: ", r.Mods)
	fmt.Println("ENTRIES:")
	for _, e := range r.Entries {
		fmt.Printf("   %-28s %v\n", e.Name, e.Files)
	}
}

func TestSplitArgs(t *testing.T) {
	got := splitArgs(`-warp 07 +map "MAP 07"  -skill 4`)
	want := []string{"-warp", "07", "+map", "MAP 07", "-skill", "4"}
	if len(got) != len(want) {
		t.Fatalf("got %q want %q", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %q want %q", got, want)
		}
	}
}

// A preset does not freeze the campaign list. The dropdowns are rebuilt from
// the directory on every scan, so dropping a new wad in still shows up; the
// preset only claims the specific files it names.
func TestPresetsDoNotBlockAutoDetection(t *testing.T) {
	dir := t.TempDir()
	write := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"DOOM2.WAD", "alpha.wad", "beta.wad", "gamma.wad", "brutal22.pk3"} {
		write(f)
	}
	presets := []Preset{{Name: "Bundle", Files: []string{"alpha.wad", "beta.wad"}}}

	names := func() []string {
		var out []string
		for _, e := range scanDir(dir, presets).Entries {
			if len(e.Files) > 0 {
				out = append(out, e.Name)
			}
		}
		return out
	}

	got := names()
	want := []string{"Bundle", "gamma"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v (claimed files must not also appear alone)", got, want)
	}

	// Drop in a new wad after the preset exists.
	write("delta.wad")
	got = names()
	want = []string{"Bundle", "delta", "gamma"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("new wad was not auto-detected: got %v want %v", got, want)
	}
}
