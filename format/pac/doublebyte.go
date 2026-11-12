package pac

import (
	"bytes"
	"maps"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// Adapted from Subtitle Edit, src/libse/SubtitleFormats/Pac.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/a0dfbfe65f0517ef85b1e69f1606bca93f68ac3c/src/libse/SubtitleFormats/Pac.cs), MIT.

// utf8Lead describes the sequences of a range of lead bytes: their length and the range of the second byte.
type utf8Lead struct {
	first, last byte
	length      int
	lo, hi      byte
}

// utf8Leads holds the lead bytes of well-formed UTF-8, as table 3-7 of the Unicode standard lists them.
//
//nolint:gochecknoglobals // lookup table of UTF-8 lead bytes
var utf8Leads = []utf8Lead{
	{0xC2, 0xDF, 2, 0x80, 0xBF},
	{0xE0, 0xE0, 3, 0xA0, 0xBF},
	{0xE1, 0xEC, 3, 0x80, 0xBF},
	{0xED, 0xED, 3, 0x80, 0x9F},
	{0xEE, 0xEF, 3, 0x80, 0xBF},
	{0xF0, 0xF0, 4, 0x90, 0xBF},
	{0xF1, 0xF3, 4, 0x80, 0xBF},
	{0xF4, 0xF4, 4, 0x80, 0x8F},
}

// decodeUnicode reads UTF-8 with FFh as a period. Each ill-formed sequence becomes "?", as PHP mb_scrub writes it.
// A cut sequence that is valid so far, such as E4 BD, counts as one.
func decodeUnicode(b []byte) string {
	var out strings.Builder
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		switch {
		case b[i] == 0xFF:
			out.WriteByte('.')
			i++
		case r != utf8.RuneError || size > 1:
			out.Write(b[i : i+size])
			i += size
		default:
			out.WriteString(replacement)
			i += invalidLength(b[i:])
		}
	}
	return out.String()
}

// invalidLength returns the length of the ill-formed sequence at the start of b: the lead byte and the
// continuation bytes after it that a well-formed sequence allows.
func invalidLength(b []byte) int {
	for _, l := range utf8Leads {
		if b[0] < l.first || b[0] > l.last {
			continue
		}
		n := 1
		lo, hi := l.lo, l.hi
		for n < l.length && n < len(b) && b[n] >= lo && b[n] <= hi {
			n++
			lo, hi = 0x80, 0xBF
		}
		return n
	}
	return 1
}

// decodeDoubleByte reads pairs of bytes. 00h before a byte and a byte below 80h are single ASCII characters.
// Other bytes go in pairs through the decoder of the code page, or Big5 for a code page without one.
func (c CodePage) decodeDoubleByte(b []byte) (string, int) {
	enc, ok := doubleByteEncodings[c]
	if !ok {
		enc = traditionalchinese.Big5
	}
	dec := enc.NewDecoder()
	var text strings.Builder
	undefined := 0
	for i := 0; i < len(b); i++ {
		if b[i] < 0x80 {
			if b[i] == 0 {
				i++
			}
			if i < len(b) && b[i] >= 0x20 && b[i] <= 0x7E {
				text.WriteByte(b[i])
			}
			continue
		}
		out, ok := decodeBytes(dec, b[i:min(i+2, len(b))])
		i++
		if !ok {
			undefined++
			continue
		}
		text.Write(out)
	}
	return text.String(), undefined
}

// decodeBytes decodes a pair of bytes. It fails for a pair that the code page does not define.
func decodeBytes(dec *encoding.Decoder, pair []byte) ([]byte, bool) {
	out, err := dec.Bytes(pair)
	return out, err == nil && !bytes.ContainsRune(out, utf8.RuneError) && !hasC1(out)
}

// encodeDoubleByte writes the line as double-byte text: per character the 2-byte code, or 00h and the ASCII byte,
// or 00h and "?".
func (c CodePage) encodeDoubleByte(text string) []byte {
	out := []byte(doubleByteCode)
	encoder := doubleByteEncodings[c].NewEncoder()
	for _, r := range text {
		if r < utf8.RuneSelf {
			out = append(out, 0, byte(r))
			continue
		}
		pair, ok := c.encodeDouble(encoder, r)
		if !ok {
			out = append(out, 0, '?')
			continue
		}
		out = append(out, pair...)
	}
	return out
}

