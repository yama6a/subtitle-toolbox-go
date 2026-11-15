package dvbsub

import (
	"encoding/binary"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/bitmap"
)

// clut holds the colours of the 2-bit, 4-bit and 8-bit pixel codes as 0xRRGGBBAA.
type clut struct {
	bits2 [4]uint32
	bits4 [16]uint32
	bits8 [256]uint32
}

// colors returns the colour map of a region of depth bits per pixel code, padded to 256 entries.
func (c *clut) colors(depth int) (m [256]uint32) {
	switch depth {
	case 2:
		copy(m[:], c.bits2[:])
	case 4:
		copy(m[:], c.bits4[:])
	default:
		m = c.bits8
	}
	return m
}

// defaultCLUT returns the colours that apply before a CLUT definition segment sets entries.
func defaultCLUT() clut {
	c := clut{bits2: [4]uint32{0, 0xFFFFFFFF, 0x000000FF, 0x7F7F7FFF}}
	for entry := 1; entry < 256; entry++ {
		base, low, high, alpha := uint32(0), uint32(255), uint32(0), uint32(63)
		if entry >= 8 {
			switch entry & 0x88 {
			case 0x00:
				base, low, high, alpha = 0, 85, 170, 255
			case 0x08:
				base, low, high, alpha = 0, 85, 170, 127
			case 0x80:
				base, low, high, alpha = 127, 43, 85, 255
			default:
				base, low, high, alpha = 0, 43, 85, 255
			}
		}
		full := uint32(0xFF)
		color := alpha
		for channel := range 3 {
			shift := 24 - 8*uint(channel)
			lowBit, highBit := uint32(entry>>channel&1), uint32(entry>>(channel+4)&1)
			color += (base + lowBit*low + highBit*high) << shift
			if entry < 8 {
				full += lowBit * 255 << shift
			} else {
				full += lowBit * 127 << shift
			}
		}
		c.bits8[entry] = color
		if entry < 16 {
			c.bits4[entry] = full
		}
	}
	return c
}

// set reads the entries of a CLUT definition segment into c. The entries start at offset 2.
func (c *clut) set(data []byte) {
	for offset := 2; offset+4 <= len(data); {
		entry, flags := int(data[offset]), data[offset+1]
		var y, cr, cb, t byte
		if flags&0x01 != 0 {
			if offset+6 > len(data) {
				return
			}
			y, cr, cb, t = data[offset+2], data[offset+3], data[offset+4], data[offset+5]
			offset += 6
		} else {
			v := binary.BigEndian.Uint16(data[offset+2:])
			y, cr, cb, t = byte(v>>8&0xFC), byte(v>>6&0x0F)<<4, byte(v>>2&0x0F)<<4, byte(v&0x03)<<6
			offset += 4
		}
		var color uint32
		if y != 0 {
			r, g, b := bitmap.ToRGB(y, cr, cb, bitmap.BT601)
			color = uint32(r)<<24 | uint32(g)<<16 | uint32(b)<<8 | uint32(255-t)
		}
		if flags&0x80 != 0 && entry < 4 {
			c.bits2[entry] = color
		}
		if flags&0x40 != 0 && entry < 16 {
			c.bits4[entry] = color
		}
		if flags&0x20 != 0 {
			c.bits8[entry] = color
		}
	}
}
