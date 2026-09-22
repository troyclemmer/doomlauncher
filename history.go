package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lxn/walk"
)

// HistoryEntry is one recorded launch.
type HistoryEntry struct {
	Time     time.Time `json:"time"`
	IWAD     string    `json:"iwad"`
	Campaign string    `json:"campaign"`
	Brutal   bool      `json:"brutal"`
	Skill    int       `json:"skill"`
	Extra    string    `json:"extra"`
	Command  string    `json:"command"`
}

const maxHistory = 500

// historyModel backs the History tab's table. items are newest-first, which is
// the order the table shows them in.
type historyModel struct {
	walk.TableModelBase
	items []HistoryEntry
	path  string
}

func historyPath(cfgPath string) string {
	return filepath.Join(filepath.Dir(cfgPath), "history.jsonl")
}

// loadHistory reads the log, skipping any line that fails to parse rather than
// throwing away the whole history over one bad record.
func loadHistory(path string) *historyModel {
	m := &historyModel{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e HistoryEntry
		if json.Unmarshal([]byte(line), &e) == nil {
			m.items = append([]HistoryEntry{e}, m.items...) // file is oldest-first
		}
	}
	return m
}

func (m *historyModel) RowCount() int { return len(m.items) }

func (m *historyModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.items) {
		return ""
	}
	e := m.items[row]
	switch col {
	case 0:
		return e.Time.Format("2006-01-02  3:04 PM")
	case 1:
		if e.Campaign == "" {
			return "(none)"
		}
		return e.Campaign
	case 2:
		return e.IWAD
	case 3:
		if e.Brutal {
			return "yes"
		}
		return "no"
	case 4:
		return skillLabel(e.Skill, e.Brutal)
	case 5:
		return e.Extra
	}
	return ""
}

func (m *historyModel) at(row int) (HistoryEntry, bool) {
	if row < 0 || row >= len(m.items) {
		return HistoryEntry{}, false
	}
	return m.items[row], true
}

func (m *historyModel) add(e HistoryEntry) {
	m.items = append([]HistoryEntry{e}, m.items...)
	if len(m.items) > maxHistory {
		m.items = m.items[:maxHistory]
	}
	m.flush()
	m.PublishRowsReset()
}

func (m *historyModel) clear() {
	m.items = nil
	m.flush()
	m.PublishRowsReset()
}

// flush rewrites the whole log. Capped at maxHistory lines, so this stays cheap
// and keeps the trimming logic in one place.
func (m *historyModel) flush() {
	var sb strings.Builder
	for i := len(m.items) - 1; i >= 0; i-- { // write oldest-first
		if b, err := json.Marshal(m.items[i]); err == nil {
			sb.Write(b)
			sb.WriteByte('\n')
		}
	}
	os.WriteFile(m.path, []byte(sb.String()), 0o644)
}
