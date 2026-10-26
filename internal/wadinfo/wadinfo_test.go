package wadinfo

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestScanRealFiles(t *testing.T) {
	dir := doomDir(t)
	for _, name := range []string{
		"DOOM2.WAD", "sunlust.wad", "Eviternity II.wad",
		"EdayTest001.pk3", "brutal22test6.pk3", "Credits.txt",
	} {
		info, err := Scan(filepath.Join(dir, name))
		if err != nil {
			fmt.Printf("%-20s ERROR: %v\n", name, err)
			continue
		}
		tot := info.Totals()
		fmt.Printf("%-20s %-4s %3d maps (%s) %9s  sk1-2 %s / sk3 %s / sk4-5 %s\n",
			info.File, info.Kind, len(info.Maps)-info.UDMFCount(), info.MapRange(),
			HumanSize(info.Bytes), Commas(tot[0]), Commas(tot[1]), Commas(tot[2]))
	}
}

func TestCommas(t *testing.T) {
	for _, c := range []struct {
		in   int
		want string
	}{{0, "0"}, {7, "7"}, {999, "999"}, {1000, "1,000"}, {52785, "52,785"}, {1234567, "1,234,567"}} {
		if got := Commas(c.in); got != c.want {
			t.Errorf("Commas(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A total conversion is recognised by its GAMEINFO lump, which is what GZDoom
// itself uses to decide a file is a whole game rather than an add-on. Ashes
// keeps one in each campaign pk3; an ordinary megawad has none.
func TestStandaloneDetection(t *testing.T) {
	dir := doomDir(t)
	cases := map[string]bool{
		"Ashes2063Enriched2_23.pk3":    true,
		"AshesAfterglow1_15.pk3":       true,
		"AshesHardReset1_00.pk3":       true,
		"AshesSAMenu.pk3":              false, // menu skin only, no GAMEINFO
		"Ashes2063EnrichedFDPatch.pk3": false,
		"sunlust.wad":                  false,
		"freedoom2.wad":                false,
	}
	ran := 0
	for name, want := range cases {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err != nil {
			continue
		}
		info, err := Scan(p)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		ran++
		if info.Standalone != want {
			t.Errorf("%s: Standalone = %v, want %v", name, info.Standalone, want)
		}
	}
	if ran == 0 {
		t.Skip("none of the sample files are present")
	}
}
