// Command wadstat reports what is inside a Doom wad: which maps it holds and
// how many monsters each one spawns at each difficulty.
//
// Drag one or more .wad files (or a folder of them) onto the exe, or run:
//
//	wadstat.exe sunlust.wad          every map, totals only
//	wadstat.exe sunlust.wad MAP28    one map, broken down by monster
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"doomlauncher/internal/wadinfo"
)

func main() {
	files, mapFilter := parseArgs(os.Args[1:])

	if len(files) == 0 {
		fmt.Println("wadstat - monster counts per difficulty in a Doom wad")
		fmt.Println()
		fmt.Println("  Drag one or more .wad files (or a folder of them) onto this exe,")
		fmt.Println("  or run:")
		fmt.Println("    wadstat.exe sunlust.wad          every map, totals only")
		fmt.Println("    wadstat.exe sunlust.wad MAP28    one map, broken down by monster")
		pauseIfLaunchedFromExplorer()
		os.Exit(2)
	}
	defer pauseIfLaunchedFromExplorer()

	printLegend()
	for i, f := range files {
		if i > 0 {
			fmt.Println()
			fmt.Println(strings.Repeat("=", 44))
			fmt.Println()
		}
		// One unreadable file must not abandon the rest of the batch.
		if err := report(f, mapFilter); err != nil {
			fmt.Printf("%s: %v\n", filepath.Base(f), err)
		}
	}
}

// parseArgs separates dropped files from a map-name filter. Explorer passes
// every dropped path as an argument, so anything that exists on disk is a file
// to read and anything else is treated as the map name.
func parseArgs(args []string) (files []string, mapFilter string) {
	for _, a := range args {
		st, err := os.Stat(a)
		switch {
		case err == nil && st.IsDir():
			// A dropped folder means "every wad in here".
			if matches, gerr := filepath.Glob(filepath.Join(a, "*.wad")); gerr == nil {
				files = append(files, matches...)
			}
		case err == nil:
			files = append(files, a)
		case mapFilter == "":
			mapFilter = strings.ToUpper(a)
		}
	}
	return files, mapFilter
}

// printLegend explains why there are only three columns. A classic Doom map
// stores three skill bits per thing, so five (or eight) skills can only ever
// resolve to three distinct monster line-ups.
func printLegend() {
	fmt.Println("A classic-format map holds only THREE monster sets:")
	fmt.Println("  sk1-2   I'm Too Young To Die  +  Hey, Not Too Rough")
	fmt.Println("  sk3     Hurt Me Plenty")
	fmt.Println("  sk4-5   Ultra-Violence  +  Nightmare")
	fmt.Println("          +  Black Metal, Realism Violence, Realism Nightmare")
	fmt.Println("             (Brutal Doom skills 6-8)")
	fmt.Println()
	fmt.Println("Skills 4-8 all fight the same monsters - they differ in damage,")
	fmt.Println("monster health and respawn, not in which monsters spawn.")
	fmt.Println()
}

func report(path, want string) error {
	info, err := wadinfo.Scan(path)
	if err != nil {
		return err
	}

	fmt.Printf("%s  (%s, %d lumps)\n\n", info.File, info.Kind, info.Lumps)
	if want == "" {
		fmt.Printf("%-8s %8s %8s %8s\n", "map", "sk1-2", "sk3", "sk4-5")
		fmt.Println(strings.Repeat("-", 36))
	}

	var grand [3]int
	found := false
	for _, m := range info.Maps {
		if m.UDMF || (want != "" && m.Name != want) {
			continue
		}
		found = true
		for k := 0; k < 3; k++ {
			grand[k] += m.Counts[k]
		}
		if want == "" {
			fmt.Printf("%-8s %8d %8d %8d\n", m.Name, m.Counts[0], m.Counts[1], m.Counts[2])
			continue
		}
		printBreakdown(m)
	}

	udmf := info.UDMFCount()
	if !found && udmf == 0 {
		if want != "" {
			return errors.New("no classic-format map called " + want)
		}
		return errors.New("no classic-format maps found")
	}
	if want == "" && found {
		fmt.Println(strings.Repeat("-", 36))
		fmt.Printf("%-8s %8d %8d %8d\n", "TOTAL", grand[0], grand[1], grand[2])
	}
	if udmf > 0 {
		fmt.Printf("\nNote: %d UDMF (text format) map(s) skipped - open those in SLADE\n", udmf)
		fmt.Println("or Ultimate Doom Builder instead.")
	}
	return nil
}

func printBreakdown(m wadinfo.MapInfo) {
	fmt.Printf("%-18s %6s %6s %6s\n", m.Name, "sk1-2", "sk3", "sk4-5")
	fmt.Println(strings.Repeat("-", 44))

	seen := map[string]bool{}
	var kinds []string
	for _, set := range m.ByName {
		for k := range set {
			if !seen[k] {
				seen[k] = true
				kinds = append(kinds, k)
			}
		}
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		fmt.Printf("%-18s %6d %6d %6d\n", k,
			m.ByName[0][k], m.ByName[1][k], m.ByName[2][k])
	}
	fmt.Println(strings.Repeat("-", 44))
	fmt.Printf("%-18s %6d %6d %6d\n", "TOTAL", m.Counts[0], m.Counts[1], m.Counts[2])
}
