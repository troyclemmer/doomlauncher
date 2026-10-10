package main

import (
	"fmt"
	"sort"
	"strings"

	"doomlauncher/internal/wadinfo"

	"github.com/lxn/walk"
)

// Play status for a campaign. The order here is the order the dropdowns appear
// on the Launch tab: what you are playing now comes first, what you have
// finished or shelved sinks to the bottom.
const (
	StatusProgress = "progress"
	StatusTodo     = "todo"
	StatusDone     = "done"
	StatusIgnore   = "ignore"
)

var statusOrder = []string{StatusProgress, StatusTodo, StatusDone, StatusIgnore}

var statusLabel = map[string]string{
	StatusProgress: "In progress",
	StatusTodo:     "Not started",
	StatusDone:     "Completed",
	StatusIgnore:   "Ignored",
}

// statusKey normalises a campaign name for use as a config key.
func statusKey(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

// statusOf reports a campaign's status, defaulting to "not started" so a newly
// dropped-in wad shows up where you would expect it.
func (l *launcher) statusOf(name string) string {
	if s, ok := l.cfg.WadStatus[statusKey(name)]; ok {
		if _, valid := statusLabel[s]; valid {
			return s
		}
	}
	return StatusTodo
}

// bucketize sorts every campaign into its status group. Index matches
// statusOrder, and the "(none)" pseudo-entry is excluded - an empty selection
// in all four dropdowns is what means "IWAD only".
func (l *launcher) bucketize() {
	for i := range l.buckets {
		l.buckets[i] = nil
	}
	for _, e := range l.scan.Entries {
		if len(e.Files) == 0 && e.IWAD == "" {
			continue // the "(none - IWAD only)" placeholder
		}
		st := l.statusOf(e.Name)
		for i, s := range statusOrder {
			if s == st {
				l.buckets[i] = append(l.buckets[i], e)
			}
		}
	}
}

// bucketNames is a dropdown's model: a blank first row, then the campaigns.
// Blank in every dropdown means no campaign is loaded.
func (l *launcher) bucketNames(i int) []string {
	out := make([]string, 0, len(l.buckets[i])+1)
	out = append(out, "")
	for _, e := range l.buckets[i] {
		out = append(out, e.Display)
	}
	return out
}

// statusModel backs the Progress tab's table.
type statusModel struct {
	walk.TableModelBase
	l     *launcher
	items []Entry
}

func (m *statusModel) reload() {
	m.items = nil
	for _, e := range m.l.scan.Entries {
		if len(e.Files) > 0 {
			m.items = append(m.items, e)
		}
	}
	sort.Slice(m.items, func(a, b int) bool {
		// group by status first, then alphabetically inside each group
		sa, sb := m.l.statusOf(m.items[a].Name), m.l.statusOf(m.items[b].Name)
		if sa != sb {
			return indexOfStatus(sa) < indexOfStatus(sb)
		}
		return strings.ToLower(m.items[a].Name) < strings.ToLower(m.items[b].Name)
	})
	m.PublishRowsReset()
}

func indexOfStatus(s string) int {
	for i, v := range statusOrder {
		if v == s {
			return i
		}
	}
	return len(statusOrder)
}

func (m *statusModel) RowCount() int { return len(m.items) }

func (m *statusModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.items) {
		return ""
	}
	e := m.items[row]
	switch col {
	case 0:
		return e.Display
	case 1:
		return e.Name
	case 2:
		return statusLabel[m.l.statusOf(e.Name)]
	case 3:
		return strings.Join(e.Files, ", ")
	}
	return ""
}

// setSelectedStatus applies a status to every selected row, then rebuilds the
// Launch tab dropdowns so the campaign moves group immediately.
func (l *launcher) setSelectedStatus(status string) {
	rows := l.statusTable.SelectedIndexes()
	if len(rows) == 0 {
		return
	}
	for _, r := range rows {
		if r < 0 || r >= len(l.statuses.items) {
			continue
		}
		l.cfg.WadStatus[statusKey(l.statuses.items[r].Name)] = status
	}
	l.cfg.save(l.cfgPath)

	keep := l.currentEntry().Name
	l.refreshBuckets(keep)
	l.statuses.reload()
	l.updateStatusBar()
}

// refreshBuckets rebuilds the four dropdowns, restoring the given selection if
// it still exists somewhere.
func (l *launcher) refreshBuckets(selected string) {
	l.bucketize()
	l.loading = true
	for i := range l.campBoxes {
		l.campBoxes[i].SetModel(l.bucketNames(i))
		l.campBoxes[i].SetCurrentIndex(0)
	}
	if selected != "" {
		for i := range l.buckets {
			for j, e := range l.buckets[i] {
				if strings.EqualFold(e.Name, selected) {
					l.campBoxes[i].SetCurrentIndex(j + 1)
				}
			}
		}
	}
	l.loading = false
	l.updatePreview()
}

// promoteOnLaunch moves a campaign to "in progress" when you play it, but only
// from "not started". Anything you have deliberately marked completed or
// ignored stays where you put it, so replaying a finished wad does not undo
// that. Reports whether anything changed.
func (l *launcher) promoteOnLaunch(name string) bool {
	if name == "" || l.statusOf(name) != StatusTodo {
		return false
	}
	l.cfg.WadStatus[statusKey(name)] = StatusProgress
	return true
}

