package pac

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/textenc"
)

// Adapted from Subtitle Edit, src/libse/SubtitleFormats/Pac.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/a0dfbfe65f0517ef85b1e69f1606bca93f68ac3c/src/libse/SubtitleFormats/Pac.cs), MIT.

// The detector decodes the text with each code page and checks that the result looks like the script of the code
// page. Greek and Cyrillic store their letters on the ASCII letter bytes, so each script also needs signs that a
// wrong code page rarely gives: Greek accents and Cyrillic punctuation. A code page with such a sign beats one
// without. Of several Latin code pages, textenc.ScoreText decides.

type fit uint8

const (
	noFit fit = iota
	weakFit
	strongFit
)

const (
	minScriptShare      = 0.7
	minRightToLeftShare = 0.6
	minThaiMarkShare    = 0.1
	maxQShare           = 0.015 // Greek read as Latin turns rho into "q" without "u"
	maxBadStartShare    = 0.02  // Latin read as Cyrillic turns "m" and "s" at a word start into a soft sign or yeru
	minPunctuationShare = 0.025 // Russian text has about one period or comma in 25 letters
	maxBadSigmaShare    = 0.05  // Latin read as Greek puts final sigma inside words and medial sigma at word ends
	maxUndefinedShare   = 0.01
	minScoreMargin      = 0.5
	minKoreanByteShare  = 0.9 // Korean in CP949 uses the KS X 1001 bytes from B0A1h
	minHangulShare      = 0.8
	minKanaShare        = 0.2
	minLatinVowels      = 0.2
	maxLatinVowels      = 0.75
	minCyrillicVowels   = 0.25
	maxCyrillicVowels   = 0.65
)

// The vowels of Cyrillic text, written as escapes because Go source holds no Cyrillic text.
const (
	cyrillicVowels = "\u0430\u0435\u0451\u0438\u043E\u0443\u044B\u044D\u044E\u044F\u0410\u0415\u0401\u0418\u041E\u0423\u042B\u042D\u042E\u042F\u0456\u0457\u0454\u0406\u0407\u0404"
	// The soft sign, the hard sign and yeru in lower and upper case.
	cyrillicBadStarts = "\u044C\u044A\u044B\u042C\u042A\u042B"
	// The accented letters of Greek.
	greekAccents = "\u0386\u0388\u0389\u038A\u038C\u038E\u038F\u0390\u03AC\u03AD\u03AE\u03AF\u03B0\u03CA\u03CB\u03CC\u03CD\u03CE"
)

//nolint:gochecknoglobals // lookup tables of the candidate order and the Latin alphabets
var (
	singleByteCandidates = []CodePage{Latin, LatinCzech, LatinTurkish, Greek, Cyrillic, Hebrew, Arabic, Thai}
	doubleByteCandidates = []CodePage{Korean, Japanese, ChineseSimplified, ChineseTraditional}
	latinLanguages       = map[CodePage][]string{
		Latin: {
			"fr", "de", "es", "pt", "it", "nl", "da", "sv", "fi", "is", "ca", "sq", "pl", "hu", "cs", "sk", "sl", "hr",
			"ro", "lt", "lv", "et",
		},
		LatinCzech:   {"cs", "sk", "pl", "hu", "sl", "hr", "ro", "de"},
		LatinTurkish: {"tr"},
	}
)

// scan returns the double-byte bytes of all lines and whether any run is PAC Unicode.
func scan(lines [][]run) ([]byte, bool) {
	var doubleBytes []byte
	for _, l := range lines {
		for _, r := range l {
			switch r.mode {
			case unicodeMode:
				return nil, true
			case doubleMode:
				doubleBytes = append(doubleBytes, r.data...)
			case singleMode:
			}
		}
	}
	return doubleBytes, false
}

// onlyDouble returns the lines with their double-byte runs only.
func onlyDouble(lines [][]run) [][]run {
	out := make([][]run, len(lines))
	for i, l := range lines {
		out[i] = slices.DeleteFunc(slices.Clone(l), func(r run) bool { return r.mode != doubleMode })
	}
	return out
}

// contest holds the result of the candidates, in candidate order. A candidate whose text equals the text of an
// earlier candidate is left out.
type contest struct {
	order     []CodePage
	fits      map[CodePage]fit
	decodings map[CodePage]string
}

