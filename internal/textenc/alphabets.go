package textenc

import (
	_ "embed"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/encoding/korean"
)

// The Han lists live in text files, because the gosmopolitan linter rejects Han text in Go source.
// Japanese uses both lists.
var (
	//go:embed han_simplified.txt
	hanSimplified string

	// The Traditional forms come from STCharacters.txt and TWVariants.txt of OpenCC 8cf737a4, Apache-2.0, see THIRD_PARTY_NOTICES.md.
	//
	//go:embed han_traditional.txt
	hanTraditional string
)

type candidate struct {
	name      string
	languages []string
	multiByte bool
}

// candidates lists the code pages that detection tries. The order breaks ties. Windows-1252 decodes ISO-8859-1 text
// the same, so ISO-8859-1 never wins a tie. CP932 and CP950 are the Windows forms of Shift_JIS and Big5.
//
//nolint:gochecknoglobals // ordered candidate table
var candidates = []candidate{
	{name: "Windows-1252", languages: []string{"fr", "de", "es", "pt", "it", "nl", "da", "sv", "fi", "is", "ca", "sq"}},
	{name: "Windows-1250", languages: []string{"cs", "sk", "pl", "hu", "sl", "hr", "ro", "sq", "de"}},
	{name: "Windows-1251", languages: []string{"ru", "uk", "be", "bg", "sr", "mk"}},
	{name: "Windows-1254", languages: []string{"tr"}},
	{name: "Windows-1253", languages: []string{"el"}},
	{name: "Windows-1256", languages: []string{"ar"}},
	{name: "Windows-1255", languages: []string{"he"}},
	{name: "Windows-1257", languages: []string{"lt", "lv", "et"}},
	{name: "Windows-1258", languages: []string{"vi"}},
	{name: "Windows-874", languages: []string{"th"}},
	{name: "KOI8-R", languages: []string{"ru", "bg"}},
	{name: "KOI8-U", languages: []string{"uk", "ru", "be"}},
	{name: "ISO-8859-1", languages: []string{"fr", "de", "es", "pt", "it", "nl", "da", "sv", "fi", "is", "ca", "sq"}},
	{name: "ISO-8859-2", languages: []string{"cs", "sk", "pl", "hu", "sl", "hr", "ro", "sq", "de"}},
	{name: "ISO-8859-5", languages: []string{"ru", "uk", "be", "bg", "sr", "mk"}},
	{name: "ISO-8859-6", languages: []string{"ar"}},
	{name: "ISO-8859-7", languages: []string{"el"}},
	{name: "ISO-8859-9", languages: []string{"tr"}},
	{name: "CP949", languages: []string{"ko"}, multiByte: true},
	{name: "CP932", languages: []string{"ja"}, multiByte: true},
	{name: "EUC-JP", languages: []string{"ja"}, multiByte: true},
	{name: "GBK", languages: []string{"zh-Hans"}, multiByte: true},
	{name: "CP950", languages: []string{"zh-Hant"}, multiByte: true},
}

