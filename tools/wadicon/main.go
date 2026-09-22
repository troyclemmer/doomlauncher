// Command wadicon builds a .ico from a sprite inside a Doom IWAD, so the
// launcher gets an authentically Doom icon instead of the engine's own logo.
//
//	go run ./tools/wadicon DOOM2.WAD STFST01 doom.ico
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

var iconSizes = []int{16, 24, 32, 48, 64, 128, 256}

type lump struct {
	pos, size int32
	name      string
}

func readWAD(path string) ([]byte, map[string]lump) {
	data, err := os.ReadFile(path)
	check(err)
	if len(data) < 12 {
		fatal("not a WAD")
	}
	count := int32(binary.LittleEndian.Uint32(data[4:]))
	dirOff := int32(binary.LittleEndian.Uint32(data[8:]))

	lumps := map[string]lump{}
	for i := int32(0); i < count; i++ {
		p := dirOff + i*16
		if int(p)+16 > len(data) {
			break
		}
		name := string(bytes.TrimRight(data[p+8:p+16], "\x00"))
		lumps[name] = lump{
			pos:  int32(binary.LittleEndian.Uint32(data[p:])),
			size: int32(binary.LittleEndian.Uint32(data[p+4:])),
			name: name,
		}
	}
	return data, lumps
}

// decodePatch turns Doom's column-major picture format into RGBA. Pixels not
// covered by any post stay transparent.
func decodePatch(b []byte, pal color.Palette) *image.RGBA {
	w := int(int16(binary.LittleEndian.Uint16(b[0:])))
	h := int(int16(binary.LittleEndian.Uint16(b[2:])))
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	for x := 0; x < w; x++ {
		off := int(binary.LittleEndian.Uint32(b[8+x*4:]))
		for off < len(b) {
			top := b[off]
			if top == 0xFF {
				break
			}
			length := int(b[off+1])
			start := off + 3 // one padding byte before the run
			for i := 0; i < length && start+i < len(b); i++ {
				y := int(top) + i
				if y >= 0 && y < h {
					img.Set(x, y, pal[b[start+i]])
				}
			}
			off = start + length + 1 // and one after
		}
	}
	return img
}

// sample scales src into a dst-sized box: nearest-neighbour when enlarging, so
// the pixel art stays crisp, box-averaged when shrinking so it stays legible.
func sample(src *image.RGBA, dstW, dstH int) *image.RGBA {
	sb := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	xs := float64(sb.Dx()) / float64(dstW)
	ys := float64(sb.Dy()) / float64(dstH)

	for y := 0; y < dstH; y++ {
		for x := 0; x < dstW; x++ {
			x0, x1 := int(float64(x)*xs), int(math.Ceil(float64(x+1)*xs))
			y0, y1 := int(float64(y)*ys), int(math.Ceil(float64(y+1)*ys))
			if x1 <= x0 {
				x1 = x0 + 1
			}
			if y1 <= y0 {
				y1 = y0 + 1
			}
			var r, g, bb, a, n float64
			for sy := y0; sy < y1 && sy < sb.Dy(); sy++ {
				for sx := x0; sx < x1 && sx < sb.Dx(); sx++ {
					c := src.RGBAAt(sx, sy)
					// premultiply so transparent pixels do not darken edges
					af := float64(c.A) / 255
					r += float64(c.R) * af
					g += float64(c.G) * af
					bb += float64(c.B) * af
					a += float64(c.A)
					n++
				}
			}
			if n == 0 {
				continue
			}
			alpha := a / n
			if alpha == 0 {
				continue
			}
			scale := 255 / alpha
			dst.SetRGBA(x, y, color.RGBA{
				R: clamp(r / n * scale), G: clamp(g / n * scale),
				B: clamp(bb / n * scale), A: clamp(alpha),
			})
		}
	}
	return dst
}

func clamp(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v + 0.5)
}

