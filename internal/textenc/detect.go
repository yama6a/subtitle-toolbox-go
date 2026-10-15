package textenc

import (
	"bytes"
	"cmp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	sampleBytes = 65536
	// scanBytes is the start of the data in which sample looks for lines with a byte from 0x80.
	scanBytes = 16 * sampleBytes

	knownScore   = 1
	unknownScore = -2
	casePenalty  = 2
	// symbolPenalty is the cost of a symbol between 2 letters and of a misplaced Thai mark.
	symbolPenalty = 3
	// minScore compares candidates per non-ASCII rune, because a CJK character takes 2 bytes and a single-byte
	// code page decodes it to 2 runes.
	minScore = 0.5
)

// DetectCodePage returns the code page of data and data decoded as UTF-8. False means that no candidate gives
// plausible text. It decodes the lines with a byte from 0x80 with each candidate and scores the result. Letters of
// a language that uses the code page score. Other letters, symbols, mixed scripts and odd letter case cost.
func DetectCodePage(data []byte) (string, []byte, bool) {
	s := sample(data)
	if len(s) == 0 {
		return "", nil, false
	}
	for _, c := range scoreCandidates(s) {
		if c.score <= 0 || c.broken {
			return "", nil, false
		}
		if text, ok := c.enc.Decode(data); ok {
			return c.name, text, true
		}
		// A CJK file with a byte that its code page cannot decode, such as a cut last character, reads as garbage
		// in any other code page.
		if c.multiByte {
			return "", nil, false
		}
	}
	return "", nil, false
}

type scored struct {
	candidate
	enc    Encoding
	score  float64
	broken bool
}

func scoreCandidates(s []byte) []scored {
	var list []scored
	for _, c := range candidates {
		e, _ := Lookup(c.name)
		score, ok := candidateScore(e, s, c.languages)
		broken := false
		if !ok && c.multiByte {
			score, ok = candidateScore(e, decodableLines(e, s), c.languages)
			broken = true
		}
		if ok {
			list = append(list, scored{candidate: c, enc: e, score: score, broken: broken})
		}
	}
	slices.SortStableFunc(list, func(a, b scored) int { return cmp.Compare(b.score, a.score) })
	return list
}

