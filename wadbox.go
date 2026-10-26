package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"doomlauncher/internal/wadinfo"
)

// wadCache remembers scan results so flicking through the dropdown does not
// re-read a 300 MB megawad every time.
type wadCache struct {
	mu     sync.Mutex
	seen   map[string]*wadinfo.Info
	bad    map[string]string
	readme map[string]*wadinfo.Readme // present with a nil value means "looked, none found"
}

func newWadCache() *wadCache {
	return &wadCache{
		seen:   map[string]*wadinfo.Info{},
		bad:    map[string]string{},
		readme: map[string]*wadinfo.Readme{},
	}
}

func (c *wadCache) get(path string) (*wadinfo.Info, string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if info, ok := c.seen[path]; ok {
		return info, "", true
	}
	if msg, ok := c.bad[path]; ok {
		return nil, msg, true
	}
	return nil, "", false
}

func (c *wadCache) put(path string, info *wadinfo.Info, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.bad[path] = err.Error()
		return
	}
	c.seen[path] = info
}

func (c *wadCache) getReadme(path string) (*wadinfo.Readme, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.readme[path]
	return r, ok
}

func (c *wadCache) putReadme(path string, r *wadinfo.Readme) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readme[path] = r
}

// auxKind names the non-wad files that legitimately ride along with a
// campaign, so they are described rather than reported as parse failures.
func auxKind(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".deh":
		return "DeHackEd patch"
	case ".bex":
		return "BEX patch"
	case ".lmp":
		return "demo/lump"
	}
	return ""
}

func shortNames(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.Base(p)
	}
	return out
}

func (l *launcher) abs(f string) string {
	if filepath.IsAbs(f) {
		return f
	}
	return filepath.Join(l.gameDir, f)
}

// selectionPaths lists the files behind the current campaign, falling back to
// the IWAD when nothing is loaded on top of the base game.
func (l *launcher) selectionPaths() []string {
	entry := l.currentEntry()
	paths := make([]string, 0, len(entry.Files)+1)
	for _, f := range entry.Files {
		paths = append(paths, l.abs(f))
	}
	if len(paths) == 0 {
		if iwad := l.currentIWAD(); iwad != "" {
			paths = append(paths, l.abs(iwad))
		}
	}
	return paths
}

// primaryPath is the file a readme would describe: the first real wad or pk3.
func primaryPath(paths []string) string {
	for _, p := range paths {
		if auxKind(p) == "" {
			return p
		}
	}
	return ""
}

// updateWadInfo refreshes the info box and Readme tab for the current
// selection. Scanning happens on a worker so the window never blocks on a slow
// disk, and a generation counter makes stale results harmless.
func (l *launcher) updateWadInfo() {
	paths := l.selectionPaths()
	if len(paths) == 0 {
		l.wadBox.SetText("")
		l.setReadme(nil)
		return
	}

	l.wadGen++
	gen := l.wadGen
	primary := primaryPath(paths)

	// Anything already cached renders immediately, with no flicker.
	if text, complete := l.renderWadInfo(paths, primary); complete {
		l.wadBox.SetText(text)
		r, _ := l.wads.getReadme(primary)
		l.setReadme(r)
		return
	}
	l.wadBox.SetText(strings.Join(shortNames(paths), " + ") + "\r\nreading...")

	go func() {
		for _, p := range paths {
			if auxKind(p) != "" {
				continue
			}
			if _, _, ok := l.wads.get(p); ok {
				continue
			}
			info, err := wadinfo.Scan(p)
			l.wads.put(p, info, err)
		}
		if primary != "" {
			if _, ok := l.wads.getReadme(primary); !ok {
				var r *wadinfo.Readme
				if txt := wadinfo.FindReadme(primary); txt != "" {
					r, _ = wadinfo.LoadReadme(txt)
				}
				l.wads.putReadme(primary, r)
			}
		}
		l.mw.Synchronize(func() {
			if gen != l.wadGen {
				return // the selection moved on while we were reading
			}
			l.suggestIWAD(primary)
			text, _ := l.renderWadInfo(paths, primary)
			l.wadBox.SetText(text)
			r, _ := l.wads.getReadme(primary)
			l.setReadme(r)
			// Whether a campaign is a total conversion is only known once it
			// has been read, which is after the checkbox was last set.
			l.syncBrutalIfChanged()
		})
	}()
}

