// Command mkicon pulls the icon out of a Windows PE binary and writes a .ico
// file. Used once, to lift the engine's own Doom icon onto the launcher exe.
//
//	go run ./tools/mkicon uzdoom.exe doom.ico
package main

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"os"
)

const (
	rtIcon      = 3
	rtGroupIcon = 14
)

type resSection struct {
	data []byte
	va   uint32
}

// entries walks one level of the resource tree at the given offset.
func (r resSection) entries(off uint32) []struct {
	ID    uint32
	Off   uint32
	IsDir bool
} {
	var out []struct {
		ID    uint32
		Off   uint32
		IsDir bool
	}
	if int(off)+16 > len(r.data) {
		return out
	}
	named := binary.LittleEndian.Uint16(r.data[off+12:])
	ids := binary.LittleEndian.Uint16(r.data[off+14:])
	base := off + 16
	for i := 0; i < int(named)+int(ids); i++ {
		p := base + uint32(i)*8
		if int(p)+8 > len(r.data) {
			break
		}
		id := binary.LittleEndian.Uint32(r.data[p:])
		o := binary.LittleEndian.Uint32(r.data[p+4:])
		out = append(out, struct {
			ID    uint32
			Off   uint32
			IsDir bool
		}{id, o &^ 0x80000000, o&0x80000000 != 0})
	}
	return out
}

// leaf resolves a resource data entry to the bytes it points at.
func (r resSection) leaf(off uint32) []byte {
	if int(off)+8 > len(r.data) {
		return nil
	}
	rva := binary.LittleEndian.Uint32(r.data[off:])
	size := binary.LittleEndian.Uint32(r.data[off+4:])
	start := rva - r.va
	if int(start)+int(size) > len(r.data) {
		return nil
	}
	return r.data[start : start+size]
}

// collect returns id -> bytes for every resource of the given type.
func (r resSection) collect(resType uint32) map[uint32][]byte {
	out := map[uint32][]byte{}
	for _, t := range r.entries(0) {
		if t.ID != resType || !t.IsDir {
			continue
		}
		for _, name := range r.entries(t.Off) {
			off := name.Off
			if name.IsDir {
				langs := r.entries(name.Off)
				if len(langs) == 0 {
					continue
				}
				off = langs[0].Off
			}
			if b := r.leaf(off); b != nil {
				out[name.ID] = b
			}
		}
	}
	return out
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: mkicon <source.exe> <out.ico>")
		os.Exit(2)
	}

	f, err := pe.Open(os.Args[1])
	check(err)
	defer f.Close()

	sec := f.Section(".rsrc")
	if sec == nil {
		fatal("no .rsrc section")
	}
	raw, err := sec.Data()
	check(err)
	r := resSection{data: raw, va: sec.VirtualAddress}

	groups := r.collect(rtGroupIcon)
	icons := r.collect(rtIcon)
	if len(groups) == 0 || len(icons) == 0 {
		fatal("no icon resources found")
	}

	// Pick the group with the most images - that is the real app icon.
	var group []byte
	for _, g := range groups {
		if len(g) > len(group) {
			group = g
		}
	}

	count := int(binary.LittleEndian.Uint16(group[4:]))
	var dir bytes.Buffer
	var blob bytes.Buffer
	offset := uint32(6 + count*16)

	dir.Write(group[:6]) // reserved, type, count carry over unchanged
	for i := 0; i < count; i++ {
		e := group[6+i*14 : 6+(i+1)*14]
		id := uint32(binary.LittleEndian.Uint16(e[12:]))
		img, ok := icons[id]
		if !ok {
			fatal(fmt.Sprintf("icon id %d referenced but missing", id))
		}
		dir.Write(e[:12]) // width/height/colors/planes/bitcount/size
		binary.Write(&dir, binary.LittleEndian, offset)
		blob.Write(img)
		offset += uint32(len(img))
	}

	out := append(dir.Bytes(), blob.Bytes()...)
	check(os.WriteFile(os.Args[2], out, 0o644))
	fmt.Printf("wrote %s: %d images, %d bytes\n", os.Args[2], count, len(out))
}

func check(err error) {
	if err != nil {
		fatal(err.Error())
	}
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "mkicon:", msg)
	os.Exit(1)
}
