package wadinfo

import (
	"os"
	"testing"
)

// doomDir locates a Doom installation to test against. These tests read real
// wads rather than fixtures, so they skip cleanly on a machine that has none.
// Point DOOM_DIR at your own folder to run them elsewhere.
func doomDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("DOOM_DIR")
	if dir == "" {
		dir = `C:\Program Files (x86)\BD`
	}
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no Doom folder at %s - set DOOM_DIR to run this test", dir)
	}
	return dir
}
