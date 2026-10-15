package linewrap

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/internal/grapheme"
	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

// cjkOther holds the CJK punctuation and the full-width and half-width forms without half-width Hangul. From U+3099
// on it also holds the Common characters whose Script_Extensions name Han, Hiragana or Katakana, because PCRE2 matches
// \p{Han} against Script_Extensions and the Go script tables hold only the script.
//
//nolint:gochecknoglobals // a lookup table
var cjkOther = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x3000, 0x303f, 1},
		{0x3099, 0x309c, 1},
		{0x30a0, 0x30a0, 1},
		{0x30fb, 0x30fc, 1},
		{0x3190, 0x319f, 1},
		{0x31c0, 0x31e3, 1},
		{0x3220, 0x3247, 1},
		{0x3280, 0x32b0, 1},
		{0x32c0, 0x32cb, 1},
		{0x32ff, 0x32ff, 1},
		{0x3358, 0x3370, 1},
		{0x337b, 0x337f, 1},
		{0x33e0, 0x33fe, 1},
		{0xa700, 0xa707, 1},
		{0xfe45, 0xfe46, 1},
		{0xff01, 0xff9f, 1},
		{0xffe0, 0xffef, 1},
	},
	R32: []unicode.Range32{{0x1d360, 0x1d371, 1}, {0x1f250, 0x1f251, 1}},
}

// noLineStart holds the characters that JIS X 4051 keeps from the start of a line: closing brackets and quotes,
// stops, commas, middle dots, small kana, the long vowel mark and iteration marks. UAX #14 adds U+3000.
//
//nolint:gochecknoglobals // a lookup table
var noLineStart = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x3000, 0x3002, 1},
		{0x3005, 0x3005, 1},
		{0x3009, 0x3009, 1},
		{0x300b, 0x300b, 1},
		{0x300d, 0x300d, 1},
		{0x300f, 0x300f, 1},
		{0x3011, 0x3011, 1},
		{0x3015, 0x3015, 1},
		{0x3017, 0x3017, 1},
		{0x3019, 0x3019, 1},
		{0x301b, 0x301c, 1},
		{0x301e, 0x301f, 1},
		{0x303b, 0x303b, 1},
		{0x3041, 0x3041, 1},
		{0x3043, 0x3043, 1},
		{0x3045, 0x3045, 1},
		{0x3047, 0x3047, 1},
		{0x3049, 0x3049, 1},
		{0x3063, 0x3063, 1},
		{0x3083, 0x3083, 1},
		{0x3085, 0x3085, 1},
		{0x3087, 0x3087, 1},
		{0x308e, 0x308e, 1},
		{0x3095, 0x3096, 1},
		{0x309b, 0x309e, 1},
		{0x30a0, 0x30a1, 1},
		{0x30a3, 0x30a3, 1},
		{0x30a5, 0x30a5, 1},
		{0x30a7, 0x30a7, 1},
		{0x30a9, 0x30a9, 1},
		{0x30c3, 0x30c3, 1},
		{0x30e3, 0x30e3, 1},
		{0x30e5, 0x30e5, 1},
		{0x30e7, 0x30e7, 1},
		{0x30ee, 0x30ee, 1},
		{0x30f5, 0x30f6, 1},
		{0x30fb, 0x30fe, 1},
		{0x31f0, 0x31ff, 1},
		{0xff01, 0xff01, 1},
		{0xff09, 0xff09, 1},
		{0xff0c, 0xff0c, 1},
		{0xff0e, 0xff0e, 1},
		{0xff1a, 0xff1b, 1},
		{0xff1f, 0xff1f, 1},
		{0xff3d, 0xff3d, 1},
		{0xff5d, 0xff5d, 1},
		{0xff60, 0xff61, 1},
		{0xff63, 0xff65, 1},
		{0xff67, 0xff70, 1},
		{0xff9e, 0xff9f, 1},
	},
}

// noLineEnd holds the opening brackets and quotes that JIS X 4051 keeps from the end of a line.
//
//nolint:gochecknoglobals // a lookup table
var noLineEnd = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x3008, 0x3008, 1},
		{0x300a, 0x300a, 1},
		{0x300c, 0x300c, 1},
		{0x300e, 0x300e, 1},
		{0x3010, 0x3010, 1},
		{0x3014, 0x3014, 1},
		{0x3016, 0x3016, 1},
		{0x3018, 0x3018, 1},
		{0x301a, 0x301a, 1},
		{0x301d, 0x301d, 1},
		{0xff08, 0xff08, 1},
		{0xff3b, 0xff3b, 1},
		{0xff5b, 0xff5b, 1},
		{0xff5f, 0xff5f, 1},
		{0xff62, 0xff62, 1},
	},
}

var tagOrEntityRegexp = regexp.MustCompile(`(` + markupcore.TagPattern + `)|` + markupcore.EntityPattern)

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, cjkOther)
}

func startsWithCJK(text string) bool {
	r, _ := utf8.DecodeRuneInString(text)
	return utf8.ValidString(text) && isCJK(r)
}

func endsWithCJK(text string) bool {
	r, _ := utf8.DecodeLastRuneInString(text)
	return utf8.ValidString(text) && isCJK(r)
}

// part is a piece of text that a line break may not split. A word is a part, or splits into parts between CJK
// characters.
type part struct {
	text        string
	length      int
	spaceBefore bool
	midPhrase   bool
}

func breakableParts(text string) []part {
	var parts []part
	for _, w := range MeasuredWords(text) {
		if !utf8.ValidString(w.Text) || strings.IndexFunc(w.Text, isCJK) < 0 {
			parts = append(parts, part{text: w.Text, length: w.Length, spaceBefore: true})
			continue
		}
		pieces := splitBetweenCJK(w.Text)
		for i, piece := range pieces {
			p := part{text: piece, length: w.Length, spaceBefore: i == 0}
			if len(pieces) > 1 {
				p.length = Length(MeasuredWords(piece))
			}
			if i > 0 {
				r, _ := utf8.DecodeLastRuneInString(markupcore.VisibleText(pieces[i-1]))
				p.midPhrase = !unicode.IsPunct(r) && r != 0x3000
			}
			parts = append(parts, p)
		}
	}
	return parts
}

// splitBetweenCJK splits a word of valid UTF-8 where JIS X 4051 allows a break between 2 CJK characters.
// A closing tag stays with the character before the break. Other tags go with the character after it.
func splitBetweenCJK(word string) []string {
	var pieces []string
	pieceStart, breakAt := 0, 0
	canEndLine := false
	visit := func(token string, at int) {
		r, _ := utf8.DecodeRuneInString(token)
		if canEndLine && isCJK(r) && !unicode.Is(noLineStart, r) {
			pieces = append(pieces, word[pieceStart:breakAt])
			pieceStart = breakAt
		}
		canEndLine = isCJK(r) && !unicode.Is(noLineEnd, r)
		breakAt = at + len(token)
	}
	visitClusters := func(from, to int) {
		at := from
		for c := range grapheme.Clusters(word[from:to]) {
			visit(c, at)
			at += len(c)
		}
	}

	at := 0
	for _, m := range tagOrEntityRegexp.FindAllStringSubmatchIndex(word, -1) {
		visitClusters(at, m[0])
		switch {
		case m[2] < 0:
			visit(word[m[0]:m[1]], m[0])
		case breakAt == m[0] && strings.HasPrefix(word[m[0]:], "</"):
			breakAt = m[1]
		}
		at = m[1]
	}
	visitClusters(at, len(word))
	return append(pieces, word[pieceStart:])
}
