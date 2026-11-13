package tesseract

import (
	"bytes"
	"math"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/bitmap"
)

// border is the number of white pixels on each side, because Tesseract finds no text at the image edge.
const border = 10

// pgm draws p on black, scales it bilinearly, adds the white border and returns a binary PGM file.
func pgm(p bitmap.Pixels, scale float64, noInvert bool, threshold int) []byte {
	grey := make([]float64, len(p.RGBA))
	for i, c := range p.RGBA {
		r, g, b, a := float64(c>>24), float64(c>>16&0xFF), float64(c>>8&0xFF), float64(c&0xFF)
		v := (0.299*r + 0.587*g + 0.114*b) * (a / 255)
		if !noInvert {
			v = 255 - v
		}
		grey[i] = v
	}

	w := int(math.Round(float64(p.Width) * scale))
	h := int(math.Round(float64(p.Height) * scale))
	fullW, fullH := w+2*border, h+2*border

	var buf bytes.Buffer
	buf.Grow(fullW*fullH + 24)
	buf.WriteString("P5\n" + strconv.Itoa(fullW) + " " + strconv.Itoa(fullH) + "\n255\n")
	white := bytes.Repeat([]byte{255}, fullW)
	for range border {
		buf.Write(white)
	}
	row := make([]byte, fullW)
	for y := range h {
		sy := min(float64(p.Height-1), max(0, (float64(y)+0.5)/scale-0.5))
		y0 := int(sy)
		y1 := min(p.Height-1, y0+1)
		fy := sy - float64(y0)
		for i := range row {
			row[i] = 255
		}
		for x := range w {
			sx := min(float64(p.Width-1), max(0, (float64(x)+0.5)/scale-0.5))
			x0 := int(sx)
			x1 := min(p.Width-1, x0+1)
			fx := sx - float64(x0)
			top := grey[y0*p.Width+x0]*(1-fx) + grey[y0*p.Width+x1]*fx
			bottom := grey[y1*p.Width+x0]*(1-fx) + grey[y1*p.Width+x1]*fx
			v := top*(1-fy) + bottom*fy
			if threshold > 0 {
				if v < float64(threshold) {
					v = 0
				} else {
					v = 255
				}
			}
			row[border+x] = byte(math.Round(v))
		}
		buf.Write(row)
	}
	for range border {
		buf.Write(white)
	}
	return buf.Bytes()
}