// encodeDouble returns the 2 bytes of r. It returns false for a character that the code page lacks or writes in
// 1 byte.
func (c CodePage) encodeDouble(encoder *encoding.Encoder, r rune) ([]byte, bool) {
	var codes map[rune][2]byte
	switch c {
	case ChineseTraditional:
		codes = big5Codes()
	case Japanese:
		codes = shiftJISCodes()
	case CodePageAuto, Latin, LatinCzech, LatinTurkish, Greek, Cyrillic, Arabic, Hebrew, Thai, ChineseSimplified, Korean, Unicode:
		pair, err := encoder.Bytes([]byte(string(r)))
		return pair, err == nil && len(pair) == 2 && pair[0] != lineStart
	}
	pair, ok := codes[r]
	return pair[:], ok && pair[0] != lineStart
}

// The x/text encoders write a different code than iconv for a character that has 2 codes. x/text writes the Hong
// Kong extension code FABDh for the Big5 character U+5305, where iconv CP950 writes A55Dh. For Shift_JIS it writes
// the NEC selected IBM extension where iconv CP932 writes the IBM extension. So these 2 code pages write through a
// table of the codes that iconv writes, built from the x/text decoder.
//
//nolint:gochecknoglobals // tables built once from the x/text decoders
var (
	big5Codes = sync.OnceValue(func() map[rune][2]byte {
		return codeIndex(traditionalchinese.Big5, big5Allowed, neverLate)
	})
	shiftJISCodes = sync.OnceValue(func() map[rune][2]byte {
		codes := codeIndex(japanese.ShiftJIS, anyCode, necSelected)
		maps.Copy(codes, shiftJISOneWay)
		return codes
	})
)

// shiftJISOneWay holds the characters that iconv CP932 writes as a code that it reads as another character: the
// wave dash, the double vertical line, the minus sign, the cent, pound and not signs, and the em dash.
//
//nolint:gochecknoglobals // lookup table of one-way CP932 codes
var shiftJISOneWay = map[rune][2]byte{
	0x301C: {0x81, 0x60}, 0x2016: {0x81, 0x61}, 0x2212: {0x81, 0x7C}, 0x00A2: {0x81, 0x91},
	0x00A3: {0x81, 0x92}, 0x00AC: {0x81, 0xCA}, 0x2014: {0x81, 0x5C},
}

// big5Allowed accepts the codes of CP950. A1h to F9h are its lead bytes. C6A1h to C8FEh are user-defined in CP950,
// and A2CCh and A2CEh repeat U+5341 and U+5345.
func big5Allowed(lead, trail byte) bool {
	switch {
	case lead < 0xA1 || lead > 0xF9, lead == 0xC7, lead == 0xC8:
		return false
	case lead == 0xC6 && trail >= 0xA1:
		return false
	case lead == 0xA2 && (trail == 0xCC || trail == 0xCE):
		return false
	}
	return true
}

func anyCode(_, _ byte) bool { return true }

func neverLate(byte) bool { return false }

// necSelected reports whether the lead byte starts the NEC selected IBM extension, which CP932 writes only when
// no other code holds the character.
func necSelected(lead byte) bool { return lead == 0xED || lead == 0xEE }

// codeIndex maps each character that the double-byte decoder reads from 2 bytes to its lowest code that allowed
// accepts. A code whose lead byte late accepts counts only when no other code holds the character.
func codeIndex(enc encoding.Encoding, allowed func(lead, trail byte) bool, late func(lead byte) bool) map[rune][2]byte {
	dec := enc.NewDecoder()
	index := map[rune][2]byte{}
	isLate := map[rune]bool{}
	for lead := 0x81; lead <= 0xFE; lead++ {
		for trail := 0x40; trail <= 0xFE; trail++ {
			if !allowed(byte(lead), byte(trail)) {
				continue
			}
			r, ok := decodePair(dec, byte(lead), byte(trail))
			if !ok {
				continue
			}
			if _, known := index[r]; !known || isLate[r] && !late(byte(lead)) {
				index[r], isLate[r] = [2]byte{byte(lead), byte(trail)}, late(byte(lead))
			}
		}
	}
	return index
}

// decodePair returns the character of a pair of bytes, when the pair is one defined character.
func decodePair(dec *encoding.Decoder, lead, trail byte) (rune, bool) {
	text, err := dec.Bytes([]byte{lead, trail})
	if err != nil {
		return 0, false
	}
	r, size := utf8.DecodeRune(text)
	return r, r != utf8.RuneError && size == len(text)
}

// hasC1 reports whether b holds a C1 control character, which x/text reads for the byte 80h of Shift_JIS and iconv
// does not define.
func hasC1(b []byte) bool {
	return bytes.ContainsFunc(b, func(r rune) bool { return r >= 0x80 && r <= 0x9F })
}
