package wadinfo

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestReadmeMatching(t *testing.T) {
	dir := doomDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skip(err)
	}
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if e.IsDir() || (ext != ".wad" && ext != ".WAD" && ext != ".pk3") {
			continue
		}
		p := FindReadme(filepath.Join(dir, e.Name()))
		if p == "" {
			fmt.Printf("%-24s -> (none)\n", e.Name())
			continue
		}
		r, err := LoadReadme(p)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("%-24s -> %-34s %s\n", e.Name(), filepath.Base(p), r.Summary())
	}
}