// ScoreText returns the score per non-ASCII rune of UTF-8 text in the language of languages that fits best.
// It scores as DetectCodePage does, without the cut-off that turns a score of 0.5 or less into 0, as PHP scoreText.
// Text without a non-ASCII rune scores 0. languages holds alphabet codes such as "cs" or "zh-Hans".
func ScoreText(text string, languages []string) float64 {
	sum, n := rawScore(text, languages)
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// sample returns the lines with a byte from 0x80, up to sampleBytes. A cut removes the trailing bytes from 0x40,
// so no CJK character is cut in half: no byte below 0x40 is the second byte of one.
func sample(data []byte) []byte {
	var lines [][]byte
	for line := range bytes.SplitSeq(data[:min(len(data), scanBytes)], []byte("\n")) {
		if slices.ContainsFunc(line, func(b byte) bool { return b >= utf8.RuneSelf }) {
			lines = append(lines, line)
		}
	}
	s := bytes.Join(lines, []byte("\n"))
	if len(s) <= sampleBytes && len(data) <= scanBytes {
		return s
	}
	s = s[:min(len(s), sampleBytes)]
	for len(s) > 0 && s[len(s)-1] >= 0x40 {
		s = s[:len(s)-1]
	}
	return s
}

func decodableLines(e Encoding, s []byte) []byte {
	var lines [][]byte
	for line := range bytes.SplitSeq(s, []byte("\n")) {
		if _, ok := e.Decode(line); ok {
			lines = append(lines, line)
		}
	}
	return bytes.Join(lines, []byte("\n"))
}

func candidateScore(e Encoding, s []byte, languages []string) (float64, bool) {
	text, ok := e.Decode(s)
	if !ok || hasC1(text) {
		return 0, false
	}
	sum, n := rawScore(string(text), languages)
	score := sum / float64(max(1, n))
	if score <= minScore {
		return 0, true
	}
	return score, true
}

func rawScore(text string, languages []string) (float64, int) {
	counts := map[rune]int{}
	n := 0
	for _, r := range text {
		if r >= utf8.RuneSelf {
			counts[r]++
			n++
		}
	}
	letters := map[rune]int{}
	symbols := 0
	for r, c := range counts {
		switch {
		case unicode.IsLetter(r) || unicode.IsMark(r):
			letters[r] = c
		case isPunctuation(r):
			symbols += knownScore * c
		default:
			symbols += unknownScore * c
		}
	}
	return float64(letterScore(letters, languages) + symbols - penalty(text)), n
}

func letterScore(letters map[rune]int, languages []string) int {
	if len(letters) == 0 {
		return 0
	}
	best := 0
	for i, language := range languages {
		score := 0
		for r, c := range letters {
			switch {
			case inAlphabet(language, r):
				score += knownScore * c
			case isRare(language, r):
			default:
				score += unknownScore * c
			}
		}
		if i == 0 || score > best {
			best = score
		}
	}
	return best
}

// penalty returns the cost of odd letter case, mixed scripts, symbols between letters and Thai marks out of place.
func penalty(s string) int {
	text := []rune(s)
	var mixedScripts, symbolsInWords, misplacedMarks int
	for i := range text {
		if isScriptSwitch(text, i) {
			mixedScripts++
		}
		if isSymbolInWord(text, i) {
			symbolsInWords++
		}
		if isMisplacedThaiMark(text, i) {
			misplacedMarks++
		}
	}
	var extraCapitals, lowerStarts int
	for line := range strings.SplitSeq(s, "\n") {
		extra, lowerStart := linePenalty(line)
		extraCapitals += extra
		if lowerStart {
			lowerStarts++
		}
	}
	return extraCapitals + casePenalty*(caseChanges(text)+lowerStarts+mixedScripts) + symbolPenalty*(symbolsInWords+misplacedMarks)
}

// linePenalty returns the capitals of line beyond its small letters, and whether its first letter is a small
// letter outside ASCII. Lines in capitals are common in subtitles, so a line without small letters costs nothing.
func linePenalty(line string) (int, bool) {
	var lower, upper int
	lowerStart, seenLetter := false, false
	for _, r := range line {
		nonASCII := r >= utf8.RuneSelf
		switch {
		case nonASCII && unicode.IsLower(r):
			lower++
		case nonASCII && unicode.IsUpper(r):
			upper++
		}
		if !seenLetter && unicode.IsLetter(r) {
			seenLetter = true
			lowerStart = nonASCII && unicode.IsLower(r)
		}
	}
	if lower == 0 {
		return 0, lowerStart
	}
	return max(0, upper-lower), lowerStart
}

func caseChanges(text []rune) int {
	n := 0
	for i := 0; i+1 < len(text); i++ {
		if unicode.IsLower(text[i]) && unicode.IsUpper(text[i+1]) {
			if text[i] >= utf8.RuneSelf || text[i+1] >= utf8.RuneSelf {
				n++
			}
			i++
		}
	}
	return n
}

func isLatin(r rune) bool {
	return unicode.Is(unicode.Latin, r)
}

func isNonLatin(r rune) bool {
	return unicode.In(r, unicode.Greek, unicode.Cyrillic, unicode.Arabic, unicode.Hebrew, unicode.Thai)
}

func isScriptSwitch(text []rune, i int) bool {
	if i+1 == len(text) {
		return false
	}
	a, b := text[i], text[i+1]
	return isLatin(a) && isNonLatin(b) || isNonLatin(a) && isLatin(b)
}

func isSymbolInWord(text []rune, i int) bool {
	return i > 0 && i+1 < len(text) && unicode.IsLetter(text[i-1]) && unicode.IsLetter(text[i+1]) && isWordSymbol(text[i])
}

// isWordSymbol leaves out CJK punctuation, because CJK text has no spaces between words.
func isWordSymbol(r rune) bool {
	switch {
	case r < utf8.RuneSelf, r == 0x2019, r == 0x00B7, r >= 0x3000 && r <= 0x303F, r >= 0xFF00 && r <= 0xFFEF:
		return false
	default:
		return !unicode.IsLetter(r) && !unicode.IsMark(r) && !unicode.IsSpace(r)
	}
}

// isMisplacedThaiMark reports a Thai vowel or tone mark above or below a letter that follows no consonant, or a
// leading vowel that no consonant follows.
func isMisplacedThaiMark(text []rune, i int) bool {
	r := text[i]
	switch {
	case isThaiMark(r):
		return i == 0 || !isThaiConsonant(text[i-1]) && !isThaiMark(text[i-1])
	case r >= 0x0E40 && r <= 0x0E44:
		return i+1 == len(text) || !isThaiConsonant(text[i+1])
	default:
		return false
	}
}

func isThaiMark(r rune) bool {
	return r == 0x0E31 || r >= 0x0E34 && r <= 0x0E3A || r >= 0x0E47 && r <= 0x0E4E
}

func isThaiConsonant(r rune) bool {
	return r >= 0x0E01 && r <= 0x0E2E
}
