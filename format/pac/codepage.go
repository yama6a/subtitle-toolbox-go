package pac

import (
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// Adapted from Subtitle Edit, src/libse/SubtitleFormats/Pac.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/a0dfbfe65f0517ef85b1e69f1606bca93f68ac3c/src/libse/SubtitleFormats/Pac.cs), MIT.

const replacement = "?"

// doubleByteEncodings holds the x/text encoding of the double-byte text of each double-byte code page.
// x/text follows the WHATWG indexes, which differ from iconv in a few codes.
//
//nolint:gochecknoglobals // lookup table of the double-byte encodings
var doubleByteEncodings = map[CodePage]encoding.Encoding{
	ChineseTraditional: traditionalchinese.Big5,
	ChineseSimplified:  simplifiedchinese.GBK,
	Korean:             korean.EUCKR,
	Japanese:           japanese.ShiftJIS,
}

// singleByte returns the code page of the single-byte text: Latin for the double-byte code pages and for Unicode.
func (c CodePage) singleByte() CodePage {
	if _, ok := doubleByteEncodings[c]; ok || c == Unicode {
		return Latin
	}
	return c
}

func (c CodePage) isLatin() bool {
	switch c.singleByte() {
	case Latin, LatinCzech, LatinTurkish:
		return true
	case CodePageAuto, Greek, Cyrillic, Arabic, Hebrew, Thai, ChineseTraditional, ChineseSimplified, Korean, Japanese, Unicode:
		return false
	}
	return false
}

// latinCharmap returns the table of the bytes from A0h in a Latin code page.
func (c CodePage) latinCharmap() *charmap.Charmap {
	switch c {
	case LatinCzech:
		return charmap.ISO8859_2
	case LatinTurkish:
		return charmap.ISO8859_9
	case CodePageAuto, Latin, Greek, Cyrillic, Arabic, Hebrew, Thai, ChineseTraditional, ChineseSimplified, Korean, Japanese, Unicode:
		return charmap.ISO8859_1
	}
	return charmap.ISO8859_1
}

// rightToLeftLine puts the Latin letters and digits of an Arabic or Hebrew line in the order of display.
func (c CodePage) rightToLeftLine(text string) string {
	switch c {
	case Arabic:
		return rightToLeft(text, arabicLatin)
	case Hebrew:
		return rightToLeft(text, hebrewLatin)
	case CodePageAuto, Latin, LatinCzech, LatinTurkish, Greek, Cyrillic, Thai, ChineseTraditional, ChineseSimplified, Korean, Japanese, Unicode:
		return text
	}
	return text
}

// decodeLine decodes the runs of a text line. It returns the text and the number of bytes that the code page lacks.
func (c CodePage) decodeLine(runs []run) (string, int) {
	var text strings.Builder
	undefined := 0
	for _, r := range runs {
		part, missing := c.decodeRun(r)
		text.WriteString(part)
		undefined += missing
	}
	return text.String(), undefined
}

func (c CodePage) decodeRun(r run) (string, int) {
	switch r.mode {
	case unicodeMode:
		return decodeUnicode(r.data), 0
	case doubleMode:
		return c.decodeDoubleByte(r.data)
	case singleMode:
		return c.singleByte().decodeSingleByte(r.data)
	}
	return "", 0
}

// encodeLine encodes a text line: as UTF-8 for Unicode, as double-byte text for a line with a character outside
// ASCII in a double-byte code page, else as single-byte text.
func (c CodePage) encodeLine(text string) []byte {
	if c == Unicode {
		out := append([]byte(unicodeCode), strings.ReplaceAll(text, ".", "\xFF")...)
		return append(out, '.')
	}
	if _, ok := doubleByteEncodings[c]; !ok || isPrintableASCII(text) {
		return c.singleByte().encodeSingleByte(text)
	}
	return c.encodeDoubleByte(text)
}

func isPrintableASCII(s string) bool {
	for _, r := range s {
		if r < 0x20 || r > 0x7E {
			return false
		}
	}
	return true
}
