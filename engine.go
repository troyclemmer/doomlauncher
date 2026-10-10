package main

import (
	"debug/pe"
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

// rtVersion is the resource type holding VS_VERSION_INFO.
const rtVersion = 16

// engineVersion reads a Windows binary's version resource and returns
// something like "UZDoom 4.14.3". Reading the resource avoids running the
// engine just to ask what it is, which would flash a window on startup.
// Returns "" when the binary carries no version information.
func engineVersion(path string) string {
	f, err := pe.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	sec := f.Section(".rsrc")
	if sec == nil {
		return ""
	}
	raw, err := sec.Data()
	if err != nil {
		return ""
	}
	blob := findVersionBlob(raw, sec.VirtualAddress)
	if blob == nil {
		return ""
	}

	fields := versionStrings(blob)
	name := firstOf(fields, "ProductName", "InternalName", "FileDescription")
	ver := firstOf(fields, "ProductVersion", "FileVersion")
	switch {
	case name != "" && ver != "":
		return name + " " + ver
	case ver != "":
		return ver
	}
	return name
}

// findVersionBlob walks the resource tree to the first RT_VERSION leaf.
func findVersionBlob(res []byte, va uint32) []byte {
	entries := func(off uint32) [][3]uint32 {
		var out [][3]uint32
		if int(off)+16 > len(res) {
			return out
		}
		named := binary.LittleEndian.Uint16(res[off+12:])
		ids := binary.LittleEndian.Uint16(res[off+14:])
		for i := 0; i < int(named)+int(ids); i++ {
			p := off + 16 + uint32(i)*8
			if int(p)+8 > len(res) {
				break
			}
			id := binary.LittleEndian.Uint32(res[p:])
			o := binary.LittleEndian.Uint32(res[p+4:])
			var dir uint32
			if o&0x80000000 != 0 {
				dir = 1
			}
			out = append(out, [3]uint32{id, o &^ 0x80000000, dir})
		}
		return out
	}

	for _, t := range entries(0) {
		if t[0] != rtVersion || t[2] == 0 {
			continue
		}
		for _, name := range entries(t[1]) {
			off := name[1]
			if name[2] == 1 {
				langs := entries(name[1])
				if len(langs) == 0 {
					continue
				}
				off = langs[0][1]
			}
			if int(off)+8 > len(res) {
				continue
			}
			rva := binary.LittleEndian.Uint32(res[off:])
			size := binary.LittleEndian.Uint32(res[off+4:])
			start := rva - va
			if int(start)+int(size) > len(res) {
				continue
			}
			return res[start : start+size]
		}
	}
	return nil
}

// versionStrings pulls the StringFileInfo key/value pairs out of the block.
// The structure is padded and nested, but the keys and values are plain
// UTF-16 strings in order, so reading them as a sequence is enough.
func versionStrings(blob []byte) []string {
	u16 := make([]uint16, 0, len(blob)/2)
	for i := 0; i+1 < len(blob); i += 2 {
		u16 = append(u16, binary.LittleEndian.Uint16(blob[i:]))
	}
	text := string(utf16.Decode(u16))

	var out []string
	for _, part := range strings.Split(text, "\x00") {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// firstOf returns the value following the first key that is present.
func firstOf(fields []string, keys ...string) string {
	for _, want := range keys {
		for i, f := range fields {
			// Keys carry leading padding bytes, so match on the suffix.
			if strings.HasSuffix(f, want) && i+1 < len(fields) {
				return strings.TrimSpace(fields[i+1])
			}
		}
	}
	return ""
}