// alphabets lists the letters outside ASCII of the standard spelling of each language.
//
//nolint:gochecknoglobals // lookup table of the alphabets
var alphabets = map[string]string{
	"fr": "àâæçéèêëîïôœùûüÿÀÂÆÇÉÈÊËÎÏÔŒÙÛÜŸ",
	"de": "äöüßÄÖÜ",
	"es": "áéíñóúüÁÉÍÑÓÚÜ",
	"pt": "áâãàçéêíóôõúÁÂÃÀÇÉÊÍÓÔÕÚ",
	"it": "àèéìíîòóùúÀÈÉÌÍÎÒÓÙÚ",
	"nl": "áéèëíïóöúüÁÉÈËÍÏÓÖÚÜ",
	"da": "æøåéÆØÅÉ",
	"sv": "åäöéÅÄÖÉ",
	"fi": "äöåšžÄÖÅŠŽ",
	"is": "áðéíóúýþæöÁÐÉÍÓÚÝÞÆÖ",
	"ca": "àçéèíïòóúüÀÇÉÈÍÏÒÓÚÜ",
	"sq": "çëÇË",
	"cs": "áčďéěíňóřšťúůýžÁČĎÉĚÍŇÓŘŠŤÚŮÝŽ",
	"sk": "áäčďéíĺľňóôŕšťúýžÁÄČĎÉÍĹĽŇÓÔŔŠŤÚÝŽ",
	"pl": "ąćęłńóśźżĄĆĘŁŃÓŚŹŻ",
	"hu": "áéíóöőúüűÁÉÍÓÖŐÚÜŰ",
	"sl": "čšžČŠŽ",
	"hr": "čćđšžČĆĐŠŽ",
	"ro": "ăâîșşțţĂÂÎȘŞȚŢ",
	"tr": "çğıöşüâîûÇĞÖŞÜÂÎÛİ",
	"lt": "ąčęėįšųūžĄČĘĖĮŠŲŪŽ",
	"lv": "āčēģīķļņšūžĀČĒĢĪĶĻŅŠŪŽ",
	"et": "äõöüšžÄÕÖÜŠŽ",
	"vi": "àáâãèéêìíòóôõùúýăđĩũơưạảấầẩẫậắằẳẵặẹẻẽếềểễệỉịọỏốồổỗộớờởỡợụủứừửữựỳỵỷỹ" +
		"ÀÁÂÃÈÉÊÌÍÒÓÔÕÙÚÝĂĐĨŨƠƯẠẢẤẦẨẪẬẮẰẲẴẶẸẺẼẾỀỂỄỆỈỊỌỎỐỒỔỖỘỚỜỞỠỢỤỦỨỪỬỮỰỲỴỶỸ",
	"el": "αβγδεζηθικλμνξοπρσςτυφχψωάέήίόύώϊϋΐΰΑΒΓΔΕΖΗΘΙΚΛΜΝΞΟΠΡΣΤΥΦΧΨΩΆΈΉΊΌΎΏΪΫ",
	"ru": "абвгдеёжзийклмнопрстуфхцчшщъыьэюяАБВГДЕЁЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯ",
	"uk": "абвгґдеєжзиіїйклмнопрстуфхцчшщьюяАБВГҐДЕЄЖЗИІЇЙКЛМНОПРСТУФХЦЧШЩЬЮЯ",
	"be": "абвгдеёжзійклмнопрстуўфхцчшыьэюяАБВГДЕЁЖЗІЙКЛМНОПРСТУЎФХЦЧШЫЬЭЮЯ",
	"bg": "абвгдежзийклмнопрстуфхцчшщъьюяАБВГДЕЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЬЮЯ",
	"sr": "абвгдђежзијклљмнњопрстћуфхцчџшАБВГДЂЕЖЗИЈКЛЉМНЊОПРСТЋУФХЦЧЏШ",
	"mk": "абвгдѓежзѕијклљмнњопрстќуфхцчџшАБВГДЃЕЖЗЅИЈКЛЉМНЊОПРСТЌУФХЦЧЏШ",
}

// ranges holds the Arabic, Hebrew and Thai letters, vowel marks and tone marks, and the Japanese kana.
//
//nolint:gochecknoglobals // lookup table of the alphabets
var ranges = map[string][][2]rune{
	"ar": {
		{0x0621, 0x063A},
		{0x0640, 0x0652},
		{0x0679, 0x0679},
		{0x067E, 0x067E},
		{0x0686, 0x0686},
		{0x0688, 0x0688},
		{0x0691, 0x0691},
		{0x0698, 0x0698},
		{0x06A9, 0x06A9},
		{0x06AF, 0x06AF},
		{0x06BA, 0x06BA},
		{0x06BE, 0x06BE},
		{0x06C1, 0x06C1},
		{0x06CC, 0x06CC},
		{0x06D2, 0x06D2},
	},
	"he": {{0x05B0, 0x05C7}, {0x05D0, 0x05EA}},
	"th": {{0x0E01, 0x0E3A}, {0x0E40, 0x0E5B}},
	"ja": {{0x3005, 0x3005}, {0x3041, 0x3096}, {0x309D, 0x309E}, {0x30A1, 0x30FA}, {0x30FC, 0x30FE}},
}

