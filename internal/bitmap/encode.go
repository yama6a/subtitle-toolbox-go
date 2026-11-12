package bitmap

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

func nrgba(c uint32) color.NRGBA {
	return color.NRGBA{R: uint8(c >> 24), G: uint8(c >> 16), B: uint8(c >> 8), A: uint8(c)}
}

// EncodeRGBA returns p as an 8-bit RGBA PNG. image/png writes RGB when every pixel is opaque.
// Equal pixels give equal bytes. A bitmap of size 0 gives nil.
func EncodeRGBA(p Pixels) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, p.Width, p.Height))
	for i, c := range p.RGBA {
		n := nrgba(c)
		img.Pix[4*i], img.Pix[4*i+1], img.Pix[4*i+2], img.Pix[4*i+3] = n.R, n.G, n.B, n.A
	}
	return encode(img)
}

// EncodeIndexed returns ix as a palette PNG. An index past the palette reads as 0x00000000 in the result too.
// A bitmap of size 0 gives nil.
func EncodeIndexed(ix Indexed) []byte {
	img := image.NewPaletted(image.Rect(0, 0, ix.Width, ix.Height), make(color.Palette, 0, len(ix.Palette)))
	for _, c := range ix.Palette {
		img.Palette = append(img.Palette, nrgba(c))
	}
	for len(img.Palette) == 0 || len(img.Palette) <= int(maxIndex(ix.Indexes)) {
		img.Palette = append(img.Palette, color.NRGBA{})
	}
	copy(img.Pix, ix.Indexes)
	return encode(img)
}

func maxIndex(indexes []byte) byte {
	var m byte
	for _, v := range indexes {
		m = max(m, v)
	}
	return m
}

func encode(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}
