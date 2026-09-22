package wadinfo

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestEviternityMatching(t *testing.T) {
	dir := doomDir(t)
	for _, wad := range []string{"Eviternity II.wad", "Eviternity.wad"} {
		fmt.Printf("%-20s -> %s\n", wad, filepath.Base(FindReadme(filepath.Join(dir, wad))))
	}
	fmt.Println("\nnormalised:")
	for _, n := range []string{"Eviternity II", "eviternityii", "Eviternity",
		"EVITERNITY II - Final Version - CHANGELOG", "Eviternity_Changelog"} {
		fmt.Printf("   %-44s -> %s\n", n, normalize(n))
	}
	if got := filepath.Base(FindReadme(filepath.Join(dir, "Eviternity II.wad"))); got != "eviternityii.txt" {
		t.Fatalf("Eviternity II matched %q", got)
	}
	if got := filepath.Base(FindReadme(filepath.Join(dir, "Eviternity.wad"))); got != "Eviternity.txt" {
		t.Fatalf("Eviternity matched %q", got)
	}
}
