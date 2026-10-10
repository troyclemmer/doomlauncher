package main

import (
	"path/filepath"
	"testing"
)

func TestEngineVersionFromPE(t *testing.T) {
	dir := doomDir(t)
	got := engineVersion(filepath.Join(dir, "uzdoom.exe"))
	if got == "" {
		t.Fatal("no version read from the engine binary")
	}
	t.Logf("engine reports: %s", got)
}

func TestEngineVersionHandlesRubbish(t *testing.T) {
	if got := engineVersion(filepath.Join(t.TempDir(), "nope.exe")); got != "" {
		t.Errorf("missing file should give \"\", got %q", got)
	}
	// a real file that is not a PE binary
	if got := engineVersion("README.md"); got != "" {
		t.Errorf("non-PE file should give \"\", got %q", got)
	}
}
