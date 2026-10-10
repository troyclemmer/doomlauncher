// Package wadinfo reads Doom wad and pk3 files and reports what maps and
// monsters they contain. It reads only the directory and the THINGS lumps
// rather than the whole file, so scanning a 300 MB megawad costs a few MB.
package wadinfo

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Monsters maps Doom thing type IDs to names. Anything else in a THINGS lump
// is a pickup, a decoration or a player start.
var Monsters = map[int16]string{
	3004: "Zombieman", 9: "Shotgun Guy", 65: "Chaingunner",
	3001: "Imp", 3002: "Demon", 58: "Spectre",
	3005: "Cacodemon", 3006: "Lost Soul", 69: "Hell Knight", 3003: "Baron of Hell",
	68: "Arachnotron", 71: "Pain Elemental", 66: "Revenant", 67: "Mancubus",
	64: "Arch-Vile", 7: "Spider Mastermind", 16: "Cyberdemon", 84: "Wolfenstein SS",
}

// Skill flag bits on each thing. Skills 1-2 share a bit and 4-5 share a bit, so
// a classic-format map can only ever hold three distinct line-ups.
const (
	SkillEasy   = 1 // skills 1 and 2
	SkillMedium = 2 // skill 3
	SkillHard   = 4 // skills 4 and 5
	multiOnly   = 16
)

// maxEmbedded caps how large an embedded wad we will pull into memory.
const maxEmbedded = 512 << 20

// Set indexes into the three monster line-ups a classic map can hold.
const (
	Easy = iota
	Medium
	Hard
)

// mapLumps are the lumps that belong to a map. A map marker is any lump
// followed by THINGS or TEXTMAP, and the map owns every mapLump after it -
// map names are arbitrary, so matching MAP01/E1M1 by pattern misses wads like
// Extermination Day, whose maps are called EDAY01..EDAY31.
var mapLumps = map[string]bool{
	"THINGS": true, "LINEDEFS": true, "SIDEDEFS": true, "VERTEXES": true,
	"SEGS": true, "SSECTORS": true, "NODES": true, "SECTORS": true,
	"REJECT": true, "BLOCKMAP": true, "BEHAVIOR": true, "SCRIPTS": true,
	"TEXTMAP": true, "ENDMAP": true, "ZNODES": true, "DIALOGUE": true,
	"GL_VERT": true, "GL_SEGS": true, "GL_SSECT": true, "GL_NODES": true,
	"GL_PVS": true, "LIGHTS": true, "MACROS": true,
}

// MapInfo is one map's monster census.
type MapInfo struct {
	Name   string
	UDMF   bool
	Counts [3]int            // totals for easy / medium / hard
	ByName [3]map[string]int // per monster name, same indexing
}

// Info is everything we learned about one file.
type Info struct {
	// HasPalette reports whether the file carries a PLAYPAL lump. Every
	// Doom-engine IWAD needs one; Doom 64's re-release wad has none, and the
	// engine dies with "GetNumForName: PLAYPAL not found!" rather than
	// explaining itself.
	HasPalette bool

	File  string
	Kind  string // IWAD, PWAD or PK3
	Lumps int
	Bytes int64
	Maps  []MapInfo
}

type lump struct {
	name string
	pos  int32
	size int32
}

// Scan reads a wad or pk3 and reports its maps.
func Scan(path string) (*Info, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pk3", ".ipk3", ".zip":
		return scanArchive(path, st.Size())
	case ".pk7":
		return nil, errors.New("pk7 archives are not supported")
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := parseWAD(f, st.Size())
	if err != nil {
		return nil, err
	}
	info.File = filepath.Base(path)
	info.Bytes = st.Size()
	return info, nil
}

// scanArchive handles pk3 files, where each map is a small wad stored under
// maps/ inside the zip.
func scanArchive(path string, size int64) (*Info, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	// An archive keeps its palette as a file rather than a lump; assume present.
	info := &Info{File: filepath.Base(path), Kind: "PK3", Bytes: size, Lumps: len(zr.File), HasPalette: true}
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		// Maps may sit under maps/ or as a whole wad embedded anywhere in the
		// archive - Extermination Day keeps its in edaymaps.wad at the root.
		if !strings.HasSuffix(name, ".wad") || f.UncompressedSize64 > maxEmbedded {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}
		sub, err := parseWAD(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			continue
		}
		// An embedded map wad is named by its file, not by a marker lump.
		base := strings.ToUpper(strings.TrimSuffix(filepath.Base(f.Name), filepath.Ext(f.Name)))
		for i := range sub.Maps {
			if sub.Maps[i].Name == "" {
				sub.Maps[i].Name = base
			}
		}
		if len(sub.Maps) == 0 {
			sub.Maps = append(sub.Maps, MapInfo{Name: base})
		}
		info.Maps = append(info.Maps, sub.Maps...)
	}
	return info, nil
}

