package teletext

import "math/bits"

// hamming84 maps a Hamming 8/4 byte to its 4 data bits with 1 bit error corrected, or -1 for 2 bit errors.
//
//nolint:gochecknoglobals // lookup table
var hamming84 = buildHamming84()

func hamming84Code(value int) byte {
	d1, d2, d3, d4 := value&1, value>>1&1, value>>2&1, value>>3&1
	p1, p2, p3 := 1^d1^d3^d4, 1^d1^d2^d4, 1^d1^d2^d3
	p4 := 1 ^ p1 ^ d1 ^ p2 ^ d2 ^ p3 ^ d3 ^ d4
	return byte(p1 | d1<<1 | p2<<2 | d2<<3 | p3<<4 | d3<<5 | p4<<6 | d4<<7)
}

func buildHamming84() [256]int8 {
	var t [256]int8
	for b := range t {
		t[b] = -1
		for v := range 16 {
			if bits.OnesCount8(byte(b)^hamming84Code(v)) <= 1 {
				t[b] = int8(v)
			}
		}
	}
	return t
}

// hamming2418 returns the 18 data bits of a Hamming 24/18 triplet with 1 bit error corrected. ok is false for 2 bit errors.
// Bits 1 to 23 form a Hamming code with odd parity checks at bits 1, 2, 4, 8 and 16. Bit 24 gives all 24 bits odd parity.
func hamming2418(b0, b1, b2 byte) (value int, ok bool) {
	v := int(b0) | int(b1)<<8 | int(b2)<<16
	syndrome := 0x1F
	for pos := 1; pos <= 23; pos++ {
		if v>>(pos-1)&1 == 1 {
			syndrome ^= pos
		}
	}
	switch {
	case bits.OnesCount(uint(v))%2 == 1:
		if syndrome != 0 {
			return 0, false
		}
	case syndrome != 0:
		if syndrome > 23 {
			return 0, false
		}
		v ^= 1 << (syndrome - 1)
	}
	return v>>2&0x01 | v>>4&0x07<<1 | v>>8&0x7F<<4 | v>>16&0x7F<<11, true
}
