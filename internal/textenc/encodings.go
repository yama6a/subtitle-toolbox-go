package textenc

import (
	"bytes"
	"encoding/binary"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// Encoding is one encoding of the supported list.
type Encoding struct {
	// Name is the canonical name, for example "Windows-1252".
	Name    string
	aliases []string
	decode  func([]byte) ([]byte, bool)
}

// Decode returns data as UTF-8. False means that data holds a byte or a sequence that the encoding does not define.
func (e Encoding) Decode(data []byte) ([]byte, bool) {
	return e.decode(data)
}

//nolint:gochecknoglobals // lookup table of the supported encodings
var encodings = []Encoding{
	{Name: "UTF-8", aliases: []string{"UTF8"}, decode: func(b []byte) ([]byte, bool) { return b, true }},
	{Name: "UTF-16LE", decode: utf16Decoder(binary.LittleEndian)},
	{Name: "UTF-16BE", decode: utf16Decoder(binary.BigEndian)},
	{Name: "UTF-16", decode: utf16Decoder(binary.BigEndian)},
	{Name: "UTF-32LE", decode: utf32Decoder(binary.LittleEndian)},
	{Name: "UTF-32BE", decode: utf32Decoder(binary.BigEndian)},
	{Name: "UTF-32", decode: utf32Decoder(binary.BigEndian)},
	windows("1250", charmap.Windows1250),
	windows("1251", charmap.Windows1251),
	windows("1252", charmap.Windows1252),
	windows("1253", charmap.Windows1253),
	windows("1254", charmap.Windows1254),
	windows("1255", charmap.Windows1255),
	windows("1256", charmap.Windows1256),
	windows("1257", charmap.Windows1257),
	windows("1258", charmap.Windows1258),
	windows("874", charmap.Windows874),
	iso("1", charmap.ISO8859_1, "LATIN1"),
	iso("2", charmap.ISO8859_2),
	iso("3", charmap.ISO8859_3),
	iso("4", charmap.ISO8859_4),
	iso("5", charmap.ISO8859_5),
	iso("6", charmap.ISO8859_6),
	iso("7", charmap.ISO8859_7),
	iso("8", charmap.ISO8859_8),
	iso("9", charmap.ISO8859_9),
	iso("10", charmap.ISO8859_10),
	iso("13", charmap.ISO8859_13),
	iso("14", charmap.ISO8859_14),
	iso("15", charmap.ISO8859_15),
	iso("16", charmap.ISO8859_16),
	// ISO-8859-11 is Windows-874 without the bytes 0x80 to 0x9F, which add no letter.
	{Name: "ISO-8859-11", aliases: []string{"ISO8859-11"}, decode: isoDecoder(charmap.Windows874)},
	{Name: "KOI8-R", decode: xDecoder(charmap.KOI8R, false)},
	{Name: "KOI8-U", decode: xDecoder(charmap.KOI8U, false)},
	{
		Name:    "Shift_JIS",
		aliases: []string{"SHIFT-JIS", "SJIS", "MS_KANJI", "CSSHIFTJIS", "CP932", "Windows-31J"},
		decode:  xDecoder(japanese.ShiftJIS, false),
	},
	{Name: "EUC-JP", decode: xDecoder(japanese.EUCJP, false)},
	{Name: "GBK", aliases: []string{"CP936"}, decode: xDecoder(simplifiedchinese.GBK, false)},
	{Name: "GB18030", decode: decodeGB18030},
	{Name: "Big5", aliases: []string{"CP950"}, decode: xDecoder(traditionalchinese.Big5, false)},
	{Name: "EUC-KR", decode: xDecoder(korean.EUCKR, false)},
	{Name: "CP949", aliases: []string{"UHC"}, decode: xDecoder(korean.EUCKR, false)},
	{Name: "CP437", aliases: []string{"IBM437"}, decode: xDecoder(charmap.CodePage437, false)},
	{Name: "CP850", aliases: []string{"IBM850"}, decode: xDecoder(charmap.CodePage850, false)},
	{Name: "CP866", aliases: []string{"IBM866"}, decode: xDecoder(charmap.CodePage866, false)},
}

// Lookup returns the encoding of name, compared without case. A name outside the list returns ErrUnknownEncoding.
func Lookup(name string) (Encoding, error) {
	for _, e := range encodings {
		if strings.EqualFold(e.Name, name) || slices.ContainsFunc(e.aliases, func(a string) bool { return strings.EqualFold(a, name) }) {
			return e, nil
		}
	}
	return Encoding{}, ErrUnknownEncoding
}

// Names returns the canonical names of the supported encodings, in table order.
func Names() []string {
	names := make([]string, len(encodings))
	for i, e := range encodings {
		names[i] = e.Name
	}
	return names
}

// windows builds a Windows code page row. Microsoft leaves the bytes undefined that x/text decodes as C1 controls,
// and glibc iconv rejects them.
func windows(number string, m *charmap.Charmap) Encoding {
	return Encoding{Name: "Windows-" + number, aliases: []string{"CP" + number}, decode: xDecoder(m, true)}
}

func iso(part string, m *charmap.Charmap, aliases ...string) Encoding {
	return Encoding{Name: "ISO-8859-" + part, aliases: append([]string{"ISO8859-" + part}, aliases...), decode: isoDecoder(m)}
}

// isoDecoder reads 0x00 to 0x9F as U+0000 to U+009F, as ISO-8859 defines them. The x/text tables of ISO-8859-2 and
// later leave 0x80 to 0x9F out.
func isoDecoder(m *charmap.Charmap) func([]byte) ([]byte, bool) {
	return func(b []byte) ([]byte, bool) {
		out := make([]byte, 0, len(b)*3)
		for _, c := range b {
			r := rune(c)
			if c >= 0xA0 {
				r = m.DecodeByte(c)
			}
			if r == utf8.RuneError {
				return nil, false
			}
			out = utf8.AppendRune(out, r)
		}
		return out, true
	}
}

// xDecoder turns the U+FFFD that an x/text decoder writes for an undefined byte into a failure.
// No encoding of the table but GB18030 can hold U+FFFD, so any U+FFFD in the output fails.
func xDecoder(e encoding.Encoding, c1Fails bool) func([]byte) ([]byte, bool) {
	return func(b []byte) ([]byte, bool) {
		out, err := e.NewDecoder().Bytes(b)
		if err != nil || bytes.ContainsRune(out, utf8.RuneError) || c1Fails && hasC1(out) {
			return nil, false
		}
		return out, true
	}
}

// decodeGB18030 fails only when the output holds more U+FFFD than the input encodes as 84 31 A4 37.
func decodeGB18030(b []byte) ([]byte, bool) {
	out, err := simplifiedchinese.GB18030.NewDecoder().Bytes(b)
	if err != nil || bytes.Count(out, []byte(string(utf8.RuneError))) > bytes.Count(b, []byte{0x84, 0x31, 0xA4, 0x37}) {
		return nil, false
	}
	return out, true
}

func hasC1(b []byte) bool {
	return bytes.ContainsFunc(b, func(r rune) bool { return r >= 0x80 && r <= 0x9F })
}

// utf16Decoder is strict, because the x/text decoder cannot tell a real U+FFFD from a replacement.
func utf16Decoder(order binary.ByteOrder) func([]byte) ([]byte, bool) {
	return func(b []byte) ([]byte, bool) {
		if len(b)%2 != 0 {
			return nil, false
		}
		out := make([]byte, 0, len(b)*3/2)
		for i := 0; i < len(b); i += 2 {
			r := rune(order.Uint16(b[i:]))
			if utf16.IsSurrogate(r) {
				if r >= 0xDC00 || i+4 > len(b) {
					return nil, false
				}
				r = utf16.DecodeRune(r, rune(order.Uint16(b[i+2:])))
				if r == utf8.RuneError {
					return nil, false
				}
				i += 2
			}
			out = utf8.AppendRune(out, r)
		}
		return out, true
	}
}

func utf32Decoder(order binary.ByteOrder) func([]byte) ([]byte, bool) {
	return func(b []byte) ([]byte, bool) {
		if len(b)%4 != 0 {
			return nil, false
		}
		out := make([]byte, 0, len(b))
		for i := 0; i < len(b); i += 4 {
			v := order.Uint32(b[i:])
			if v > utf8.MaxRune || utf16.IsSurrogate(rune(v)) {
				return nil, false
			}
			out = utf8.AppendRune(out, rune(v))
		}
		return out, true
	}
}