func newContest(candidates []CodePage, lines [][]run, doubleBytes []byte) contest {
	k := contest{fits: map[CodePage]fit{}, decodings: map[CodePage]string{}}
	for _, c := range candidates {
		text, undefined := decodeAll(c, lines)
		if slices.ContainsFunc(k.order, func(o CodePage) bool { return k.decodings[o] == text }) {
			continue
		}
		k.order = append(k.order, c)
		k.decodings[c] = text
		k.fits[c] = noFit
		if float64(undefined) <= maxUndefinedShare*float64(utf8.RuneCountInString(text)) {
			k.fits[c] = scriptFit(c, text, doubleBytes)
		}
	}
	return k
}

// decodeAll decodes all lines with c, joined with LF. It returns the text and the number of undefined bytes.
func decodeAll(c CodePage, lines [][]run) (string, int) {
	texts := make([]string, len(lines))
	undefined := 0
	for i, l := range lines {
		var missing int
		texts[i], missing = c.decodeLine(l)
		undefined += missing
	}
	return strings.Join(texts, "\n"), undefined
}

// top returns the candidates with the best fit, and that fit.
func (k contest) top() ([]CodePage, fit) {
	best := noFit
	for _, f := range k.fits {
		best = max(best, f)
	}
	var top []CodePage
	for _, c := range k.order {
		if k.fits[c] == best {
			top = append(top, c)
		}
	}
	return top, best
}

// detectCodePage returns the code page of the text lines and a reason when no code page or more than one fits.
func detectCodePage(lines [][]run) (CodePage, string) {
	doubleBytes, hasUnicode := scan(lines)
	if hasUnicode {
		return Unicode, ""
	}
	// The double-byte text of a Chinese, Korean or Japanese file decides. Its single-byte text is Latin.
	candidates := singleByteCandidates
	if len(doubleBytes) > 0 {
		candidates = doubleByteCandidates
		lines = onlyDouble(lines)
	}
	k := newContest(candidates, lines, doubleBytes)
	top, best := k.top()
	if best == noFit {
		return candidates[0], "No code page gives text in its script."
	}
	// Korean and Japanese read as Chinese give Han characters, but Chinese read as them rarely gives kana or Hangul.
	kanaOrHangul := slices.DeleteFunc(slices.Clone(top), func(c CodePage) bool { return c != Korean && c != Japanese })
	switch {
	case len(kanaOrHangul) == 1:
		return kanaOrHangul[0], ""
	case len(top) == 1:
		return top[0], ""
	}
	return byScore(top, k.decodings)
}

// byScore picks one of several Latin code pages by the textenc.ScoreText score. Other code pages stay unclear.
func byScore(top []CodePage, decodings map[CodePage]string) (CodePage, string) {
	names := make([]string, len(top))
	for i, c := range top {
		names[i] = string(c)
	}
	unclear := "These code pages fit: " + strings.Join(names, ", ") + "."
	scores := make([]float64, len(top))
	for i, c := range top {
		languages, ok := latinLanguages[c]
		if !ok {
			return top[0], unclear
		}
		scores[i] = textenc.ScoreText(decodings[c], languages)
	}
	best, second := leaders(scores)
	if scores[best]-scores[second] >= minScoreMargin {
		return top[best], ""
	}
	return top[0], unclear
}

// leaders returns the indexes of the highest and the second highest score. A tie keeps the earlier index first.
func leaders(scores []float64) (int, int) {
	best, second := 0, -1
	for i := 1; i < len(scores); i++ {
		switch {
		case scores[i] > scores[best]:
			best, second = i, best
		case second < 0 || scores[i] > scores[second]:
			second = i
		}
	}
	return best, second
}

// fitFuncs holds the script check of each code page. A Latin code page has latinFit.
//
//nolint:gochecknoglobals // lookup table of the script checks
var fitFuncs = map[CodePage]func(text string, letters []rune, doubleBytes []byte) fit{
	Greek:              greekFit,
	Cyrillic:           cyrillicFit,
	Hebrew:             hebrewFit,
	Arabic:             arabicFit,
	Thai:               thaiFit,
	Korean:             koreanFit,
	Japanese:           japaneseFit,
	ChineseSimplified:  chineseFit(ChineseSimplified),
	ChineseTraditional: chineseFit(ChineseTraditional),
}

