package main

import (
	"sort"
	"strings"

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
		if len(e.Files) == 0 {
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
		out = append(out, e.Name)
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
		return e.Name
	case 1:
		return statusLabel[m.l.statusOf(e.Name)]
	case 2:
		return len(e.Files)
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
