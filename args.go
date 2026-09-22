package main

import (
	"strconv"
	"strings"
)

// launchSpec is everything the UI contributes to one launch.
type launchSpec struct {
	IWAD   string
	Files  []string // campaign files, in the order they must load
	Mod    string   // Brutal Doom build, appended last
	Brutal bool
	Skill  int
	Extra  string
}

// buildArgs turns a spec into the uzdoom.exe argument list. Campaign files keep
// the order they were listed in; the mod is always appended after them, which is
// what the hand-written .bat files did.
func buildArgs(s launchSpec) []string {
	var args []string
	if s.IWAD != "" {
		args = append(args, "-iwad", s.IWAD)
	}

	files := append([]string{}, s.Files...)
	if s.Brutal && s.Mod != "" {
		files = append(files, s.Mod)
	}
	if len(files) > 0 {
		args = append(args, "-file")
		args = append(args, files...)
	}

	if s.Skill > 0 {
		args = append(args, "-skill", strconv.Itoa(s.Skill))
	}
	return append(args, splitArgs(s.Extra)...)
}

func commandLine(engine string, args []string) string {
	parts := []string{quoteForDisplay(engine)}
	for _, a := range args {
		parts = append(parts, quoteForDisplay(a))
	}
	return strings.Join(parts, " ")
}

// skillLabel names a skill number. Brutal Doom replaces the stock five skills
// with eight of its own, so the same number means different things depending
// on whether the mod is loaded.
func skillLabel(skill int, brutal bool) string {
	if skill <= 0 {
		return "default"
	}
	names := vanillaSkills
	if brutal {
		names = brutalSkills
	}
	if skill < len(names) {
		if i := strings.Index(names[skill], " - "); i >= 0 {
			return names[skill][i+3:]
		}
		return names[skill]
	}
	return strconv.Itoa(skill)
}