// renderWadInfo builds the summary, reporting whether every file involved was
// already cached.
func (l *launcher) renderWadInfo(paths []string, primary string) (string, bool) {
	complete := true
	var withMaps []*wadinfo.Info
	var notes []string
	var totals [3]int
	var bytes int64
	mapCount, udmf := 0, 0

	for _, p := range paths {
		if kind := auxKind(p); kind != "" {
			notes = append(notes, filepath.Base(p)+" ("+kind+")")
			continue
		}
		info, badMsg, ok := l.wads.get(p)
		if !ok {
			complete = false
			continue
		}
		if info == nil {
			notes = append(notes, filepath.Base(p)+" ("+badMsg+")")
			continue
		}
		bytes += info.Bytes
		playable := len(info.Maps) - info.UDMFCount()
		if playable > 0 || info.UDMFCount() > 0 {
			withMaps = append(withMaps, info)
			mapCount += playable
			udmf += info.UDMFCount()
			t := info.Totals()
			for k := 0; k < 3; k++ {
				totals[k] += t[k]
			}
		} else {
			notes = append(notes, filepath.Base(p)+" (no maps)")
		}
	}
	if !complete {
		return "", false
	}
	if primary != "" {
		if _, ok := l.wads.getReadme(primary); !ok {
			return "", false
		}
	}

	var head string
	switch len(withMaps) {
	case 0:
		head = strings.Join(shortNames(paths), " + ") + "  -  no maps found"
	case 1:
		head = fmt.Sprintf("%s  -  %s, %d maps (%s), %s",
			withMaps[0].File, withMaps[0].Kind, mapCount,
			withMaps[0].MapRange(), wadinfo.HumanSize(bytes))
	default:
		names := make([]string, len(withMaps))
		for i, w := range withMaps {
			names[i] = w.File
		}
		head = fmt.Sprintf("%s  -  %d maps, %s",
			strings.Join(names, " + "), mapCount, wadinfo.HumanSize(bytes))
	}
	if udmf > 0 {
		head += fmt.Sprintf("  (+%d UDMF)", udmf)
	}
	if len(notes) > 0 {
		head += "   [" + strings.Join(notes, ", ") + "]"
	}

	lines := []string{head}
	if mapCount > 0 {
		monsters := fmt.Sprintf("Monsters    sk1-2: %s     sk3: %s     sk4-5: %s",
			wadinfo.Commas(totals[0]), wadinfo.Commas(totals[1]), wadinfo.Commas(totals[2]))
		if l.brutalBox != nil && l.brutalBox.Checked() {
			monsters += "  (sk4-8)"
		}
		lines = append(lines, monsters)
	}
	if info, _, ok := l.wads.get(primary); ok && info != nil && info.Kind == "IWAD" && !info.HasPalette {
		lines = append(lines, "WARNING: no PLAYPAL lump - this engine cannot run it")
	}
	if era := l.eraOf(primary); era != "" {
		lines = append(lines, "Map names are "+era+" style  -  needs a "+era+" base game")
	}
	if r, _ := l.wads.getReadme(primary); r != nil {
		if sum := r.Summary(); sum != "" {
			lines = append(lines, sum)
		}
	}
	return strings.Join(lines, "\r\n"), true
}

// setReadme fills the Readme tab, which shows the text file verbatim.
func (l *launcher) setReadme(r *wadinfo.Readme) {
	if l.readmeBox == nil {
		return
	}
	l.readmeFile = ""
	if r == nil {
		l.readmeName.SetText("No readme text file found next to this wad.")
		l.readmeBox.SetText("")
		return
	}
	l.readmeFile = r.Path
	l.readmeName.SetText(filepath.Base(r.Path))
	// The Windows edit control needs CRLF; wadinfo normalises to LF.
	l.readmeBox.SetText(strings.ReplaceAll(r.Raw, "\n", "\r\n"))
}