// render composes one icon size: a dark rounded backdrop with the sprite on top.
func render(face *image.RGBA, size int) *image.RGBA {
	const ss = 4 // supersample the backdrop so the rounded corners are smooth
	big := size * ss
	back := image.NewRGBA(image.Rect(0, 0, big, big))
	radius := float64(big) / 5

	for y := 0; y < big; y++ {
		// vertical gradient, dried-blood red into near black
		t := float64(y) / float64(big)
		col := color.RGBA{
			R: clamp(74 - 52*t), G: clamp(20 - 14*t), B: clamp(18 - 13*t), A: 255,
		}
		for x := 0; x < big; x++ {
			if insideRounded(float64(x), float64(y), float64(big), radius) {
				back.SetRGBA(x, y, col)
			}
		}
	}
	out := sample(back, size, size)

	// Fit the sprite into the middle, preserving its aspect ratio.
	fb := face.Bounds()
	target := int(float64(size) * 0.80)
	fw := target * fb.Dx() / fb.Dy()
	if fw > target {
		fw = target
		target = fw * fb.Dy() / fb.Dx()
	}
	scaled := sample(face, fw, target)
	ox, oy := (size-fw)/2, (size-target)/2

	for y := 0; y < target; y++ {
		for x := 0; x < fw; x++ {
			c := scaled.RGBAAt(x, y)
			if c.A == 0 {
				continue
			}
			dx, dy := ox+x, oy+y
			if dx < 0 || dy < 0 || dx >= size || dy >= size {
				continue
			}
			bg := out.RGBAAt(dx, dy)
			a := float64(c.A) / 255
			out.SetRGBA(dx, dy, color.RGBA{
				R: clamp(float64(c.R)*a + float64(bg.R)*(1-a)),
				G: clamp(float64(c.G)*a + float64(bg.G)*(1-a)),
				B: clamp(float64(c.B)*a + float64(bg.B)*(1-a)),
				A: clamp(float64(c.A) + float64(bg.A)*(1-a)),
			})
		}
	}
	return out
}

func insideRounded(x, y, size, r float64) bool {
	cx, cy := x, y
	switch {
	case x < r && y < r:
		cx, cy = r, r
	case x > size-r && y < r:
		cx, cy = size-r, r
	case x < r && y > size-r:
		cx, cy = r, size-r
	case x > size-r && y > size-r:
		cx, cy = size-r, size-r
	default:
		return true
	}
	return math.Hypot(x-cx, y-cy) <= r
}

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: wadicon <iwad> <lump> <out.ico>")
		os.Exit(2)
	}
	data, lumps := readWAD(os.Args[1])

	pl, ok := lumps["PLAYPAL"]
	if !ok {
		fatal("no PLAYPAL in this WAD")
	}
	pal := make(color.Palette, 256)
	for i := 0; i < 256; i++ {
		p := pl.pos + int32(i*3)
		pal[i] = color.RGBA{data[p], data[p+1], data[p+2], 255}
	}

	sp, ok := lumps[os.Args[2]]
	if !ok {
		fatal("lump " + os.Args[2] + " not found")
	}
	face := decodePatch(data[sp.pos:sp.pos+sp.size], pal)
	fmt.Printf("%s is %dx%d\n", os.Args[2], face.Bounds().Dx(), face.Bounds().Dy())

	var dir, blob bytes.Buffer
	binary.Write(&dir, binary.LittleEndian, [3]uint16{0, 1, uint16(len(iconSizes))})
	offset := uint32(6 + len(iconSizes)*16)

	for _, s := range iconSizes {
		var buf bytes.Buffer
		check(png.Encode(&buf, render(face, s)))

		dim := byte(s)
		if s >= 256 {
			dim = 0 // 0 means 256 in the ICO header
		}
		dir.Write([]byte{dim, dim, 0, 0})
		binary.Write(&dir, binary.LittleEndian, uint16(1))  // planes
		binary.Write(&dir, binary.LittleEndian, uint16(32)) // bpp
		binary.Write(&dir, binary.LittleEndian, uint32(buf.Len()))
		binary.Write(&dir, binary.LittleEndian, offset)
		offset += uint32(buf.Len())
		blob.Write(buf.Bytes())
	}

	check(os.WriteFile(os.Args[3], append(dir.Bytes(), blob.Bytes()...), 0o644))
	fmt.Printf("wrote %s: %d sizes, %d bytes\n", os.Args[3], len(iconSizes), 6+len(iconSizes)*16+blob.Len())
}

func check(err error) {
	if err != nil {
		fatal(err.Error())
	}
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "wadicon:", msg)
	os.Exit(1)
}
