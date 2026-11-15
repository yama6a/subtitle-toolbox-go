package arib

import (
	"fmt"
	"unicode/utf8"
)

func (d *Decoder) text(b []byte) {
	for offset := 0; offset < len(b); {
		if d.encoding == encUTF8 {
			offset += d.utf8Code(b, offset)
		} else {
			offset += d.eightUnitCode(b, offset)
		}
	}
}

func (d *Decoder) utf8Code(b []byte, i int) int {
	switch c := b[i]; {
	case c <= 0x1F:
		return d.c0(b, i)
	case c == 0x7F:
		return 1
	case c == 0xC2 && at(b, i+1) >= 0x80 && at(b, i+1) <= 0x9F:
		return 1 + d.c1(b, i+1)
	default:
		return d.utf8(b, i)
	}
}

func (d *Decoder) eightUnitCode(b []byte, i int) int {
	switch c := b[i]; {
	case c <= 0x20:
		return d.c0(b, i)
	case c < 0x7F:
		return d.graphic(b, i, d.sets[d.left])
	case c <= 0xA0:
		return d.c1(b, i)
	case c < 0xFF:
		return d.graphic(b, i, d.sets[d.right])
	default:
		return 1
	}
}

// c0 runs a C0 control function and returns the number of bytes it takes, at least 1.
func (d *Decoder) c0(b []byte, i int) int {
	switch b[i] {
	case 0x16:
		d.move(at(b, i+1)&0x3F, 0)
		return 2
	case 0x19:
		return 1 + d.graphic(b, i+1, d.sets[2])
	case 0x1B:
		return 1 + d.escape(b, i+1)
	case 0x1C:
		d.setPosition(at(b, i+2)&0x3F, at(b, i+1)&0x3F)
		return 3
	case 0x1D:
		return 1 + d.graphic(b, i+1, d.sets[3])
	case 0x20:
		d.space()
		return 1
	}
	d.c0Single(b[i])
	return 1
}

// space writes SP, a full width space in Japanese captions at normal size.
func (d *Decoder) space() {
	if d.encoding == encJapanese && !d.halfWidth() {
		d.put("\u3000")
	} else {
		d.put(" ")
	}
}

// c0Single runs a C0 control function of one byte.
func (d *Decoder) c0Single(c byte) {
	switch c {
	case 0x08:
		d.move(-1, 0)
	case 0x09:
		d.move(1, 0)
	case 0x0A:
		d.move(0, 1)
	case 0x0B:
		d.move(0, -1)
	case 0x0C:
		d.clearScreen()
	case 0x0D:
		d.newline()
	case 0x0E:
		d.left = 1
	case 0x0F:
		d.left = 0
	}
}

// escape runs the ESC sequence that starts at b[i] and returns the number of bytes after ESC.
func (d *Decoder) escape(b []byte, i int) int {
	first := at(b, i)
	switch {
	case first == 0x6E || first == 0x6F:
		d.left = first - 0x6C
	case first >= 0x7C && first <= 0x7E:
		d.right = 0x7F - first
	case first == 0x24:
		return d.designateTwoByte(b, i)
	case first >= 0x28 && first <= 0x2B:
		return d.designate(b, i, first-0x28)
	}
	return 1
}

// designateTwoByte reads the final bytes after ESC 0x24 and sets G0 to G3 to a 2-byte set.
func (d *Decoder) designateTwoByte(b []byte, i int) int {
	second := at(b, i+1)
	if second < 0x28 || second > 0x2B {
		d.sets[0] = second
		return 2
	}
	if at(b, i+2) == 0x20 {
		d.sets[second-0x28] = setDRCS | at(b, i+3)
		return 4
	}
	d.sets[second-0x28] = at(b, i+2)
	return 3
}

// designate reads the final byte after ESC 0x28 to 0x2B and sets G0 to G3 to a 1-byte set.
func (d *Decoder) designate(b []byte, i, index int) int {
	if at(b, i+1) == 0x20 {
		d.sets[index] = setDRCS | at(b, i+2)
		return 3
	}
	d.sets[index] = at(b, i+1)
	return 2
}

// c1 runs a C1 control function and returns the number of bytes it takes, at least 1.
func (d *Decoder) c1(b []byte, i int) int {
	code := b[i]
	switch {
	case code >= 0x80 && code <= 0x87:
		d.color = color(d.palette, int(code)-0x80)
	case code >= 0x88 && code <= 0x8B:
		return d.size(code, at(b, i+1))
	case code >= 0x90 && code <= 0x98:
		return d.c1Parameters(b, i)
	case code == 0x9B:
		return 1 + d.csi(b, i+1)
	case code == 0x9D:
		return d.timeControl(b, i)
	}
	return 1
}

// size runs SSZ, MSZ, NSZ or SZX.
func (d *Decoder) size(code byte, parameter int) int {
	switch code {
	case 0x88:
		d.scaleX, d.scaleY = 0.5, 0.5
	case 0x89:
		d.scaleX, d.scaleY = 0.5, 1
	case 0x8A:
		d.scaleX, d.scaleY = 1, 1
	default:
		if parameter == 0x41 || parameter == 0x45 {
			d.scaleY = 2
		}
		if parameter == 0x44 || parameter == 0x45 {
			d.scaleX = 2
		}
		return 2
	}
	return 1
}

// c1Parameters runs the C1 functions 0x90 to 0x98 that take parameters.
func (d *Decoder) c1Parameters(b []byte, i int) int {
	parameter := at(b, i+1)
	switch b[i] {
	case 0x90:
		return d.setColor(b, i)
	case 0x91, 0x93, 0x94, 0x97:
		return 2
	case 0x92:
		if parameter == 0x20 {
			return 3
		}
		return 2
	case 0x95:
		// A macro definition. TR-B14 does not use them, so the decoder skips it to its end code.
		if end := indexOf(b[i+1:], 0x95, 0x4F); end >= 0 {
			return end + 3
		}
		return len(b) - i
	case 0x98:
		d.repeat = parameter & 0x3F
		if d.repeat == 0 {
			d.repeat = -1
		}
		return 2
	}
	return 1
}