func parseWAD(r io.ReaderAt, size int64) (*Info, error) {
	hdr := make([]byte, 12)
	if _, err := r.ReadAt(hdr, 0); err != nil {
		return nil, errors.New("not a wad file")
	}
	kind := string(hdr[:4])
	if kind != "IWAD" && kind != "PWAD" {
		return nil, errors.New("not a wad file")
	}

	count := int32(binary.LittleEndian.Uint32(hdr[4:]))
	dirOff := int32(binary.LittleEndian.Uint32(hdr[8:]))
	if count < 0 || dirOff < 0 || int64(dirOff)+int64(count)*16 > size {
		return nil, errors.New("wad directory is out of range")
	}

	raw := make([]byte, int(count)*16)
	if _, err := r.ReadAt(raw, int64(dirOff)); err != nil {
		return nil, err
	}
	dir := make([]lump, 0, count)
	for i := int32(0); i < count; i++ {
		p := i * 16
		dir = append(dir, lump{
			name: strings.TrimRight(string(raw[p+8:p+16]), "\x00"),
			pos:  int32(binary.LittleEndian.Uint32(raw[p:])),
			size: int32(binary.LittleEndian.Uint32(raw[p+4:])),
		})
	}

	info := &Info{Kind: kind, Lumps: len(dir)}
	for _, l := range dir {
		if l.name == "PLAYPAL" {
			info.HasPalette = true
			break
		}
	}
	for i, l := range dir {
		if i+1 >= len(dir) {
			break
		}
		if n := dir[i+1].name; n != "THINGS" && n != "TEXTMAP" {
			continue
		}

		// Map format is a per-map property: one wad can mix classic and UDMF.
		var th lump
		step, offset := int32(10), int32(6)
		mi := MapInfo{Name: l.name}
		for j := i + 1; j < len(dir) && mapLumps[dir[j].name]; j++ {
			switch dir[j].name {
			case "THINGS":
				th = dir[j]
			case "TEXTMAP":
				mi.UDMF = true
			case "BEHAVIOR":
				step, offset = 20, 10 // Hexen-format things are 20 bytes
			}
		}
		if mi.UDMF {
			info.Maps = append(info.Maps, mi)
			continue
		}
		if th.size <= 0 || int64(th.pos)+int64(th.size) > size {
			continue
		}

		body := make([]byte, th.size)
		if _, err := r.ReadAt(body, int64(th.pos)); err != nil {
			continue
		}
		for k := range mi.ByName {
			mi.ByName[k] = map[string]int{}
		}
		for j := int32(0); j+step <= th.size; j += step {
			p := j + offset
			if p+4 > th.size {
				break
			}
			typ := int16(binary.LittleEndian.Uint16(body[p:]))
			flags := binary.LittleEndian.Uint16(body[p+2:])
			name, ok := Monsters[typ]
			if !ok || flags&multiOnly != 0 {
				continue
			}
			for idx, bit := range []uint16{SkillEasy, SkillMedium, SkillHard} {
				if flags&bit != 0 {
					mi.Counts[idx]++
					mi.ByName[idx][name]++
				}
			}
		}
		info.Maps = append(info.Maps, mi)
	}
	return info, nil
}

// Era reports which generation of base game a wad's maps belong to, inferred
// from their names: ExMy maps are Doom 1 / Heretic style, MAPxx are Doom 2
// style. Returns "" when the maps are named something else entirely.
func (i *Info) Era() string {
	ep, mp := 0, 0
	for _, m := range i.Maps {
		switch {
		case len(m.Name) == 4 && m.Name[0] == 'E' && m.Name[2] == 'M':
			ep++
		case strings.HasPrefix(m.Name, "MAP"):
			mp++
		}
	}
	switch {
	case ep > mp:
		return "doom1"
	case mp > 0:
		return "doom2"
	}
	return ""
}

// Totals sums monsters across every classic-format map in the file.
func (i *Info) Totals() [3]int {
	var t [3]int
	for _, m := range i.Maps {
		for k := 0; k < 3; k++ {
			t[k] += m.Counts[k]
		}
	}
	return t
}

// UDMFCount reports how many maps were skipped for being text format.
func (i *Info) UDMFCount() int {
	n := 0
	for _, m := range i.Maps {
		if m.UDMF {
			n++
		}
	}
	return n
}

// MapRange describes the map list compactly, e.g. "MAP01-MAP32".
func (i *Info) MapRange() string {
	if len(i.Maps) == 0 {
		return ""
	}
	if len(i.Maps) == 1 {
		return i.Maps[0].Name
	}
	names := make([]string, 0, len(i.Maps))
	for _, m := range i.Maps {
		names = append(names, m.Name)
	}
	sort.Strings(names) // EDAY31 is stored before EDAY01 in some wads
	return names[0] + "-" + names[len(names)-1]
}

// HumanSize renders a byte count the way a file manager would.
func HumanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit && exp < 3; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

// Commas formats a number with thousands separators.
func Commas(n int) string {
	s := fmt.Sprint(n)
	if n < 0 {
		return "-" + Commas(-n)
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}
