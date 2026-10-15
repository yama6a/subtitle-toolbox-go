package markupcore

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/internal/grapheme"
)

const (
	unicodeHyphen          = 0x2010
	enDash                 = 0x2013
	emDash                 = 0x2014
	rightSingleQuote       = 0x2019
	noBreakSpace           = 0x00A0
	englishIContractionSet = "m ll ve d"
)

// IsDialogueDash reports whether r is one of the 4 dialogue dash characters: the hyphen-minus, U+2010, U+2013 and U+2014.
func IsDialogueDash(r rune) bool {
	return r == '-' || r == unicodeHyphen || r == enDash || r == emDash
}

// StartsDialogueTurn reports whether text starts with a dialogue dash. The dash must not come before another dash,
// and after its spaces, tabs and U+00A0 a rune must follow that is neither white space nor a number.
// So "- Hi" and "-Hi" start a turn, and "-20 degrees", "--" and "-" do not. Invalid UTF-8 starts no turn.
func StartsDialogueTurn(text string) bool {
	dash, size := utf8.DecodeRuneInString(text)
	if !IsDialogueDash(dash) || !utf8.ValidString(text) {
		return false
	}
	if next, _ := utf8.DecodeRuneInString(text[size:]); IsDialogueDash(next) {
		return false
	}
	rest := strings.TrimLeftFunc(text[size:], func(r rune) bool { return r == ' ' || r == '\t' || r == noBreakSpace })
	r, _ := utf8.DecodeRuneInString(rest)
	return rest != "" && !unicode.IsSpace(r) && !unicode.IsNumber(r)
}

// SplitLeadingBidiControls returns the bidi controls at the start of text and the rest. Invalid UTF-8 has none.
func SplitLeadingBidiControls(text string) (controls, rest string) {
	if !utf8.ValidString(text) {
		return "", text
	}
	i := strings.IndexFunc(text, func(r rune) bool { return !grapheme.IsBidiControl(r) })
	if i < 0 {
		i = len(text)
	}
	return text[:i], text[i:]
}

// RemoveBidiControls removes the bidi controls U+200E, U+200F, U+202A to U+202E and U+2066 to U+2069.
// It keeps every other byte, also of invalid UTF-8.
func RemoveBidiControls(text string) string {
	var b strings.Builder
	copied := 0
	for i, r := range text {
		if grapheme.IsBidiControl(r) {
			b.WriteString(text[copied:i])
			copied = i + utf8.RuneLen(r)
		}
	}
	if copied == 0 {
		return text
	}
	b.WriteString(text[copied:])
	return b.String()
}

// UpperCaseEnglishI writes the English pronoun i and its contractions i'm, i'll, i've and i'd in upper case in the
// runs of one line. It returns a new slice. The i must not follow a letter, a digit, an apostrophe, U+2019, or a
// letter and a dot. It must not come before a letter, a digit, an apostrophe, U+2019, or a dot and a letter.
// The runs count as one text, so a run boundary is no word boundary. For invalid UTF-8, the rule reads ASCII
// letters, ASCII digits and the apostrophe when asciiFallback is set, and nothing changes otherwise.
func UpperCaseEnglishI(runs []string, asciiFallback bool) []string {
	text := strings.Join(runs, "")
	if !strings.Contains(text, "i") {
		return slices.Clone(runs)
	}
	m := englishIMatcher{letter: unicode.IsLetter, word: unicodeWordChar, apostrophe: isApostrophe}
	if !utf8.ValidString(text) {
		if !asciiFallback {
			return slices.Clone(runs)
		}
		m = englishIMatcher{letter: asciiLetter, word: asciiWordChar, apostrophe: func(r rune) bool { return r == '\'' }}
	}
	chars := splitChars(text)

	upper := []byte(text)
	for k, c := range chars {
		if c.r == 'i' && m.matchesAt(chars, k) {
			upper[c.at] = 'I'
		}
	}

	out := make([]string, len(runs))
	at := 0
	for i, run := range runs {
		out[i] = string(upper[at : at+len(run)])
		at += len(run)
	}
	return out
}

type char struct {
	r  rune
	at int
}

// splitChars returns the runes of text, or its bytes for invalid UTF-8. A byte of 0x80 or more is then no ASCII letter.
func splitChars(text string) []char {
	var chars []char
	if utf8.ValidString(text) {
		for i, r := range text {
			chars = append(chars, char{r: r, at: i})
		}
		return chars
	}
	for i := range len(text) {
		chars = append(chars, char{r: rune(text[i]), at: i})
	}
	return chars
}

type englishIMatcher struct {
	letter, word, apostrophe func(rune) bool
}

func (m englishIMatcher) matchesAt(chars []char, k int) bool {
	if k > 0 && m.word(chars[k-1].r) || k > 1 && chars[k-1].r == '.' && m.letter(chars[k-2].r) {
		return false
	}
	return m.boundaryAt(chars, k+1) || m.contractionAt(chars, k+1)
}

// contractionAt reports whether chars[j:] starts with an apostrophe and m, ll, ve or d, followed by a word boundary.
func (m englishIMatcher) contractionAt(chars []char, j int) bool {
	if j >= len(chars) || !m.apostrophe(chars[j].r) {
		return false
	}
	for suffix := range strings.FieldsSeq(englishIContractionSet) {
		if end := j + 1 + len(suffix); end <= len(chars) && runesEqual(chars[j+1:end], suffix) && m.boundaryAt(chars, end) {
			return true
		}
	}
	return false
}

func (m englishIMatcher) boundaryAt(chars []char, j int) bool {
	if j >= len(chars) {
		return true
	}
	if m.word(chars[j].r) {
		return false
	}
	return chars[j].r != '.' || j+1 >= len(chars) || !m.letter(chars[j+1].r)
}

func runesEqual(chars []char, s string) bool {
	for i, r := range s {
		if chars[i].r != r {
			return false
		}
	}
	return true
}

func isApostrophe(r rune) bool { return r == '\'' || r == rightSingleQuote }

func unicodeWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || isApostrophe(r)
}

func asciiLetter(r rune) bool { return r < utf8.RuneSelf && isASCIILetter(byte(r)) }

func asciiWordChar(r rune) bool {
	return asciiLetter(r) || '0' <= r && r <= '9' || r == '\''
}
