package bitmap

import (
	"cmp"
	"math"
	"slices"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const maxPalette = 256

type colorCount struct {
	color uint32
	count int
}

func transparent(c uint32) bool { return c&0xFF == 0 }

// Reduce maps pixels to a palette of at most maxColors entries, from 2 to 256.
// With at most maxColors distinct colours, the palette holds them in order of first appearance, with the colours of alpha 0 first.
// With more, a median cut picks the colours and entry 0 is 0x00000000.
func Reduce(pixels []uint32, maxColors int) ([]uint32, []byte, error) {
	if maxColors < 2 || maxColors > maxPalette {
		return nil, nil, &model.InvalidArgumentError{Msg: "The color count must be from 2 to 256, got " + strconv.Itoa(maxColors) + "."}
	}
	colors, position := countColors(pixels)
	entryOf := make([]byte, len(colors))
	var palette []uint32
	if len(colors) <= maxColors {
		palette = firstUseOrder(colors, entryOf)
	} else {
		palette = medianCut(colors, entryOf, maxColors)
	}

	indexes := make([]byte, len(pixels))
	var last uint32
	var lastEntry byte
	for i, c := range pixels {
		if i == 0 || c != last {
			last, lastEntry = c, entryOf[position[c]]
		}
		indexes[i] = lastEntry
	}
	return palette, indexes, nil
}

// countColors returns the distinct colours in order of first appearance with their pixel counts, and the position of each colour in that list.
func countColors(pixels []uint32) ([]colorCount, map[uint32]int) {
	var colors []colorCount
	position := map[uint32]int{}
	var last uint32
	lastAt := -1
	for _, c := range pixels {
		if lastAt < 0 || c != last {
			at, seen := position[c]
			if !seen {
				at = len(colors)
				position[c] = at
				colors = append(colors, colorCount{color: c})
			}
			last, lastAt = c, at
		}
		colors[lastAt].count++
	}
	return colors, position
}

// firstUseOrder returns the colours with those of alpha 0 first, and sets entryOf for each colour.
func firstUseOrder(colors []colorCount, entryOf []byte) []uint32 {
	palette := make([]uint32, 0, len(colors))
	for _, c := range colors {
		if transparent(c.color) {
			palette = append(palette, c.color)
		}
	}
	next := len(palette)
	zeros := 0
	for i, c := range colors {
		if transparent(c.color) {
			entryOf[i] = byte(zeros)
			zeros++
			continue
		}
		entryOf[i] = byte(next)
		palette = append(palette, c.color)
		next++
	}
	return palette
}

type box struct {
	colors  []colorCount
	total   int
	channel uint
	span    int
}

func newBox(colors []colorCount) box {
	b := box{colors: colors}
	lo, hi := [4]int{255, 255, 255, 255}, [4]int{}
	for _, c := range colors {
		b.total += c.count
		for ch := range 4 {
			v := int(c.color >> (24 - 8*uint(ch)) & 0xFF)
			lo[ch], hi[ch] = min(lo[ch], v), max(hi[ch], v)
		}
	}
	for ch := range 4 {
		if hi[ch]-lo[ch] > b.span {
			b.span, b.channel = hi[ch]-lo[ch], uint(24-8*ch)
		}
	}
	if b.span == 0 {
		b.channel = 24
	}
	return b
}

// medianCut sets entryOf for each colour and returns the palette. Entry 0 holds the colours of alpha 0.
func medianCut(colors []colorCount, entryOf []byte, maxColors int) []uint32 {
	opaque := make([]colorCount, 0, len(colors))
	for _, c := range colors {
		if !transparent(c.color) {
			opaque = append(opaque, c)
		}
	}
	boxes := cutBoxes(opaque, maxColors-1)
	palette := make([]uint32, 1, len(boxes)+1)
	entryByColor := make(map[uint32]byte, len(opaque))
	for i, b := range boxes {
		for _, c := range b.colors {
			entryByColor[c.color] = byte(i + 1)
		}
		palette = append(palette, b.mean())
	}
	for i, c := range colors {
		entryOf[i] = entryByColor[c.color]
	}
	return palette
}

// cutBoxes starts with one box of all colours and splits the box with the widest range until there are n boxes or all ranges are 0.
func cutBoxes(colors []colorCount, n int) []box {
	var boxes []box
	if len(colors) > 0 {
		boxes = append(boxes, newBox(colors))
	}
	for len(boxes) < n {
		widest := 0
		for i, b := range boxes {
			if b.span > boxes[widest].span {
				widest = i
			}
		}
		if len(boxes) == 0 || boxes[widest].span == 0 {
			break
		}
		first, second := boxes[widest].split()
		boxes = slices.Insert(boxes, widest+1, second)
		boxes[widest] = first
	}
	return boxes
}

// split sorts the colours of b by its widest channel and cuts them where half of the pixels lie.
func (b box) split() (box, box) {
	shift := b.channel
	slices.SortFunc(b.colors, func(x, y colorCount) int {
		if cx, cy := x.color>>shift&0xFF, y.color>>shift&0xFF; cx != cy {
			return int(cx) - int(cy)
		}
		return cmp.Compare(x.color, y.color)
	})
	at, sum := 1, b.colors[0].count
	for at < len(b.colors)-1 && 2*(sum+b.colors[at].count) <= b.total {
		sum += b.colors[at].count
		at++
	}
	return newBox(b.colors[:at]), newBox(b.colors[at:])
}

// mean returns the colour of b: the count-weighted mean of each channel, rounded half away from zero.
func (b box) mean() uint32 {
	var out uint32
	for shift := uint(24); shift < 32; shift -= 8 {
		var sum int
		for _, c := range b.colors {
			sum += int(c.color>>shift&0xFF) * c.count
		}
		out |= uint32(math.Round(float64(sum)/float64(b.total))) << shift
	}
	return out
}

// ReduceIndexed gives the palette and indexes that Reduce gives for the pixels of all images in order, up to 256 colours.
// The result is false when the images hold more colours.
func ReduceIndexed(images []Indexed) ([]uint32, []byte, bool) {
	order, position, used := firstUses(images)
	if len(order) > maxPalette {
		return nil, nil, false
	}
	colors := make([]colorCount, len(order))
	for i, c := range order {
		colors[i].color = c
	}
	entryOf := make([]byte, len(order))
	palette := firstUseOrder(colors, entryOf)

	var indexes []byte
	for n, im := range images {
		var entry [maxPalette]byte
		for v := range maxPalette {
			if used[n][v] {
				entry[v] = entryOf[position[paletteColor(im, byte(v))]]
			}
		}
		for _, v := range im.Indexes {
			indexes = append(indexes, entry[v])
		}
	}
	return palette, indexes, true
}

// firstUses returns the colours in order of first use, their positions in that order, and the indexes that each image uses.
// It stops one colour past the limit of 256, because the caller only needs to see that the limit is broken.
func firstUses(images []Indexed) ([]uint32, map[uint32]int, [][maxPalette]bool) {
	var order []uint32
	position := map[uint32]int{}
	used := make([][maxPalette]bool, len(images))
	for n, im := range images {
		for _, v := range im.Indexes {
			if used[n][v] {
				continue
			}
			used[n][v] = true
			c := paletteColor(im, v)
			if _, ok := position[c]; ok {
				continue
			}
			position[c] = len(order)
			order = append(order, c)
			if len(order) > maxPalette {
				return order, position, used
			}
		}
	}
	return order, position, used
}

func paletteColor(im Indexed, index byte) uint32 {
	if int(index) < len(im.Palette) {
		return im.Palette[index]
	}
	return 0
}
