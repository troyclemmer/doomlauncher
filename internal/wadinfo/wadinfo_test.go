package wadinfo

import (
	"fmt"
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
