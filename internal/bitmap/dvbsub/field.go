package dvbsub

const (
	endOfLine = 0xF0
	map2to4   = 0x20
	map2to8   = 0x21
	map4to8   = 0x22
)

// maps holds the map tables of one field. A table sends a shallow pixel code to the code of a deeper region.
type maps struct {
	to4      []byte // 2-bit codes in a 4-bit region
	to8      []byte // 2-bit codes in an 8-bit region
	to8From4 []byte // 4-bit codes in an 8-bit region
}

func defaultMaps() maps {
	m := maps{to4: []byte{0x0, 0x7, 0x8, 0xF}, to8: []byte{0x00, 0x77, 0x88, 0xFF}, to8From4: make([]byte, 16)}
	for i := range m.to8From4 {
		m.to8From4[i] = byte(i * 0x11)
	}
	return m
}

// read replaces the table of the type typ with the bytes of data and returns the count of bytes it used.
func (m *maps) read(typ byte, data []byte) int {
	switch typ {
	case map2to4:
		n := min(2, len(data))
		m.to4 = nibbles(data[:n])
		return n
	case map2to8:
		n := min(4, len(data))
		m.to8 = data[:n]
		return n
	}
	n := min(16, len(data))
	m.to8From4 = data[:n]
	return n
}

// table returns the table that maps codes of bits to a region of depth bits, nil when no mapping applies.
func (m *maps) table(bits, depth int) []byte {
	switch {
	case bits == depth:
		return nil
	case bits == 4:
		return m.to8From4
	case depth == 4:
		return m.to4
	}
	return m.to8
}

func lookup(table []byte, code byte) byte {
	if int(code) < len(table) {
		return table[code]
	}
	return code
}

// drawField draws the pixel code strings of one field of an object into every second row of r, from row y.
// With nonModifying set, the pixel code 1 leaves the region pixels as they are.
func (r *region) drawField(data []byte, x, y int, nonModifying bool) {
	m := defaultMaps()
	column := x
	for offset := 0; offset < len(data); {
		typ := data[offset]
		offset++
		switch typ {
		case endOfLine:
			column, y = x, y+2
		case map2to4, map2to8, map4to8:
			offset += m.read(typ, data[offset:])
		case 0x10, 0x11, 0x12:
			bits := 2 << (typ - 0x10)
			if bits > r.depth {
				return
			}
			runs, next := pixelCodes(data, offset, bits, r.width-column)
			offset = next
			column = r.drawRuns(runs, m.table(bits, r.depth), column, y, nonModifying)
		}
	}
}

// drawRuns draws the runs from column in row y and returns the column after them.
func (r *region) drawRuns(runs []run, table []byte, column, y int, nonModifying bool) int {
	for _, rn := range runs {
		if (rn.code != 1 || !nonModifying) && y < r.height && column < r.width {
			v := rn.code
			if table != nil {
				v = lookup(table, rn.code)
			}
			row := r.pixels[y*r.width+column:][:min(rn.count, r.width-column)]
			for i := range row {
				row[i] = v
			}
		}
		column += rn.count
	}
	return column
}

func nibbles(b []byte) []byte {
	out := make([]byte, 0, 2*len(b))
	for _, v := range b {
		out = append(out, v>>4, v&0x0F)
	}
	return out
}

// run is count pixels of one pixel code.
type run struct {
	code  byte
	count int
}

// bitReader reads bits most significant first. A bit past the end reads as 0.
type bitReader struct {
	data []byte
	pos  int
}

func (b *bitReader) read(n int) int {
	v := 0
	for range n {
		bit := 0
		if i := b.pos >> 3; i < len(b.data) {
			bit = int(b.data[i]) >> (7 - b.pos&7) & 1
		}
		v = v<<1 | bit
		b.pos++
	}
	return v
}

// pixelCodes reads one pixel code string of the given bits per code from byte offset. It returns the runs and the
// offset after the string. The string also ends after pixels pixels, at the end of the region row, because ffmpeg
// ends an 8-bit string with 1 zero byte in place of 2. drawField reads the zero bytes after the row end as data types
// and skips them.
func pixelCodes(data []byte, offset, bits, pixels int) (runs []run, next int) {
	b := &bitReader{data: data, pos: offset * 8}
	for b.pos < len(data)*8 && pixels > 0 {
		if code := b.read(bits); code != 0 {
			runs = append(runs, run{byte(code), 1})
			pixels--
			continue
		}
		count, codeBits, ok := runHeader(b, bits)
		if !ok {
			break
		}
		code := 0
		if codeBits > 0 {
			code = b.read(codeBits)
		}
		runs = append(runs, run{byte(code), count})
		pixels -= count
	}
	return runs, (b.pos + 7) >> 3
}

// runHeader reads the bits after a zero pixel code. ok is false at the end of the string. codeBits is 0 for a run of
// the pixel code 0.
func runHeader(b *bitReader, bits int) (count, codeBits int, ok bool) {
	switch bits {
	case 2:
		return runHeader2(b)
	case 4:
		return runHeader4(b)
	}
	if b.read(1) == 0 {
		n := b.read(7)
		return n, 0, n != 0
	}
	return b.read(7), 8, true
}

func runHeader2(b *bitReader) (count, codeBits int, ok bool) {
	if b.read(1) == 1 {
		return 3 + b.read(3), 2, true
	}
	if b.read(1) == 1 {
		return 1, 0, true
	}
	switch b.read(2) {
	case 0:
		return 0, 0, false
	case 1:
		return 2, 0, true
	case 2:
		return 12 + b.read(4), 2, true
	}
	return 29 + b.read(8), 2, true
}

func runHeader4(b *bitReader) (count, codeBits int, ok bool) {
	if b.read(1) == 0 {
		n := b.read(3)
		return n + 2, 0, n != 0
	}
	if b.read(1) == 0 {
		return 4 + b.read(2), 4, true
	}
	switch b.read(2) {
	case 0:
		return 1, 0, true
	case 1:
		return 2, 0, true
	case 2:
		return 9 + b.read(4), 4, true
	}
	return 25 + b.read(8), 4, true
}
