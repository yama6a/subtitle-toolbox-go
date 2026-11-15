package arib

import (
	"fmt"
	"math"
)

func (d *Decoder) put(text string) {
	if text == "" {
		return
	}
	if !d.positioned {
		d.setPosition(0, 0)
	}
	ruby := d.encoding == encJapanese &&
		(d.scaleX == 0.5 && d.scaleY == 0.5 || d.charWidth == 18 && d.charHeight == 18)
	for range d.takeRepeat() {
		d.store(cell{d.x, d.y - d.sectionHeight(), d.sectionWidth(), d.sectionHeight(), ruby, d.color, text})
		d.move(1, 0)
	}
}

// takeRepeat returns how many cells the next character fills, and ends the repeat.
func (d *Decoder) takeRepeat() int {
	times := 1
	switch {
	case d.repeat > 0:
		times = d.repeat
	case d.repeat < 0:
		times = min(maxRepeat, max(1, (d.areaX+d.areaWidth-d.x)/max(1, d.sectionWidth())))
	}
	d.repeat = 0
	return times
}

// store puts c at its cell. A character past maxCells is dropped with one warning.
func (d *Decoder) store(c cell) {
	key := position{c.x, c.y}
	if i, ok := d.cellAt[key]; ok {
		d.cells[i] = c
		return
	}
	if len(d.cells) >= maxCells {
		d.skip("cells", fmt.Sprintf("The caption screen holds more than %d characters. The reader dropped the characters past that.", maxCells))
		return
	}
	d.cellAt[key] = len(d.cells)
	d.cells = append(d.cells, c)
}

func (d *Decoder) halfWidth() bool { return d.scaleX*2 == d.scaleY }

func (d *Decoder) sectionWidth() int {
	return int(math.Floor(float64(d.charWidth+d.spacingX) * d.scaleX))
}

func (d *Decoder) sectionHeight() int {
	return int(math.Floor(float64(d.charHeight+d.spacingY) * d.scaleY))
}

func (d *Decoder) setPosition(column, row int) {
	d.x = d.areaX + column*d.sectionWidth()
	d.y = d.areaY + (row+1)*d.sectionHeight()
	d.positioned = true
}

func (d *Decoder) newline() {
	if !d.positioned {
		d.setPosition(0, 0)
	}
	d.x = d.areaX
	d.y += d.sectionHeight()
}

// move moves the active position by whole cells. It wraps at the edges of the display area, as STD-B24 section 7.2.5 says.
func (d *Decoder) move(columns, rows int) {
	if !d.positioned {
		d.setPosition(0, 0)
	}
	width := max(1, d.sectionWidth())
	height := max(1, d.sectionHeight())
	for ; columns < 0; columns++ {
		d.x -= width
		if d.x < d.areaX {
			d.x = d.areaX + d.areaWidth - width
			rows--
		}
	}
	for ; columns > 0; columns-- {
		d.x += width
		if d.x >= d.areaX+d.areaWidth {
			d.x = d.areaX
			rows++
		}
	}
	for ; rows < 0; rows++ {
		d.y -= height
		if d.y < d.areaY {
			d.y = d.areaY + d.areaHeight
		}
	}
	for ; rows > 0; rows-- {
		d.y += height
		if d.y > d.areaY+d.areaHeight {
			d.y = d.areaY + height
		}
	}
}

// clearScreen clears the screen and resets the decoder, as the CS control does.
func (d *Decoder) clearScreen() {
	d.cells = d.cells[:0]
	clear(d.cellAt)
	d.reset()
}

// reset returns to the state after a clear screen: the default sets, writing format, size and color.
func (d *Decoder) reset() {
	d.resetSets()
	d.resetFormat()
	d.areaX, d.areaY, d.x, d.y, d.positioned = 0, 0, 0, 0, false
	// Latin captions are middle size by default, Japanese captions normal size.
	d.scaleX, d.scaleY = 1, 1
	if d.encoding == encLatin {
		d.scaleX = 0.5
	}
	d.palette, d.color, d.repeat = 0, white, 0
}

func (d *Decoder) resetSets() {
	d.sets = [4]int{setKanji, setAlphanumeric, setHiragana, setMacro}
	if d.encoding == encLatin {
		d.sets = [4]int{setAlphanumeric, setAlphanumeric, setLatinExtension, setLatinSpecial}
	}
	d.left, d.right = 0, 2
}

// resetFormat sets the plane, the display area and the character size of the writing format of SWF, as ARIB TR-B14
// profile A gives them.
func (d *Decoder) resetFormat() {
	width, height, spacingX, spacingY := 960, 540, 4, 24
	switch d.format {
	case 5:
		width, height = 1920, 1080
	case 8:
		spacingX = 12
	case 9:
		width, height, spacingY = 720, 480, 16
	case 10:
		width, height, spacingX = 720, 480, 8
	}
	d.planeWidth, d.planeHeight, d.areaWidth, d.areaHeight = width, height, width, height
	d.charWidth, d.charHeight = 36, 36
	d.spacingX, d.spacingY = spacingX, spacingY
	if d.encoding == encLatin {
		d.spacingX, d.spacingY = 2, 16
	}
}
