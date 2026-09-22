package main

import (
	"fmt"
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
