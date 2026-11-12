// Package bitmap holds the pixel code that image formats, containers and OCR share.
package bitmap

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const pngSignature = "\x89PNG\r\n\x1a\n"

// Pixels is an RGBA bitmap with straight alpha.
type Pixels struct {
	Width, Height int
	// RGBA holds one 0xRRGGBBAA value per pixel, row by row.
	RGBA []uint32
}

// Indexed is a palette bitmap as a decoder returns it.
type Indexed struct {
	Width, Height int
	// Indexes holds one palette entry per pixel, row by row.
	Indexes []byte
	// Palette holds the 0xRRGGBBAA colours. It may have fewer than 256 entries.
	// An index past the end reads as 0x00000000.
	Palette []uint32
}

// DecodePNG returns the pixels of a PNG with straight alpha.
// It checks the size limit of model.Image before the data inflates.
// Every error is an *model.InvalidArgumentError.
func DecodePNG(data []byte) (Pixels, error) {
	img, err := decodeImage(data)
	if err != nil {
		return Pixels{}, err
	}
	return pixelsOf(img), nil
}

// decodeImage decodes data after the size check. A palette image keeps only the entries of its PLTE chunk,
// because image/png adds opaque black entries up to the highest index in use.
func decodeImage(data []byte) (image.Image, error) {
	cfg, err := checkedConfig(data)
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, decodeError(err.Error())
	}
	if p, ok := img.(*image.Paletted); ok {
		if plte, ok := cfg.ColorModel.(color.Palette); ok && len(plte) < len(p.Palette) {
			p.Palette = p.Palette[:len(plte)]
		}
	}
	return img, nil
}

// DecodeIndexedPNG returns the palette bitmap of a PNG. The result is false when the PNG has no palette.
func DecodeIndexedPNG(data []byte) (Indexed, bool, error) {
	cfg, err := checkedConfig(data)
	if err != nil {
		return Indexed{}, false, err
	}
	if _, ok := cfg.ColorModel.(color.Palette); !ok {
		return Indexed{}, false, nil
	}
	img, err := decodeImage(data)
	if err != nil {
		return Indexed{}, false, err
	}
	p, ok := img.(*image.Paletted)
	if !ok {
		return Indexed{}, false, nil
	}
	w, h := p.Rect.Dx(), p.Rect.Dy()
	ix := Indexed{Width: w, Height: h, Indexes: make([]byte, 0, w*h), Palette: make([]uint32, len(p.Palette))}
	for y := range h {
		ix.Indexes = append(ix.Indexes, p.Pix[y*p.Stride:y*p.Stride+w]...)
	}
	for i, c := range p.Palette {
		ix.Palette[i] = colorValue(c)
	}
	return ix, true, nil
}

func checkedConfig(data []byte) (image.Config, error) {
	if !bytes.HasPrefix(data, []byte(pngSignature)) {
		return image.Config{}, decodeError("the data does not start with the PNG signature.")
	}
	if len(data) >= 24 && string(data[12:16]) == "IHDR" && (binary.BigEndian.Uint32(data[16:20]) == 0 || binary.BigEndian.Uint32(data[20:24]) == 0) {
		return image.Config{}, decodeError("the width and the height must be at least 1.")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return image.Config{}, decodeError(err.Error())
	}
	if cfg.Width > model.MaxImageSide || cfg.Height > model.MaxImageSide || cfg.Width*cfg.Height > model.MaxImagePixels {
		return image.Config{}, decodeError("an image of " + strconv.Itoa(cfg.Width) + "x" + strconv.Itoa(cfg.Height) +
			" pixels is larger than the limit of " + strconv.Itoa(model.MaxImageSide) + " pixels per side and " +
			strconv.Itoa(model.MaxImagePixels) + " pixels in total.")
	}
	return cfg, nil
}

func decodeError(msg string) error {
	if len(msg) > 0 && msg[len(msg)-1] != '.' {
		msg += "."
	}
	return &model.InvalidArgumentError{Msg: "Cannot decode the PNG: " + msg}
}

func pack(r, g, b, a uint8) uint32 {
	return uint32(r)<<24 | uint32(g)<<16 | uint32(b)<<8 | uint32(a)
}

// colorValue returns c as 0xRRGGBBAA. A color.NRGBA and a color.RGBA keep their bytes.
func colorValue(c color.Color) uint32 {
	if c, ok := c.(color.RGBA); ok {
		return pack(c.R, c.G, c.B, c.A)
	}
	n, _ := color.NRGBAModel.Convert(c).(color.NRGBA)
	return pack(n.R, n.G, n.B, n.A)
}

// pixelsOf takes the high byte of each sample. It never goes through color.NRGBAModel,
// which premultiplies and divides again and so changes pixels that are not opaque.
func pixelsOf(img image.Image) Pixels {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := Pixels{Width: w, Height: h, RGBA: make([]uint32, 0, w*h)}
	pix, stride, bytesPerPixel, value := samples(img)
	if value == nil {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				out.RGBA = append(out.RGBA, colorValue(img.At(x, y)))
			}
		}
		return out
	}
	for y := range h {
		for x := range w {
			out.RGBA = append(out.RGBA, value(pix[y*stride+x*bytesPerPixel:]))
		}
	}
	return out
}

// samples returns the sample bytes of img, their row stride and size per pixel, and a function that reads one pixel.
// The function is nil for a type that has no byte layout here.
func samples(img image.Image) (pix []byte, stride, bytesPerPixel int, value func(p []byte) uint32) {
	switch img := img.(type) {
	case *image.NRGBA:
		pix, stride, bytesPerPixel, value = img.Pix, img.Stride, 4, func(p []byte) uint32 { return pack(p[0], p[1], p[2], p[3]) }
	case *image.RGBA:
		pix, stride, bytesPerPixel, value = img.Pix, img.Stride, 4, func(p []byte) uint32 { return pack(p[0], p[1], p[2], 255) }
	case *image.NRGBA64:
		pix, stride, bytesPerPixel, value = img.Pix, img.Stride, 8, func(p []byte) uint32 { return pack(p[0], p[2], p[4], p[6]) }
	case *image.RGBA64:
		pix, stride, bytesPerPixel, value = img.Pix, img.Stride, 8, func(p []byte) uint32 { return pack(p[0], p[2], p[4], 255) }
	case *image.Gray:
		pix, stride, bytesPerPixel, value = img.Pix, img.Stride, 1, func(p []byte) uint32 { return pack(p[0], p[0], p[0], 255) }
	case *image.Gray16:
		pix, stride, bytesPerPixel, value = img.Pix, img.Stride, 2, func(p []byte) uint32 { return pack(p[0], p[0], p[0], 255) }
	case *image.Paletted:
		palette := make([]uint32, len(img.Palette))
		for i, c := range img.Palette {
			palette[i] = colorValue(c)
		}
		pix, stride, bytesPerPixel = img.Pix, img.Stride, 1
		value = func(p []byte) uint32 {
			if int(p[0]) < len(palette) {
				return palette[p[0]]
			}
			return 0
		}
	}
	return pix, stride, bytesPerPixel, value
}