func scriptFit(c CodePage, text string, doubleBytes []byte) fit {
	var letters []rune
	for _, r := range text {
		if unicode.IsLetter(r) {
			letters = append(letters, r)
		}
	}
	if len(letters) == 0 {
		return weakFit
	}
	if f, ok := fitFuncs[c]; ok {
		return f(text, letters, doubleBytes)
	}
	return latinFit(text, letters, doubleBytes)
}

// share returns the part of the letters that in accepts.
func share(letters []rune, in func(rune) bool) float64 {
	n := 0
	for _, r := range letters {
		if in(r) {
			n++
		}
	}
	return float64(n) / float64(len(letters))
}

func inScript(table *unicode.RangeTable) func(rune) bool {
	return func(r rune) bool { return unicode.Is(table, r) }
}

func isASCIILetter(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

func latinFit(text string, letters []rune, _ []byte) fit {
	ascii, vowels := 0, 0
	for _, r := range letters {
		if isASCIILetter(r) {
			ascii++
		}
		if strings.ContainsRune("aeiouyAEIOUY", r) {
			vowels++
		}
	}
	if ascii == 0 || float64(ascii) < minScriptShare*float64(len(letters)) {
		return noFit
	}
	vowelShare := float64(vowels) / float64(ascii)
	qShare := float64(qWithoutU(text)) / float64(len(letters))
	if vowelShare >= minLatinVowels && vowelShare <= maxLatinVowels && qShare <= maxQShare {
		return strongFit
	}
	return noFit
}

// qWithoutU counts the letters q and Q that no u or U follows.
func qWithoutU(text string) int {
	n := 0
	for i := range len(text) {
		if text[i]|0x20 == 'q' && (i+1 == len(text) || text[i+1]|0x20 != 'u') {
			n++
		}
	}
	return n
}

func greekFit(text string, letters []rune, _ []byte) fit {
	sigmas, badSigmas := sigmaCounts([]rune(text))
	if share(letters, inScript(unicode.Greek)) < minScriptShare || float64(badSigmas) > maxBadSigmaShare*float64(sigmas) {
		return noFit
	}
	if strings.ContainsAny(text, greekAccents) {
		return strongFit
	}
	return weakFit
}

// sigmaCounts returns the number of sigmas and the number of misplaced sigmas: a final sigma before a letter and a
// medial sigma that no letter follows.
func sigmaCounts(runes []rune) (int, int) {
	sigmas, bad := 0, 0
	for i, r := range runes {
		if r != '\u03C3' && r != '\u03C2' {
			continue
		}
		sigmas++
		followedByLetter := i+1 < len(runes) && unicode.IsLetter(runes[i+1])
		if followedByLetter == (r == '\u03C3') {
			continue
		}
		bad++
	}
	return sigmas, bad
}

func cyrillicFit(text string, letters []rune, _ []byte) fit {
	words, badStarts := wordCounts(text)
	vowels := share(letters, func(r rune) bool { return strings.ContainsRune(cyrillicVowels, r) })
	if share(letters, inScript(unicode.Cyrillic)) < minScriptShare || vowels < minCyrillicVowels || vowels > maxCyrillicVowels ||
		float64(badStarts) > maxBadStartShare*float64(words) {
		return noFit
	}
	// The Cyrillic code page stores the period and the comma on the bytes of "&" and "^", which Latin text rarely has.
	punctuation := strings.Count(text, ".") + strings.Count(text, ",")
	if float64(punctuation) >= minPunctuationShare*float64(len(letters)) {
		return strongFit
	}
	return weakFit
}

// wordCounts returns the number of words, which are runs of letters, and the number of words that start with the
// soft sign, the hard sign or yeru.
func wordCounts(text string) (int, int) {
	words, badStarts := 0, 0
	previousLetter := false
	for _, r := range text {
		letter := unicode.IsLetter(r)
		if letter && !previousLetter {
			words++
			if strings.ContainsRune(cyrillicBadStarts, r) {
				badStarts++
			}
		}
		previousLetter = letter
	}
	return words, badStarts
}

func hebrewFit(_ string, letters []rune, _ []byte) fit {
	return atLeast(share(letters, inScript(unicode.Hebrew)), minRightToLeftShare)
}

func arabicFit(_ string, letters []rune, _ []byte) fit {
	return atLeast(share(letters, inScript(unicode.Arabic)), minRightToLeftShare)
}

func inRanges(r rune, ranges ...[2]rune) bool {
	for _, b := range ranges {
		if r >= b[0] && r <= b[1] {
			return true
		}
	}
	return false
}

func isHangul(r rune) bool { return inRanges(r, [2]rune{0xAC00, 0xD7A3}) }

func isKana(r rune) bool { return inRanges(r, [2]rune{0x3040, 0x30FF}) }

// isHan matches the CJK ideographs only, because unicode.Han also holds the CJK punctuation.
func isHan(r rune) bool { return inRanges(r, [2]rune{0x3400, 0x4DBF}, [2]rune{0x4E00, 0x9FFF}) }

func count(text string, in func(rune) bool) int {
	n := 0
	for _, r := range text {
		if in(r) {
			n++
		}
	}
	return n
}

// countPairs counts the non-overlapping pairs of bytes of b whose first byte is in the first range and second byte
// in the second range.
func countPairs(b []byte, first, second [2]byte) int {
	n := 0
	for i := 0; i+1 < len(b); {
		if b[i] >= first[0] && b[i] <= first[1] && b[i+1] >= second[0] && b[i+1] <= second[1] {
			n++
			i += 2
			continue
		}
		i++
	}
	return n
}

func koreanFit(text string, _ []rune, doubleBytes []byte) fit {
	hangul, han := count(text, isHangul), count(text, isHan)
	pairs := countPairs(doubleBytes, [2]byte{0x81, 0xFE}, [2]byte{0x00, 0xFF})
	common := countPairs(doubleBytes, [2]byte{0xB0, 0xFE}, [2]byte{0xA1, 0xFE})
	if hangul == 0 || float64(hangul) < minHangulShare*float64(hangul+han) || float64(common) < minKoreanByteShare*float64(pairs) {
		return noFit
	}
	// Korean puts spaces between words. Chinese read as Korean gives no spaces.
	if hasSpaceBetween(text, isHangul) {
		return strongFit
	}
	return weakFit
}

// hasSpaceBetween reports whether a space stands between 2 characters that in accepts.
func hasSpaceBetween(text string, in func(rune) bool) bool {
	runes := []rune(text)
	for i := 0; i+2 < len(runes); i++ {
		if in(runes[i]) && runes[i+1] == ' ' && in(runes[i+2]) {
			return true
		}
	}
	return false
}

func japaneseFit(text string, _ []rune, _ []byte) fit {
	kana, han := count(text, isKana), count(text, isHan)
	if kana > 0 && float64(kana) >= minKanaShare*float64(kana+han) {
		return strongFit
	}
	return noFit
}

// chineseFit uses that GB2312 text has second bytes from A1h only, and Big5 text has second bytes from 40h to 7Eh too.
func chineseFit(c CodePage) func(string, []rune, []byte) fit {
	return func(text string, _ []rune, doubleBytes []byte) fit {
		if count(text, isHan) == 0 || count(text, func(r rune) bool { return isHangul(r) || isKana(r) }) > 0 {
			return noFit
		}
		lowSecondByte := countPairs(doubleBytes, [2]byte{0x81, 0xFE}, [2]byte{0x40, 0x7E}) > 0
		if lowSecondByte == (c == ChineseTraditional) {
			return strongFit
		}
		return weakFit
	}
}

// thaiFit expects vowels and tone marks. Hebrew read as Thai gives consonants only.
func thaiFit(text string, letters []rune, _ []byte) fit {
	marks := count(text, func(r rune) bool { return r >= 0x0E30 && r <= 0x0E4E })
	all := count(text, func(r rune) bool { return r >= 0x0E01 && r <= 0x0E4E })
	if share(letters, inScript(unicode.Thai)) >= minScriptShare && float64(marks) >= minThaiMarkShare*float64(all) {
		return strongFit
	}
	return noFit
}

func atLeast(share, minimum float64) fit {
	if share >= minimum {
		return strongFit
	}
	return noFit
}
