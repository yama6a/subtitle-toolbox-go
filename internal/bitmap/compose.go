package bitmap

import (
	"slices"
	"strconv"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Part is one indexed bitmap and its place on the screen.
type Part struct {
	X, Y          int
	Width, Height int
	// Indexes holds one palette entry per pixel, row by row.
	Indexes []byte
	// Palette holds the 0xRRGGBBAA colour of each entry.
	Palette [256]uint32
}

func (p Part) overlaps(q Part) bool {
	return p.X < q.X+q.Width && q.X < p.X+p.Width && p.Y < q.Y+q.Height && q.Y < p.Y+p.Height
}

// ImageOf joins the parts into one image. One part gives a palette PNG, 2 or more an RGBA PNG.
// The error is a *model.ParseError.
func ImageOf(parts []Part, screenWidth, screenHeight int) (model.Image, error) {
	if err := checkParts(parts); err != nil {
		return model.Image{}, err
	}
	im := model.Image{ScreenWidth: screenWidth, ScreenHeight: screenHeight}
	if len(parts) == 1 {
		im.PNG, im.X, im.Y, im.Width, im.Height = indexedPNG(parts[0]), parts[0].X, parts[0].Y, parts[0].Width, parts[0].Height
		return im, nil
	}

	left, top, width, height := canvasOf(parts)
	if reason := SizeLimit(width, height); reason != "" {
		return model.Image{}, joinError(reason)
	}
	canvas := Pixels{Width: width, Height: height, RGBA: make([]uint32, width*height)}
	for n, p := range parts {
		draw(canvas, p, p.X-left, p.Y-top, slices.ContainsFunc(parts[:n], p.overlaps))
	}
	im.PNG = EncodeRGBA(canvas)
	im.X, im.Y, im.Width, im.Height = left, top, width, height
	return im, nil
}

func checkParts(parts []Part) error {
	if len(parts) == 0 {
		return joinError("there is no part.")
	}
	for _, p := range parts {
		if p.Width < 1 || p.Height < 1 || len(p.Indexes) != p.Width*p.Height {
			return joinError("a part of " + strconv.Itoa(p.Width) + "x" + strconv.Itoa(p.Height) +
				" pixels holds " + strconv.Itoa(len(p.Indexes)) + " indexes.")
		}
	}
	return nil
}

// indexedPNG returns the palette PNG of p with only the colours that p uses, in order of first use.
func indexedPNG(p Part) []byte {
	palette, indexes, _ := ReduceIndexed([]Indexed{{Width: p.Width, Height: p.Height, Indexes: p.Indexes, Palette: p.Palette[:]}})
	return EncodeIndexed(Indexed{Width: p.Width, Height: p.Height, Indexes: indexes, Palette: palette})
}

// canvasOf returns the corner and size of the smallest rectangle that holds all parts.
func canvasOf(parts []Part) (left, top, width, height int) {
	left, top = parts[0].X, parts[0].Y
	right, bottom := left+parts[0].Width, top+parts[0].Height
	for _, p := range parts[1:] {
		left, top = min(left, p.X), min(top, p.Y)
		right, bottom = max(right, p.X+p.Width), max(bottom, p.Y+p.Height)
	}
	return left, top, right - left, bottom - top
}

// draw puts p on the canvas at x, y. A part that overlaps an earlier part blends with it, else it replaces the pixels below.
func draw(canvas Pixels, p Part, x, y int, blend bool) {
	for row := range p.Height {
		line := canvas.RGBA[(y+row)*canvas.Width+x:][:p.Width]
		for col, v := range p.Indexes[row*p.Width : (row+1)*p.Width] {
			if src := p.Palette[v]; blend {
				line[col] = over(src, line[col])
			} else {
				line[col] = src
			}
		}
	}
}

func joinError(msg string) error {
	return &model.ParseError{Msg: "Cannot join the parts of one display set: " + msg}
}

// over draws src over below with the straight-alpha "source over" operator, in integers.
func over(src, below uint32) uint32 {
	as, ab := int(src&0xFF), int(below&0xFF)
	if as == 255 || ab == 0 {
		return src
	}
	if as == 0 {
		return below
	}
	belowWeight := ab * (255 - as)
	a := as*255 + belowWeight
	var out uint32
	for shift := uint(24); shift >= 8; shift -= 8 {
		cs, cb := int(src>>shift&0xFF), int(below>>shift&0xFF)
		out |= uint32((2*(cs*as*255+cb*belowWeight)+a)/(2*a)) << shift
	}
	return out | uint32((2*a+255)/510)
}

// Cue returns a cue that shows im from start to end. The image is on the top of the screen when its centre lies in the top third.
func Cue(start, end time.Duration, im model.Image, forced bool) model.Cue {
	c := model.Cue{Start: start, End: end, Image: &im, Forced: forced}.Clone()
	if (2*im.Y+im.Height)*3 < 2*im.ScreenHeight {
		c.Position.Alignment = model.TopCenter
	}
	return c
}
