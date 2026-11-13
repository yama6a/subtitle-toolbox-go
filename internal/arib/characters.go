package arib

import (
	"fmt"
	"slices"
	"sync"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
)

// Adapted from xqq/libaribcaption, src/decoder/b24_conv_tables.hpp, b24_gaiji_table.hpp, b24_macros.hpp and b24_colors.cpp
// (https://github.com/xqq/libaribcaption/blob/37dde0f32652c32aa98bf92278944d1cfb2e7354/src/decoder/), MIT.

// The final bytes of the graphic sets. A DRCS set is setDRCS plus its final byte.
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
	hiraganaIterationMarks = "\u309D\u309E"
	katakanaIterationMarks = "\u30FD\u30FE"
	kanaSymbols            = "\u30FC\u3002\u300C\u300D\u3001\u30FB"
	replacement            = "\uFFFD"
)

// halfWidthForms are the Japanese symbols that have a half width form at middle size, as libaribcaption writes them.
//
//nolint:gochecknoglobals // lookup table
var halfWidthForms = map[rune]string{
	0x30FC: "\uFF70", 0x3002: "\uFF61", 0x300C: "\uFF62", 0x300D: "\uFF63", 0x3001: "\uFF64",
	0x30FB: "\uFF65", 0x309B: "\uFF9E", 0x309C: "\uFF9F", 0xFFE3: "\u00AF", 0x301C: "~",
	0xFFE5: "\u00A5", 0x25CB: "\uFFEE", 0x25A0: "\uFFED", 0x2192: "\uFFEB", 0x2190: "\uFFE9",
	0x2191: "\uFFEA", 0x2193: "\uFFEC",
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
	c := setCharacter(set, first, second, halfWidth || latin)
	if !halfWidth || latin {
		return c
	}
	return toHalfWidth(c)
}

func setCharacter(set, first, second int, halfWidth bool) string {
	switch set {
	case setKanji, setJISX0213Plane1, setAdditional:
		return kanji(first, second, false)
	case setJISX0213Plane2:
		return kanji(first, second, true)
	case setAlphanumeric, setPropAlphanumeric:
		return alphanumeric(first, halfWidth)
	case setHiragana, setPropHiragana:
		return kana(first, 0x3041, 0x73, hiraganaIterationMarks)
	case setKatakana, setPropKatakana:
		return kana(first, 0x30A1, 0x76, katakanaIterationMarks)
	case setJISX0201Kana:
		return jisX0201(first, halfWidth)
	case setLatinExtension:
		return latinExtensionCharacter(first)
	case setLatinSpecial:
		return latinSpecialCharacter(first)
	default:
		return ""
	}
}

// toHalfWidth turns the full width forms of middle size text into their half width forms.
func toHalfWidth(c string) string {
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

func latinExtensionCharacter(code int) string {
	if code == 0x2D {
		return "\u00FF"
	}
	return string(latinExtension[code|0x80-0xA0])
}

func latinSpecialCharacter(code int) string {
	if r, ok := latinSpecial[code]; ok {
		return string(r)
	}
	return replacement
}

func jisX0201(code int, halfWidth bool) string {
	switch {
	case code > 0x5F:
		return replacement
	case halfWidth:
		return string(rune(0xFF61 + code - 0x21))
	default:
		return string([]rune(jisX0201FullWidth)[code-0x21])
	}
}

// jisX0208Exceptions are the cells where x/text and the glibc EUC-JP table of the PHP reader differ. x/text follows
// the Encoding Standard, glibc follows JIS X 0208. The value 0xFFFD marks a NEC row 13 cell that glibc lacks.
//
//nolint:gochecknoglobals // lookup table
var jisX0208Exceptions = map[int]rune{
	0x2141: 0x301C, 0x2142: 0x2016, 0x215D: 0x2212, 0x2171: 0x00A2, 0x2172: 0x00A3, 0x224C: 0x00AC,
	0x2D70: 0xFFFD, 0x2D71: 0xFFFD, 0x2D72: 0xFFFD, 0x2D74: 0xFFFD, 0x2D75: 0xFFFD, 0x2D76: 0xFFFD,
	0x2D77: 0xFFFD, 0x2D7A: 0xFFFD, 0x2D7B: 0xFFFD, 0x2D7C: 0xFFFD,
}

const jisX0208Rows = 84

// jisX0208 holds the characters of the rows 1 to 84 of JIS X 0208 from column 1. 0 marks an empty cell.
//
//nolint:gochecknoglobals // lookup table that is built on first use
var jisX0208 = sync.OnceValue(func() *[jisX0208Rows][94]rune {
	var table [jisX0208Rows][94]rune
	dec := japanese.EUCJP.NewDecoder()
	for row := range jisX0208Rows {
		for column := range 94 {
			out, err := dec.Bytes([]byte{byte(0xA1 + row), byte(0xA1 + column)})
			if r, size := utf8.DecodeRune(out); err == nil && size == len(out) && r != utf8.RuneError {
				table[row][column] = r
			}
			if r, ok := jisX0208Exceptions[(0x21+row)<<8|0x21+column]; ok {
				table[row][column] = r
			}
		}
	}
	return &table
})

// kanji returns a character of JIS X 0208, or of the additional table in the rows 85 to 94. x/text has no
// JIS X 0213, so every other cell below row 85, and all of plane 2 there, gives U+FFFD.
func kanji(first, second int, plane2 bool) string {
	row := first - 0x20
	var r rune
	switch {
	case row >= 85:
		r = additional[row-85][second-0x21]
	case !plane2:
		r = jisX0208()[row-1][second-0x21]
	}
	if r == 0 || r == 0xFFFD {
		return replacement
	}
	return string(r)
}

func alphanumeric(code int, halfWidth bool) string {
	switch {
	case code == 0x5C && halfWidth:
		return "\u00A5"
	case code == 0x5C:
		return "\uFFE5"
	case code == 0x7E && halfWidth:
		return "\u203E"
	case code == 0x7E:
		return "\uFFE3"
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
		return "\u3000"
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

type rgb struct {
	r, g, b int
	ok      bool
}

// buildColors lists the 8 full and 7 half intensity colors with transparent between them, then the other mixes of
// the levels 0, 85, 170 and 255. Indexes 65 to 127 repeat the colors without transparent. They have half opacity,
// which a font tag cannot show.
func buildColors() []string {
	levels := intensityColors()
	steps := []int{0, 85, 170, 255}
	for _, r := range steps {
		for _, g := range steps {
			for _, b := range steps {
				if c := (rgb{r, g, b, true}); !slices.Contains(levels, c) {
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

// intensityColors returns the 8 colors at full intensity and the 8 at half intensity, with transparent at index 8.
func intensityColors() []rgb {
	levels := make([]rgb, 0, 16)
	for _, level := range []int{255, 170} {
		for bits := range 8 {
			c := rgb{ok: true}
			for i, p := range []*int{&c.r, &c.g, &c.b} {
				if bits>>i&1 != 0 {
					*p = level
				}
			}
			levels = append(levels, c)
		}
	}
	levels[8] = rgb{}
	return levels
}