// setColor runs COL: the palette with 0x20, else a color of the palette.
func (d *Decoder) setColor(b []byte, i int) int {
	parameter := at(b, i+1)
	if parameter == 0x20 {
		d.palette = at(b, i+2) & 0x07
		return 3
	}
	if parameter&0xF0 == 0x40 {
		d.color = color(d.palette, parameter&0x0F)
	}
	return 2
}

// timeControl runs TIME. The wait with 0x20 takes a snapshot first, and the other forms are skipped.
func (d *Decoder) timeControl(b []byte, i int) int {
	switch at(b, i+1) {
	case 0x20:
		d.snapshot()
		d.time += int64(at(b, i+2)&0x3F) * tenthSecond
		return 3
	case 0x28:
		return 3
	}
	end := i + 2
	for end < len(b) && (b[end] < 0x40 || b[end] > 0x7E) {
		end++
	}
	return min(end+1, len(b)) - i
}

func indexOf(b []byte, x, y byte) int {
	for i := range len(b) - 1 {
		if b[i] == x && b[i+1] == y {
			return i
		}
	}
	return -1
}

// csi reads a control sequence: decimal parameters split by 0x3B, then 0x20 and the final byte.
func (d *Decoder) csi(b []byte, i int) int {
	parameters, end := csiParameters(b, i)
	if end+1 >= len(b) {
		return len(b) - i
	}
	d.applyCSI(b[end+1], parameters)
	return end + 2 - i
}

// csiParameters returns the numbers of a control sequence and the index of the 0x20 that ends them.
func csiParameters(b []byte, i int) ([]int, int) {
	var parameters []int
	value := 0
	end := i
	for ; end < len(b) && b[end] != ' '; end++ {
		switch c := b[end]; {
		case c == 0x3B:
			parameters = append(parameters, value)
			value = 0
		case c >= 0x30 && c <= 0x39:
			value = min(value*10+int(c)-0x30, maxParameter)
		}
	}
	return append(parameters, value), end
}

func (d *Decoder) applyCSI(final byte, parameters []int) {
	first, second := parameters[0], 0
	if len(parameters) > 1 {
		second = parameters[1]
	}
	switch final {
	case 0x53:
		if len(parameters) == 1 {
			d.format = first
		}
		d.resetFormat()
	case 0x56:
		d.areaWidth, d.areaHeight = first, second
	case 0x57:
		d.charWidth, d.charHeight = first, second
	case 0x58:
		d.spacingX = first
	case 0x59:
		d.spacingY = first
	case 0x5F:
		d.displayPosition(parameters)
	case 0x61:
		d.x, d.y, d.positioned = first, second, true
	}
}

// displayPosition runs SDP. It also positions the cursor at 0, 0 when nothing did before.
func (d *Decoder) displayPosition(parameters []int) {
	d.areaX = parameters[0]
	if len(parameters) > 1 {
		d.areaY = parameters[1]
	}
	if !d.positioned {
		d.setPosition(0, 0)
	}
}

func (d *Decoder) graphic(b []byte, i, set int) int {
	count := 1
	if twoByteSet(set) {
		count = 2
	}
	if i+count > len(b) {
		return len(b) - i
	}
	first, second := int(b[i]&0x7F), 0
	if count == 2 {
		second = int(b[i+1] & 0x7F)
	}
	d.writeGraphic(set, first, second, count)
	return count
}

func (d *Decoder) writeGraphic(set, first, second, count int) {
	switch {
	case set == setMacro:
		d.macro(first)
	case set >= setDRCS:
		code := first
		if count == 2 {
			code = first<<8 | second
		}
		d.put(d.drcs(set<<16 | code))
	case first >= 0x21 && first <= 0x7E && (count == 1 || second >= 0x21 && second <= 0x7E):
		d.put(character(set, first, second, d.halfWidth(), d.encoding == encLatin))
	}
}

func (d *Decoder) macro(code int) {
	if code >= 0x60 && code <= 0x6F {
		d.text([]byte(defaultMacros[code-0x60]))
	}
}

func (d *Decoder) utf8(b []byte, i int) int {
	r, n := utf8.DecodeRune(b[i:])
	switch {
	case r == utf8.RuneError && n == 1:
		d.put(replacement)
		return 1
	case r >= 0xEC00 && r <= 0xF8FF:
		d.put(d.drcs(setDRCS0<<16 | int(r)))
	default:
		d.put(string(r))
	}
	return n
}

func (d *Decoder) drcs(key int) string {
	g, found := d.glyphs[key]
	if found && g.character != "" {
		return g.character
	}
	set := key>>16 - setDRCS0
	code := fmt.Sprintf("0x%02X", key&0xFFFF)
	if set == 0 {
		code = fmt.Sprintf("0x%04X", key&0xFFFF)
	}
	id := fmt.Sprintf("DRCS-%d %s", set, code)
	message := fmt.Sprintf("The character %s of the set DRCS-%d has no glyph in the stream. The reader wrote U+3013 for it.", code, set)
	if found {
		id = g.hash
		message = fmt.Sprintf("The glyph of the character %s of the set DRCS-%d (%s dots, MD5 %s) has no known character. The reader wrote U+3013 for it.",
			code, set, g.size, g.hash)
	}
	if !d.warned[id] {
		d.warned[id] = true
		d.warnings = append(d.warnings, Warning{Time: d.time, Message: message})
	}
	return "\u3013"
}
