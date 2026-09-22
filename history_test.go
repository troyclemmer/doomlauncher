package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "history.jsonl")

	m := loadHistory(p)
	if m.RowCount() != 0 {
		t.Fatal("fresh history should be empty")
	}

	m.add(HistoryEntry{Time: time.Now().Add(-time.Hour), IWAD: "DOOM2.WAD",
		Campaign: "sunlust", Brutal: true, Skill: 4, Command: "uzdoom.exe -iwad DOOM2.WAD"})
	m.add(HistoryEntry{Time: time.Now(), IWAD: "DOOM2.WAD",
		Campaign: "Extermination Day", Brutal: true, Skill: 0, Command: "uzdoom.exe -file a b c"})

	back := loadHistory(p)
	if back.RowCount() != 2 {
		t.Fatalf("want 2 rows, got %d", back.RowCount())
	}
	// newest first
	if back.Value(0, 1) != "Extermination Day" {
		t.Fatalf("row 0 should be newest, got %v", back.Value(0, 1))
	}
	if back.Value(0, 4) != "default" || back.Value(1, 4) != "Ultra-Violence" {
		t.Fatalf("skill rendering wrong: %v %v", back.Value(0, 4), back.Value(1, 4))
	}
	if back.Value(0, 3) != "yes" {
		t.Fatalf("brutal column wrong: %v", back.Value(0, 3))
	}

	raw, _ := os.ReadFile(p)
	fmt.Printf("history.jsonl:\n%s", raw)

	back.clear()
	if loadHistory(p).RowCount() != 0 {
		t.Fatal("clear did not empty the log")
	}
}

func TestHistoryCap(t *testing.T) {
	m := &historyModel{path: filepath.Join(t.TempDir(), "h.jsonl")}
	for i := 0; i < maxHistory+50; i++ {
		m.add(HistoryEntry{Time: time.Now(), Campaign: fmt.Sprint(i)})
	}
	if m.RowCount() != maxHistory {
		t.Fatalf("want cap %d, got %d", maxHistory, m.RowCount())
	}
	if m.Value(0, 1) != fmt.Sprint(maxHistory+49) {
		t.Fatalf("newest entry lost: %v", m.Value(0, 1))
	}
}
