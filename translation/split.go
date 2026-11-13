package translation

import (
	"math"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
)

// scriptExtensions holds the Common and Inherited characters whose Script_Extensions name Han, Hiragana, Katakana,
// Thai, Lao, Khmer or Myanmar. PCRE2 matches \p{Han} and the others against Script_Extensions, and the Go script
// tables hold only the script.
//
//nolint:gochecknoglobals // a lookup table
var scriptExtensions = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x3001, 0x3003, 1},
		{0x3006, 0x3006, 1},
		{0x3008, 0x3011, 1},
		{0x3013, 0x301f, 1},
		{0x302a, 0x302d, 1},
		{0x3030, 0x3035, 1},
		{0x3037, 0x3037, 1},
		{0x303c, 0x303f, 1},
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
		{0xa92e, 0xa92e, 1},
		{0xfe45, 0xfe46, 1},
		{0xff61, 0xff65, 1},
		{0xff70, 0xff70, 1},
		{0xff9e, 0xff9f, 1},
	},
	R32: []unicode.Range32{{0x1d360, 0x1d371, 1}, {0x1f250, 0x1f251, 1}},
}

func noSpaceScript(c string) bool {
	r, size := utf8.DecodeRuneInString(c)
	if r == utf8.RuneError || size != len(c) {
		return false
	}
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Thai, unicode.Lao, unicode.Khmer, unicode.Myanmar, scriptExtensions)
}

func isSpaceCharacter(c string) bool {
	return len(c) == 1 && strings.IndexByte(markupcore.PHPTrimSet, c[0]) >= 0
}

// split cuts tokens into one piece per weight, in proportion to the weights.
// A cut falls at a space, or between two characters of a script without spaces such as Chinese.
// Each piece closes the tags that are open at its end, and the next piece opens them again.
func split(tokens []token, weights []int) [][]token {
	var chars []string
	for _, t := range tokens {
		if t.kind == textToken {
			chars = append(chars, markupcore.Characters(t.text)...)
		}
	}
	return balance(distribute(tokens, cutPositions(chars, weights), len(weights)))
}

// cutPositions returns the character offsets where pieces 2 to n start, each as close as the candidates allow to its
// share of the weights.
func cutPositions(chars []string, weights []int) []int {
	var candidates []int
	for k := 1; k < len(chars); k++ {
		if isSpaceCharacter(chars[k]) {
			if k < len(chars)-1 {
				candidates = append(candidates, k)
			}
		} else if noSpaceScript(chars[k-1]) && noSpaceScript(chars[k]) {
			candidates = append(candidates, k)
		}
	}

	total := 0
	for _, w := range weights {
		total += w
	}
	n := len(chars)
	cutCount := min(len(weights)-1, len(candidates))
	cuts := make([]int, 0, max(cutCount, 0))
	share, previous := 0, -1
	for k := range cutCount {
		share += weights[k]
		target := float64(n*share) / float64(total)
		best := previous + 1
		last := len(candidates) - (cutCount - 1 - k)
		// The candidates rise, so the distance to the target falls and then rises. The first
		// distance that does not fall ends the search at the minimum that a full scan finds first.
		for c := best + 1; c < last; c++ {
			if math.Abs(float64(candidates[c])-target) >= math.Abs(float64(candidates[best])-target) {
				break
			}
			best = c
		}
		cuts = append(cuts, candidates[best])
		previous = best
	}
	return cuts
}

// distribute puts each token into the piece that holds its position. A text token splits at the cuts.
// A white space character at the start of a piece is dropped.
func distribute(tokens []token, cuts []int, pieces int) [][]token {
	out := make([][]token, pieces)
	piece, offset := 0, 0
	for _, t := range tokens {
		switch t.kind {
		case closeToken:
			out[piece] = append(out[piece], t)
		case textToken:
			piece, offset = distributeText(out, t.text, cuts, piece, offset)
		case openToken, singleToken:
			for piece < len(cuts) && offset >= cuts[piece] {
				piece++
			}
			out[piece] = append(out[piece], t)
		}
	}
	return out
}

// distributeText adds text to the pieces from piece on, and returns the piece and the offset after its last character.
func distributeText(out [][]token, text string, cuts []int, piece, offset int) (int, int) {
	run, pos := 0, 0
	for _, c := range markupcore.Characters(text) {
		for piece < len(cuts) && offset >= cuts[piece] {
			out[piece] = appendText(out[piece], text[run:pos])
			piece++
			run = pos
		}
		pos += len(c)
		if piece > 0 && offset == cuts[piece-1] && isSpaceCharacter(c) {
			run = pos
		}
		offset++
	}
	out[piece] = appendText(out[piece], text[run:])
	return piece, offset
}

// removeLast removes the last number from open. A number is in open at most once, because the codec numbers
// each tag pair once, so this is the match that PHP removes.
func removeLast(open []int, number int) []int {
	for k := len(open) - 1; k >= 0; k-- {
		if open[k] == number {
			return slices.Delete(open, k, k+1)
		}
	}
	return open
}

// balance closes in each piece the tags that are open at its end, and opens them again at the start of the next piece.
func balance(pieces [][]token) [][]token {
	var open []int
	for i, piece := range pieces {
		balanced := make([]token, 0, len(open)+len(piece))
		for _, number := range open {
			balanced = append(balanced, token{kind: openToken, number: number})
		}
		for _, t := range piece {
			switch t.kind {
			case openToken:
				open = append(open, t.number)
			case closeToken:
				open = removeLast(open, t.number)
			case textToken, singleToken:
			}
			balanced = append(balanced, t)
		}
		for k := len(open) - 1; k >= 0; k-- {
			balanced = append(balanced, token{kind: closeToken, number: open[k]})
		}
		pieces[i] = balanced
	}
	return pieces
}