// punctuation lists the symbols outside ASCII that subtitles use: quotes, dashes, currency, CJK punctuation and
// full-width forms.
//
//nolint:gochecknoglobals // lookup table of the symbols
var punctuation = []rune{
	0x00A0, 0x2018, 0x2019, 0x201A, 0x201C, 0x201D, 0x201E, 0x2013, 0x2014, 0x2026, 0x00AB, 0x00BB, 0x2039, 0x203A,
	0x00BF, 0x00A1, 0x00B0, 0x00B7, 0x20AC, 0x00A3, 0x00A7, 0x00A9, 0x00AE, 0x2122, 0x2022, 0x00B4, 0x00D7, 0x00B1,
	0x00BD, 0x00BC, 0x00BE, 0x2116, 0x00A2, 0x00A5, 0x00B9, 0x00B2, 0x00B3, 0x00B6, 0x266A, 0x2027, 0x3000, 0x3001,
	0x3002, 0x3003, 0x3008, 0x3009, 0x300A, 0x300B, 0x300C, 0x300D, 0x300E, 0x300F, 0x3010, 0x3011, 0x3014, 0x3015,
	0x301C, 0x30FB, 0xFF01, 0xFF02, 0xFF03, 0xFF05, 0xFF06, 0xFF07, 0xFF08, 0xFF09, 0xFF0A, 0xFF0B, 0xFF0C, 0xFF0D,
	0xFF0E, 0xFF0F, 0xFF1A, 0xFF1B, 0xFF1C, 0xFF1D, 0xFF1E, 0xFF1F, 0xFF20, 0xFF3B, 0xFF3D, 0xFF3F, 0xFF5B, 0xFF5D,
	0xFF5E, 0xFF10, 0xFF11, 0xFF12, 0xFF13, 0xFF14, 0xFF15, 0xFF16, 0xFF17, 0xFF18, 0xFF19,
}

func inAlphabet(language string, r rune) bool {
	switch language {
	case "zh-Hans":
		return strings.ContainsRune(hanSimplified, r)
	case "zh-Hant":
		return strings.ContainsRune(hanTraditional, r)
	case "ja":
		return strings.ContainsRune(hanSimplified, r) || strings.ContainsRune(hanTraditional, r) || inRanges(ranges[language], r)
	case "ko":
		return isKSX1001Syllable(r)
	default:
		return strings.ContainsRune(alphabets[language], r) || inRanges(ranges[language], r)
	}
}

// isRare reports a letter of the script of language that its alphabet lacks. It adds 0 to the score, not -2.
func isRare(language string, r rune) bool {
	switch language {
	case "zh-Hans", "zh-Hant", "ja":
		return unicode.Is(unicode.Han, r)
	case "ko":
		return unicode.Is(unicode.Hangul, r)
	default:
		return false
	}
}

func isPunctuation(r rune) bool {
	return slices.Contains(punctuation, r)
}

func inRanges(rs [][2]rune, r rune) bool {
	for _, rg := range rs {
		if r >= rg[0] && r <= rg[1] {
			return true
		}
	}
	return false
}

// isKSX1001Syllable reports the 2,350 common syllables of KS X 1001, which glibc EUC-KR encodes. The CP949 form of
// each has a first byte from 0xB0 and a second byte from 0xA1. The other CP949 syllables are rare.
func isKSX1001Syllable(r rune) bool {
	if r < 0xAC00 || r > 0xD7A3 {
		return false
	}
	b, err := korean.EUCKR.NewEncoder().Bytes([]byte(string(r)))
	return err == nil && len(b) == 2 && b[0] >= 0xB0 && b[1] >= 0xA1
}