// onReadmeOpen hands the text file to whatever normally opens .txt files.
func (l *launcher) onReadmeOpen() {
	if l.readmeFile == "" {
		return
	}
	cmd := exec.Command("cmd", "/c", "start", "", l.readmeFile)
	cmd.Dir = l.gameDir
	cmd.Start()
}

// eraOf describes which generation of base game a campaign's maps imply.
func (l *launcher) eraOf(primary string) string {
	info, _, ok := l.wads.get(primary)
	if !ok || info == nil {
		return ""
	}
	switch info.Era() {
	case "doom1":
		return "Doom 1"
	case "doom2":
		return "Doom 2"
	}
	return ""
}

// suggestIWAD picks the right base game for a campaign the first time you
// select it, so SIGIL does not silently launch on Doom 2 months from now
// because you forgot it is episode 5. Your own choice always wins: once a
// campaign has a remembered base game, this leaves it alone.
func (l *launcher) suggestIWAD(primary string) {
	entry := l.currentEntry()
	if entry.Name == "" || entry.IWAD != "" {
		return
	}
	if _, remembered := l.cfg.CampaignIWAD[strings.ToLower(entry.Name)]; remembered {
		return
	}
	current := l.currentIWAD()
	if !supportsDoomMods(current) {
		return // Heretic and friends also use ExMy names; do not touch them
	}

	var want []string
	switch l.eraOf(primary) {
	case "Doom 1":
		if isDoom1IWAD(current) {
			return
		}
		want = []string{"DOOM.WAD", "DOOM1.WAD", "FREEDOOM1.WAD"}
	case "Doom 2":
		if !isDoom1IWAD(current) {
			return
		}
		want = []string{"DOOM2.WAD", "FREEDOOM2.WAD", "TNT.WAD", "PLUTONIA.WAD"}
	default:
		return
	}
	for _, w := range want {
		if i := indexOfFold(l.scan.IWADs, w, -1); i >= 0 {
			l.loading = true
			l.iwadBox.SetCurrentIndex(i)
			l.loading = false
			l.cfg.CampaignIWAD[strings.ToLower(entry.Name)] = l.scan.IWADs[i]
			l.preview.SetText(commandLine(l.cfg.Engine, buildArgs(l.spec())))
			return
		}
	}
}

// isDoom1IWAD reports whether a base game uses episode-and-map naming.
func isDoom1IWAD(iwad string) bool {
	switch strings.ToLower(iwad) {
	case "doom.wad", "doom1.wad", "freedoom1.wad":
		return true
	}
	return false
}

// unplayableReason checks the selected base game before handing it to the
// engine. A wad with no PLAYPAL makes the engine abort with a bare
// "GetNumForName: PLAYPAL not found!", which says nothing useful.
func (l *launcher) unplayableReason() string {
	iwad := l.currentIWAD()
	if iwad == "" {
		return ""
	}
	// Selecting a base game clears the campaign, so a clash should be
	// unreachable from the UI. It can still arrive in a launcher.json copied
	// from another machine whose wads differ, so check before launching.
	if entry := l.currentEntry(); entry.Name != "" && l.clashesWith(entry, iwad) {
		return entry.Name + " cannot run on " + iwad + "." + crlf + crlf +
			"Pick a base game that matches the campaign, or choose a different campaign."
	}
	path := l.abs(iwad)
	info, _, cached := l.wads.get(path)
	if !cached {
		var err error
		info, err = wadinfo.Scan(path)
		l.wads.put(path, info, err)
	}
	if info == nil || info.HasPalette {
		return ""
	}
	return iwad + " has no PLAYPAL lump, so this engine cannot run it.\n\n" +
		"Doom 64 stores its data in its own format rather than as Doom lumps. " +
		"To play it you need a dedicated port such as Doom64EX-Plus, or a " +
		"GZDoom recreation like Doom 64: Retribution."
}
