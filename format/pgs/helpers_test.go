package pgs_test

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/pgs"
	"github.com/yama6a/subtitle-toolbox-go/v3/imaging"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// The palette entry ids of the test palette.
const (
	transparent = iota
	white
	black
	yellow
	gray
	halfWhite
)

// yccEntry is the Y, Cr, Cb code and the alpha of a palette entry.
type yccEntry [4]byte

// testPalette holds the palette of the PHP fixtures, plus a half transparent white. Yellow is 255, 255, 0 in BT.709.
var testPalette = map[byte]yccEntry{
	transparent: {16, 128, 128, 0},
	white:       {235, 128, 128, 255},
	black:       {16, 128, 128, 255},
	yellow:      {219, 138, 16, 255},
	gray:        {126, 128, 128, 128},
	halfWhite:   {235, 128, 128, 128},
}

// The colours of the test palette after the conversion, 0xRRGGBBAA.
const (
	rgbaWhite       = 0xFFFFFFFF
	rgbaBlack       = 0x000000FF
	rgbaGray        = 0x80808080
	rgbaYellow709   = 0xFEFF00FF
	rgbaYellow601   = 0xFCFF0AFF
	stateEpochStart = 0x80
	stateNormal     = 0x00
)

// obj is a composition object of a presentation segment.
type obj struct {
	id      uint16
	window  byte
	x, y    int
	forced  bool
	crop    *[4]int
	pixels  string
	w, h    int
	version byte
}

// builder writes the segments of a PGS stream, as the fixture writer of the PHP library does.
type builder struct{ bytes.Buffer }

func (b *builder) segment(pts uint32, typ byte, data []byte) *builder {
	var h [13]byte
	h[0], h[1] = 'P', 'G'
	binary.BigEndian.PutUint32(h[2:], pts)
	h[10] = typ
	binary.BigEndian.PutUint16(h[11:], uint16(len(data)))
	b.Write(h[:])
	b.Write(data)
	return b
}

func u16(v ...int) []byte {
	out := make([]byte, 0, 2*len(v))
	for _, x := range v {
		out = binary.BigEndian.AppendUint16(out, uint16(x))
	}
	return out
}

func (b *builder) presentation(pts uint32, w, h int, state, paletteUpdate byte, objects ...obj) *builder {
	data := append(u16(w, h), 0x10, 0, 1, state, paletteUpdate, 0, byte(len(objects)))
	for _, o := range objects {
		flags := byte(0)
		if o.crop != nil {
			flags |= 0x80
		}
		if o.forced {
			flags |= 0x40
		}
		data = append(data, u16(int(o.id))...)
		data = append(data, o.window, flags)
		data = append(data, u16(o.x, o.y)...)
		if o.crop != nil {
			data = append(data, u16(o.crop[:]...)...)
		}
	}
	return b.segment(pts, 0x16, data)
}

func (b *builder) windows(pts uint32, ws map[byte][4]int) *builder {
	data := []byte{byte(len(ws))}
	for id, w := range ws {
		data = append(append(data, id), u16(w[:]...)...)
	}
	return b.segment(pts, 0x17, data)
}

func (b *builder) palette(pts uint32, version byte, entries map[byte]yccEntry) *builder {
	data := []byte{0, version}
	for id := range 256 {
		if e, ok := entries[byte(id)]; ok {
			data = append(append(data, byte(id)), e[:]...)
		}
	}
	return b.segment(pts, 0x14, data)
}

// object writes the object o as one ODS segment per fragmentSize bytes of run-length data.
func (b *builder) object(pts uint32, o obj, fragmentSize int) *builder {
	rle := encodeRLE([]byte(o.pixels), o.w)
	for first, rest := true, rle; first || len(rest) > 0; first = false {
		n := min(fragmentSize, len(rest))
		seq := byte(0)
		if first {
			seq |= 0x80
		}
		if n == len(rest) {
			seq |= 0x40
		}
		data := append(u16(int(o.id)), o.version, seq)
		if first {
			length := len(rle) + 4
			data = append(data, byte(length>>16), byte(length>>8), byte(length))
			data = append(data, u16(o.w, o.h)...)
		}
		b.segment(pts, 0x15, append(data, rest[:n]...))
		rest = rest[n:]
	}
	return b
}

func (b *builder) end(pts uint32) *builder { return b.segment(pts, 0x80, nil) }

// encodeRLE encodes rows of palette entry bytes with every code of the PGS run-length table.
func encodeRLE(pixels []byte, width int) []byte {
	var rle []byte
	for row := 0; row+width <= len(pixels); row += width {
		line := pixels[row : row+width]
		for pos := 0; pos < width; {
			c, run := line[pos], 1
			for pos+run < width && line[pos+run] == c && run < 0x3FFF {
				run++
			}
			switch {
			case c != 0 && run <= 2:
				for range run {
					rle = append(rle, c)
				}
			case c == 0 && run < 64:
				rle = append(rle, 0, byte(run))
			case c == 0:
				rle = append(rle, 0, 0x40|byte(run>>8), byte(run))
			case run < 64:
				rle = append(rle, 0, 0x80|byte(run), c)
			default:
				rle = append(rle, 0, 0xC0|byte(run>>8), byte(run), c)
			}
			pos += run
		}
		rle = append(rle, 0, 0)
	}
	return rle
}

// shows builds the stream of one display set at pts that shows objects.
func (b *builder) shows(pts uint32, w, h int, objects ...obj) *builder {
	b.presentation(pts, w, h, stateEpochStart, 0, objects...)
	b.palette(pts, 0, testPalette)
	for _, o := range objects {
		if o.pixels != "" {
			b.object(pts, o, 65000)
		}
	}
	return b.end(pts)
}

// clears builds the display set at pts that shows nothing.
func (b *builder) clears(pts uint32, w, h int) *builder {
	return b.presentation(pts, w, h, stateNormal, 0).end(pts)
}

func ticks(s float64) uint32 { return uint32(s * 90000) }

func read(t *testing.T, content []byte, common model.CommonReadOptions) (*model.Subtitle, []model.ParseWarning) {
	t.Helper()

	s, warnings, err := pgs.Read(bytes.NewReader(content), common, pgs.ReadOptions{})
	require.NoError(t, err)
	return s, warnings
}

// pixel returns the 0xRRGGBBAA colour at x, y of the image of c.
func pixel(t *testing.T, c model.Cue, x, y int) uint32 {
	t.Helper()

	img, err := imaging.Decode(*c.Image)
	require.NoError(t, err)
	p := img.NRGBAAt(x, y)
	return uint32(p.R)<<24 | uint32(p.G)<<16 | uint32(p.B)<<8 | uint32(p.A)
}

func seconds(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }
