package arib

import (
	"fmt"
	"slices"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
)

// Adapted from xqq/libaribcaption, src/decoder/b24_conv_tables.hpp, b24_gaiji_table.hpp, b24_macros.hpp and b24_colors.cpp
// (https://github.com/xqq/libaribcaption/blob/37dde0f32652c32aa98bf92278944d1cfb2e7354/src/decoder/), MIT.

// The final bytes of the graphic sets. A DRCS set is drcs plus its final byte.
const (
	setKanji            = 0x42
	setAlphanumeric     = 0x4A
	setLatinExtension   = 0x4B
	setLatinSpecial     = 0x4C
	setHiragana         = 0x30
	setKatakana         = 0x31
	setPropAlphanumeric = 0x36
	setPropHiragana     = 0x37
	setPropKatakana     = 0x38
	setJISX0201Kana     = 0x49
	setJISX0213Plane1   = 0x39
	setJISX0213Plane2   = 0x3A
	setAdditional       = 0x3B
	setDRCS             = 0x100
	setDRCS0            = 0x140
	setMacro            = 0x170
)

const (
	hiraganaIterationMarks = "ゝゞ"
	katakanaIterationMarks = "ヽヾ"
	kanaSymbols            = "ー。「」、・"
)

// halfWidthForms are the Japanese symbols that have a half width form at middle size, as libaribcaption writes them.
//
//nolint:gochecknoglobals // lookup table
var halfWidthForms = map[rune]string{
	0x30FC: "ｰ", 0x3002: "｡", 0x300C: "｢", 0x300D: "｣", 0x3001: "､",
	0x30FB: "･", 0x309B: "ﾞ", 0x309C: "ﾟ", 0xFFE3: "¯", 0x301C: "~",
	0xFFE5: "¥", 0x25CB: "￮", 0x25A0: "￭", 0x2192: "￫", 0x2190: "￩",
	0x2191: "￪", 0x2193: "￬",
}

// twoByteSet reports whether the set has 2-byte codes.
func twoByteSet(set int) bool {
	switch set {
	case setKanji, setJISX0213Plane1, setJISX0213Plane2, setAdditional, setDRCS0:
		return true
	default:
		return false
	}
}

// character returns the character of the codes first and second, from 0x21 to 0x7E, in set. It returns "" for a
// mosaic set. halfWidth is true for middle size text.
func character(set, first, second int, halfWidth, latin bool) string {
	var c string
	switch set {
	case setKanji, setJISX0213Plane1, setAdditional:
		c = kanji(first, second, false)
	case setJISX0213Plane2:
		c = kanji(first, second, true)
	case setAlphanumeric, setPropAlphanumeric:
		c = alphanumeric(first, halfWidth || latin)
	case setHiragana, setPropHiragana:
		c = kana(first, 0x3041, 0x73, hiraganaIterationMarks)
	case setKatakana, setPropKatakana:
		c = kana(first, 0x30A1, 0x76, katakanaIterationMarks)
	case setJISX0201Kana:
		c = jisX0201(first, halfWidth)
	case setLatinExtension:
		c = "ÿ"
		if first != 0x2D {
			c = string(latinExtension[first|0x80-0xA0])
		}
	case setLatinSpecial:
		c = "�"
		if r, ok := latinSpecial[first]; ok {
			c = string(r)
		}
	}
	if !halfWidth || latin {
		return c
	}
	r, _ := utf8.DecodeRuneInString(c + " ")
	switch {
	case r >= 0xFF01 && r <= 0xFF5E:
		return string(r - 0xFEE0)
	case r == 0x3000:
		return " "
	}
	if h, ok := halfWidthForms[r]; ok {
		return h
	}
	return c
}

func jisX0201(code int, halfWidth bool) string {
	switch {
	case code > 0x5F:
		return "�"
	case halfWidth:
		return string(rune(0xFF61 + code - 0x21))
	default:
		return string([]rune(jisX0201FullWidth)[code-0x21])
	}
}

// kanji returns a character of JIS X 0208 from x/text, or of the additional table in the rows 85 to 94. x/text has
// no JIS X 0213, so every other cell, and all of plane 2, gives U+FFFD.
func kanji(first, second int, plane2 bool) string {
	if row := first - 0x20; row >= 85 {
		if r := additional[row-85][second-0x21]; r != 0 {
			return string(r)
		}
		return "�"
	}
	if plane2 {
		return "�"
	}
	out, err := japanese.EUCJP.NewDecoder().Bytes([]byte{byte(first | 0x80), byte(second | 0x80)})
	if err != nil || !utf8.Valid(out) || utf8.RuneCount(out) != 1 || string(out) == "�" {
		return "�"
	}
	return string(out)
}

func alphanumeric(code int, halfWidth bool) string {
	switch {
	case code == 0x5C && halfWidth:
		return "¥"
	case code == 0x5C:
		return "￥"
	case code == 0x7E && halfWidth:
		return "‾"
	case code == 0x7E:
		return "￣"
	case halfWidth:
		return string(rune(code))
	default:
		return string(rune(0xFF01 + code - 0x21))
	}
}

func kana(code, start, last int, iterationMarks string) string {
	switch {
	case code <= last:
		return string(rune(start + code - 0x21))
	case code <= 0x76:
		return "　"
	case code <= 0x78:
		return string([]rune(iterationMarks)[code-0x77])
	default:
		return string([]rune(kanaSymbols)[code-0x79])
	}
}

// colors holds the 128 colors of STD-B24 table 9-1. "" marks transparent.
//
//nolint:gochecknoglobals // lookup table
var colors = buildColors()

// color returns the text color of index in palette, or "" for transparent.
func color(palette, index int) string {
	return colors[palette*16+index]
}

// buildColors lists the 8 full and 7 half intensity colors with transparent between them, then the other mixes of
// the levels 0, 85, 170 and 255. Indexes 65 to 127 repeat the colors without transparent. They have half opacity,
// which a font tag cannot show.
func buildColors() []string {
	type rgb struct {
		r, g, b int
		ok      bool
	}
	var levels []rgb
	for _, level := range []int{255, 170} {
		for bits := range 8 {
			c := rgb{ok: true}
			if bits&1 != 0 {
				c.r = level
			}
			if bits&2 != 0 {
				c.g = level
			}
			if bits&4 != 0 {
				c.b = level
			}
			levels = append(levels, c)
		}
	}
	levels[8] = rgb{}
	steps := []int{0, 85, 170, 255}
	for _, r := range steps {
		for _, g := range steps {
			for _, b := range steps {
				c := rgb{r, g, b, true}
				if !slices.Contains(levels, c) {
					levels = append(levels, c)
				}
			}
		}
	}
	for _, c := range levels {
		if c.ok {
			levels = append(levels, c)
		}
	}
	out := make([]string, 128)
	for i, c := range levels[:128] {
		if c.ok {
			out[i] = fmt.Sprintf("#%02x%02x%02x", c.r, c.g, c.b)
		}
	}
	return out
}
