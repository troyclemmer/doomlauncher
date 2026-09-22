package wadinfo

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Readme is the text file that ships beside a wad. Most follow the /idgames
// template, a run of "Field : value" lines, so the common fields can be pulled
// out even though nothing about the format is enforced.
type Readme struct {
	Path   string
	Fields map[string]string
	Raw    string
}

// fieldPat matches "Release date            : 2015-04-16". The key is kept
// short and wordlike to avoid matching prose that happens to contain a colon.
var fieldPat = regexp.MustCompile(`^[ \t]*([A-Za-z][A-Za-z0-9 /'&.()-]{1,28}?)[ \t]*:[ \t]*(.*)$`)

// normalize reduces a filename to comparable letters and digits.
func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var versionTail = regexp.MustCompile(`(v?[0-9]+)+$`)

// stem strips a trailing version so "sunder2308" can still find
// "SunderReadMeV2.txt" and "oku2v31" can find an "oku" readme.
func stem(s string) string {
	t := versionTail.ReplaceAllString(s, "")
	if len(t) < 4 {
		return s
	}
	return t
}

// FindReadme looks beside a wad for its text file. Names rarely match exactly -
// deathless.wad ships deathless1.1.txt, Sunder 2308.wad ships
// SunderReadMeV2.TXT - so candidates are scored rather than matched.
func FindReadme(wadPath string) string {
	dir := filepath.Dir(wadPath)
	base := filepath.Base(wadPath)
	want := normalize(strings.TrimSuffix(base, filepath.Ext(base)))
	if want == "" {
		return ""
	}
	short := stem(want)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	best, bestScore := "", 0
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".txt") {
			continue
		}
		got := normalize(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
		if got == "" {
			continue
		}

		score := 0
		switch {
		case got == want:
			score = 100
		case strings.HasPrefix(got, want):
			score = 80
		case strings.HasPrefix(want, got) && len(got) >= 4:
			score = 70
		case len(short) >= 4 && strings.HasPrefix(got, short):
			score = 50
		}
		if score == 0 {
			continue
		}
		// Among equal matches prefer the shorter name: "Eviternity.txt" beats
		// "Eviternity_Changelog.txt".
		score = score*1000 - len(got)
		if score > bestScore {
			best, bestScore = filepath.Join(dir, e.Name()), score
		}
	}
	return best
}

// LoadReadme reads and parses a text file.
func LoadReadme(path string) (*Readme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")

	r := &Readme{Path: path, Fields: map[string]string{}, Raw: text}
	for _, line := range strings.Split(text, "\n") {
		m := fieldPat.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(m[1]))
		val := strings.TrimSpace(m[2])
		if val == "" || r.Fields[key] != "" {
			continue // keep the first occurrence; later ones are usually prose
		}
		r.Fields[key] = val
	}
	return r, nil
}

// Get returns the first of the named fields that has a value.
func (r *Readme) Get(keys ...string) string {
	if r == nil {
		return ""
	}
	for _, k := range keys {
		if v := r.Fields[k]; v != "" {
			return v
		}
	}
	return ""
}

// Summary renders the one-line description shown under the wad stats.
func (r *Readme) Summary() string {
	if r == nil {
		return ""
	}
	var parts []string
	if t := r.Get("title"); t != "" {
		parts = append(parts, t)
	}
	if a := r.Get("author", "authors"); a != "" {
		parts = append(parts, "by "+a)
	}
	if d := r.Get("release date", "date released", "date", "date finished"); d != "" {
		parts = append(parts, d)
	}
	return strings.Join(parts, "   -   ")
}