// nameEntries works out what each dropdown row should say. A preset keeps the
// name you gave it, marked with a trailing asterisk; an auto-detected wad uses
// the title from its readme when there is one, falling back to the filename.
//
// Two wads can share a readme - STRG.wad and STRG_M28.wad both match STRG.txt -
// so a title already taken by another entry is qualified with the filename
// rather than producing two identical rows.
func (l *launcher) nameEntries() {
	tf := wadinfo.NewTitleFinder(l.gameDir)
	used := map[string]string{}

	for i := range l.scan.Entries {
		e := &l.scan.Entries[i]
		// Base games already have a readable title and no readme to consult.
		if len(e.Files) == 0 || e.IWAD != "" {
			e.Display = e.Name
			continue
		}
		if l.isPreset(e.Name) {
			e.Display = e.Name + " *"
			continue
		}
		e.Display = e.Name
		title := strings.TrimSpace(tf.Title(e.Files[0]))
		if title == "" && len(e.Files) == 1 {
			title = addonTitles[strings.ToLower(e.Files[0])]
		}
		if title == "" {
			continue // no readme, or it has no Title field
		}
		if owner, taken := used[strings.ToLower(title)]; taken {
			_ = owner
			e.Display = title + " (" + e.Name + ")"
			continue
		}
		used[strings.ToLower(title)] = e.Name
		e.Display = title
	}

	// A title claimed later by a second wad must also qualify the first one,
	// or the earlier row stays ambiguous.
	counts := map[string]int{}
	for _, e := range l.scan.Entries {
		counts[strings.ToLower(e.Display)]++
	}
	for i := range l.scan.Entries {
		e := &l.scan.Entries[i]
		if counts[strings.ToLower(e.Display)] > 1 && e.Display != e.Name {
			e.Display = e.Display + " (" + e.Name + ")"
		}
	}
}

// syncBrutal enables or disables the mod checkbox for the selected base game.
// While it is disabled the stored preference must not be overwritten, or
// visiting Heretic once would quietly turn Brutal Doom off for Doom too.
func (l *launcher) syncBrutal() {
	if l.brutalBox == nil {
		return
	}
	iwad := l.currentIWAD()
	ok := l.modFile != "" && supportsDoomMods(iwad)

	l.brutalBox.SetEnabled(ok) // before SetChecked, so the handler sees the state
	switch {
	case l.modFile == "":
		l.brutalBox.SetText("Brutal Doom  (no brutal*.pk3 found in this folder)")
	case !ok:
		l.brutalBox.SetText("Brutal Doom  (not compatible with " + iwad + ")")
	default:
		l.brutalBox.SetText("Brutal Doom  (" + l.modFile + ")")
	}
	if !ok {
		l.brutalBox.SetChecked(false)
	} else {
		l.brutalBox.SetChecked(l.cfg.BrutalOn != nil && *l.cfg.BrutalOn)
	}
	l.onBrutalChanged() // the skill list depends on whether the mod is loaded
}

// updateStatusBar refreshes the counts along the bottom of the window. These
// are the numbers you would compare between two machines to see whether the
// collections and progress actually match.
func (l *launcher) updateStatusBar() {
	if l.sbMain == nil {
		return
	}
	var campaigns, bases int
	var byStatus [4]int
	for _, e := range l.scan.Entries {
		if len(e.Files) == 0 && e.IWAD == "" {
			continue
		}
		if e.IWAD != "" {
			bases++
		} else {
			campaigns++
		}
		byStatus[indexOfStatus(l.statusOf(e.Name))]++
	}

	engine := l.engineVer
	if engine == "" {
		engine = l.cfg.Engine
	}
	// Ordered so the least important field truncates first on a narrow window.
	l.sbMain.SetText(fmt.Sprintf(
		"%s     %d campaigns, %d base games, %d presets     %d done / %d playing / %d to play / %d ignored     %s",
		engine, campaigns, bases, len(l.cfg.Presets),
		byStatus[indexOfStatus(StatusDone)], byStatus[indexOfStatus(StatusProgress)],
		byStatus[indexOfStatus(StatusTodo)], byStatus[indexOfStatus(StatusIgnore)],
		wadinfo.HumanSize(l.scan.TotalBytes)))
}

// crlf is what the Windows message box and edit controls want for a line break.
const crlf = "\r\n"

// statusLabels lists the statuses in dropdown order.
func statusLabels() []string {
	out := make([]string, 0, len(statusOrder))
	for _, s := range statusOrder {
		out = append(out, statusLabel[s])
	}
	return out
}

// syncStatusBox points the Status control at whatever is currently selected.
func (l *launcher) syncStatusBox() {
	if l.statusBox == nil {
		return
	}
	entry := l.currentEntry()
	if entry.Name == "" {
		l.statusBox.SetEnabled(false)
		l.statusBox.SetCurrentIndex(-1)
		return
	}
	l.statusBox.SetEnabled(true)
	l.statusBox.SetCurrentIndex(indexOfStatus(l.statusOf(entry.Name)))
}

// onStatusBoxChanged reassigns the selected campaign's status from the Launch
// tab, so you do not have to go hunting for the Progress tab to do it.
func (l *launcher) onStatusBoxChanged() {
	if l.loading || l.statusBox == nil {
		return
	}
	entry := l.currentEntry()
	i := l.statusBox.CurrentIndex()
	if entry.Name == "" || i < 0 || i >= len(statusOrder) {
		return
	}
	if statusOrder[i] == l.statusOf(entry.Name) {
		return
	}
	l.cfg.WadStatus[statusKey(entry.Name)] = statusOrder[i]
	l.cfg.save(l.cfgPath)
	l.refreshBuckets(entry.Name)
	l.statuses.reload()
	l.updateStatusBar()
	l.syncStatusBox()
}
