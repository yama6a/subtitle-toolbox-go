package arib

import (
	"crypto/md5" //nolint:gosec // the hash names a glyph, it protects nothing
	"encoding/hex"
	"fmt"
	"math/bits"
)

// Adapted from xqq/libaribcaption, src/decoder/decoder_impl.cpp (ParseDRCS, GetDRCSDigest) and src/decoder/b24_drcs_conv.cpp
// (https://github.com/xqq/libaribcaption/blob/37dde0f32652c32aa98bf92278944d1cfb2e7354/src/decoder/), MIT.

// glyph is a DRCS glyph of the stream. character is "" for a glyph that has no known character.
type glyph struct {
	hash, size, character string
}

type keyedGlyph struct {
	key int
	glyph
}

// parseDRCS reads a DRCS data unit. The key of a glyph is the DRCS set times 0x10000 plus the character code.
func parseDRCS(unit []byte, twoByte bool) []keyedGlyph {
	var glyphs []keyedGlyph
	offset := 1
	count := 0
	if len(unit) > 0 {
		count = int(unit[0])
	}
	for code := 0; code < count && offset+3 <= len(unit); code++ {
		characterCode := int(unit[offset])<<8 | int(unit[offset+1])
		fonts := int(unit[offset+2])
		offset += 3
		for font := 0; font < fonts && offset+4 <= len(unit); font++ {
			if unit[offset]&0x0F > 1 {
				offset += 5
				if offset <= len(unit) {
					offset += int(unit[offset-2])<<8 | int(unit[offset-1])
				}
				continue
			}
			depth, width, height := int(unit[offset+1])+2, int(unit[offset+2]), int(unit[offset+3])
			size := (width*height*bits.Len(uint(depth-1)) + 7) / 8
			pixels := unit[offset+4 : min(len(unit), offset+4+size)]
			offset += 4 + len(pixels)
			var key int
			switch {
			case twoByte && characterCode >= 0xEC00 && characterCode <= 0xF8FF:
				key = setDRCS0<<16 | characterCode
			case twoByte:
				key = setDRCS0<<16 | characterCode&0x7F7F
			default:
				key = (setDRCS|0x40|characterCode>>8&0x0F)<<16 | characterCode&0x7F
			}
			glyphs = append(glyphs, keyedGlyph{key, glyph{
				hash:      hashOf(pixels),
				size:      fmt.Sprintf("%dx%d", width, height),
				character: knownCharacter(pixels, width, height, depth),
			}})
		}
	}
	return glyphs
}

func hashOf(b []byte) string {
	sum := md5.Sum(b) //nolint:gosec // see the import
	return hex.EncodeToString(sum[:])
}

// knownCharacter looks the bitmap up as it is, then in the shifted or scaled forms that libaribcaption also tries.
func knownCharacter(pixels []byte, width, height, depth int) string {
	for variation := 0; ; variation++ {
		hash, ok := digest(pixels, width, height, depth, variation)
		if !ok {
			return ""
		}
		if r, ok := knownGlyphs[hash]; ok {
			return string(r)
		}
	}
}

// digest returns the hash of a variation of the bitmap. Variation 0 is the bitmap. For a 16 by 18 glyph with 2
// levels, the variations 1 to 3 draw it twice as large on the left, in the middle and on the right of a 36 by 36
// glyph with 4 levels, as ARIB TR-B14 profile C does. For a glyph with 4 levels, variation 1 moves it 2 dots to
// the right and variation 2 moves it 2 dots to the left. The result false ends the variations.
func digest(pixels []byte, width, height, depth, variation int) (string, bool) {
	switch {
	case variation == 0:
		return hashOf(pixels), true
	case variation <= 3 && depth == 2 && width == 16 && height == 18:
		return hashOf(scaled(pixels, variation)), true
	case variation > 2 || depth != 4 || width < 4 || width > 36 || width%2 != 0 || height < 1 || height > 36:
		return "", false
	}
	// Each 4 bits hold 2 dots, so a move of 2 dots is a move of 4 bits.
	nibbles := width / 2
	count := nibbles * height
	buffer := make([]byte, (count+1)/2)
	target := -1
	if variation == 1 {
		target = 1
	}
	for index := range count {
		twoDots := byte(at(pixels, index>>1) >> (4 - (index&1)*4) & 0x0F)
		edge := index
		if variation == 1 {
			edge = target
		}
		if edge%nibbles == 0 {
			if twoDots != 0 {
				if variation == 1 {
					return hashOf(pixels), true
				}
				return "", false
			}
			last := 0
			if variation == 1 {
				last = count
			}
			if edge == last {
				target++
				continue
			}
		}
		if target&1 == 0 {
			buffer[target>>1] = twoDots << 4
		} else {
			buffer[target>>1] |= twoDots
		}
		target++
	}
	return hashOf(buffer), true
}

// scaled draws a 16 by 18 glyph of 2 levels at the offset variation-1 of a 36 by 36 glyph of 4 levels.
func scaled(pixels []byte, variation int) []byte {
	buffer := make([]byte, 36*36/4)
	for y := range 18 {
		index := 36*y + variation - 1
		for x := range 16 {
			pixel := byte(at(pixels, 2*y+x>>3) >> (7 - x&7) & 1 * 15)
			i := index >> 1
			if index&1 == 0 {
				buffer[i] = pixel << 4
			} else {
				buffer[i] |= pixel
			}
			buffer[9+i] = buffer[i]
			index++
		}
	}
	return buffer
}
