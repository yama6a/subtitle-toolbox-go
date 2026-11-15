package cavena890

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/internal/codepage"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"golang.org/x/text/encoding/charmap"
)

// hebrewEnds holds the characters that Subtitle Edit moves from the start to the end of a Hebrew line, and back.
const hebrewEnds = "-\u2014 !?.\u2026\"\u060C,():;[]+~*/<>&^%$#\\|'"

// table is the text table of a variant. A code above 0xFF is a 2-byte code.
type table struct {
	decode map[uint16]string
	encode map[string]uint16
}

func newTable(entries []entry, skip ...uint16) table {
	t := table{decode: make(map[uint16]string, len(entries)), encode: make(map[string]uint16, len(entries))}
	for _, e := range entries {
		if slices.Contains(skip, e.code) {
			continue
		}
		t.decode[e.code] = e.text
		if _, ok := t.encode[e.text]; !ok {
			t.encode[e.text] = e.code
		}
	}
	return t
}

// tables holds the text table of each variant. Chinese has none, and Scandinavian drops the codes E5h and E6h,
// which stay a and ae with a ring and a diaeresis there.
//
//nolint:gochecknoglobals // lookup tables of the Cavena 890 character codes
var tables = map[variant]table{
	latin:        newTable(latinEntries),
	scandinavian: newTable(latinEntries, 0xE5, 0xE6),
	hebrew:       newTable(hebrewEntries),
	russian:      newTable(russianEntries),
	greek:        newTable(greekEntries),
	arabic:       newTable(arabicEntries),
}

// decode returns the core markup of a text line of 51 bytes. The filler 7Fh, zero bytes and the color codes give no text.
//
// Adapted from Subtitle Edit, src/libse/SubtitleFormats/Cavena890.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/a0dfbfe65f0517ef85b1e69f1606bca93f68ac3c/src/libse/SubtitleFormats/Cavena890.cs), MIT.
func (v variant) decode(b []byte) string {
	codes := make([]uint16, 0, len(b))
	if v == chinese {
		codes = chineseCodes(b)
	} else {
		for _, c := range b {
			codes = append(codes, uint16(c))
		}
	}
	var runs []markupcore.Run
	var text strings.Builder
	italic := false
	for i := 0; i < len(codes); {
		code := codes[i]
		if v.hasItalics() && (code == italicOn || code == italicOff) {
			runs = append(runs, markupcore.Run{Text: text.String(), Italic: italic})
			text.Reset()
			italic = code == italicOn
			i++
			continue
		}
		i += v.appendText(&text, codes[i:])
	}
	runs = append(runs, markupcore.Run{Text: text.String(), Italic: italic})
	if v == hebrew {
		runs = []markupcore.Run{{Text: swapEnds(reverseAnsi(text.String()))}}
	}
	return strings.TrimRight(markupcore.StyleRuns(runs, true), markupcore.TrimSet)
}

// appendText writes the text of the first code of codes, or of its 2-byte code, to text. It returns the count of codes used.
func (v variant) appendText(text *strings.Builder, codes []uint16) int {
	t := tables[v]
	code := codes[0]
	if code < 0x100 && len(codes) > 1 {
		if s, ok := t.decode[code<<8|codes[1]]; ok {
			text.WriteString(s)
			return 2
		}
	}
	s, ok := t.decode[code]
	switch {
	case ok:
		text.WriteString(s)
	case code == dash:
		text.WriteString("-")
	case givesNothing(code):
	case v == chinese:
		return writeUTF16(text, codes)
	default:
		text.WriteString(windows1252(byte(code)))
	}
	return 1
}

// givesNothing reports whether a code without a table entry gives no text: a control code, the filler or a color.
func givesNothing(code uint16) bool {
	return code < 0x20 || code == filler || code >= firstColor && code <= lastColor
}

// writeUTF16 writes the character of the first code unit of codes, or of a surrogate pair. A lone surrogate gives
// nothing. It returns the count of code units used.
func writeUTF16(text *strings.Builder, codes []uint16) int {
	r := rune(codes[0])
	if !utf16.IsSurrogate(r) {
		text.WriteRune(r)
		return 1
	}
	if len(codes) > 1 {
		if pair := utf16.DecodeRune(r, rune(codes[1])); pair != utf8.RuneError {
			text.WriteRune(pair)
			return 2
		}
	}
	return 1
}

// windows1252 returns the character of byte b. The bytes that Windows-1252 leaves undefined give nothing.
func windows1252(b byte) string {
	switch b {
	case 0x81, 0x8D, 0x8F, 0x90, 0x9D:
		return ""
	}
	return string(charmap.Windows1252.DecodeByte(b))
}

// chineseCodes returns the UTF-16BE code units of b. Subtitle Edit drops the zero bytes and fillers at the end
// first. An odd length takes the next byte back, because it can be the low byte 7Fh of a character.
func chineseCodes(b []byte) []uint16 {
	trimmed := bytes.TrimRight(b, "\x00\x7F")
	n := len(trimmed) + len(trimmed)%2
	if n > len(b) {
		b = append(slices.Clone(b), 0)
	}
	b = b[:n]
	codes := make([]uint16, len(b)/2)
	for i := range codes {
		codes[i] = uint16(b[2*i])<<8 | uint16(b[2*i+1])
	}
	return codes
}

// encode returns the 51 bytes of a line of core markup. A character outside the table becomes ?. A line of more than
// 51 bytes fails with an error for the cue at cueIndex.
//
// Adapted from Subtitle Edit, src/libse/SubtitleFormats/Cavena890.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/a0dfbfe65f0517ef85b1e69f1606bca93f68ac3c/src/libse/SubtitleFormats/Cavena890.cs), MIT.
func (v variant) encode(line string, stripTags bool, cueIndex int) ([]byte, error) {
	var out []byte
	for i, token := range markupcore.SplitTags(line) {
		switch {
		case i%2 == 0:
			out = append(out, v.encodeText(markupcore.DecodeEntities(token))...)
		case v.hasItalics() && !stripTags:
			out = append(out, v.italicBytes(token)...)
		}
	}
	if len(out) > textSize {
		return nil, &model.UnwritableError{
			Format: model.Cavena890, CueIndex: cueIndex,
			Reason: fmt.Sprintf("Cavena 890 holds %d bytes per line, got %d for \"%s\".", textSize, len(out), line),
		}
	}
	pad := byte(filler)
	if v == chinese {
		pad = 0
	}
	return append(out, bytes.Repeat([]byte{pad}, textSize-len(out))...), nil
}

// italicBytes returns the bytes of the tag <i> or </i>, and nothing for another tag.
func (v variant) italicBytes(tag string) []byte {
	var code byte
	switch markupcore.ASCIILower(tag) {
	case "<i>":
		code = italicOn
	case "</i>":
		code = italicOff
	default:
		return nil
	}
	if v == chinese {
		return []byte{0, code}
	}
	return []byte{code}
}

func (v variant) encodeText(text string) []byte {
	if v == chinese {
		return utf16Bytes(text)
	}
	if v == hebrew {
		text = reverseAnsi(swapEnds(text))
	}
	chars := markupcore.Characters(text)
	var out []byte
	for i := 0; i < len(chars); i++ {
		if i+1 < len(chars) {
			if code, ok := tables[v].encode[chars[i]+chars[i+1]]; ok {
				out = appendCode(out, code)
				i++
				continue
			}
		}
		out = append(out, v.encodeChar(chars[i])...)
	}
	return out
}

// encodeChar returns the bytes of one character by the rules 2 to 5 of the writer.
func (v variant) encodeChar(char string) []byte {
	t := tables[v]
	if _, used := t.decode[uint16(char[0])]; len(char) == 1 && char[0] >= 0x20 && char[0] <= 0x7E && !used {
		return []byte{char[0]}
	}
	if code, ok := t.encode[char]; ok {
		return appendCode(nil, code)
	}
	return []byte{v.windows1252Byte(char)}
}

func appendCode(out []byte, code uint16) []byte {
	if code < 0x100 {
		return append(out, byte(code))
	}
	return append(out, byte(code>>8), byte(code))
}

// windows1252Byte returns the Windows-1252 byte of char when v reads that byte back as the same character, else ?.
func (v variant) windows1252Byte(char string) byte {
	r, _ := utf8.DecodeRuneInString(char)
	if b, ok := charmap.Windows1252.EncodeRune(r); ok && v.decode([]byte{b}) == markupcore.EscapeText(char) {
		return b
	}
	return codepage.Replacement
}

// utf16Bytes returns the UTF-16BE bytes of text, with a surrogate pair for a character above U+FFFF.
// An invalid byte gives ?.
func utf16Bytes(text string) []byte {
	var out []byte
	for text != "" {
		r, size := utf8.DecodeRuneInString(text)
		text = text[size:]
		if r == utf8.RuneError && size == 1 {
			r = codepage.Replacement
		}
		for _, unit := range utf16.Encode([]rune{r}) {
			out = append(out, byte(unit>>8), byte(unit))
		}
	}
	return out
}

// reverseAnsi reverses each run of characters up to U+00FF, as Subtitle Edit stores Hebrew lines. Doing it twice
// gives the line back.
func reverseAnsi(line string) string {
	chars := markupcore.Characters(line)
	for start := 0; start < len(chars); {
		if !isAnsi(chars[start]) {
			start++
			continue
		}
		end := start
		for end < len(chars) && isAnsi(chars[end]) {
			end++
		}
		slices.Reverse(chars[start:end])
		start = end
	}
	return strings.Join(chars, "")
}

func isAnsi(char string) bool {
	r, _ := utf8.DecodeRuneInString(char)
	return r <= 0xFF
}

// swapEnds moves the punctuation at the start of a line to its end and back, with mirrored brackets. Doing it twice
// gives the line back.
func swapEnds(line string) string {
	chars := markupcore.Characters(line)
	isEnd := func(char string) bool { return strings.Contains(hebrewEnds, char) }
	first := 0
	for first < len(chars) && isEnd(chars[first]) {
		first++
	}
	last := len(chars)
	for last > first && isEnd(chars[last-1]) {
		last--
	}
	return mirror(chars[last:]) + strings.Join(chars[first:last], "") + mirror(chars[:first])
}

// mirror reverses chars and swaps each bracket for its partner.
func mirror(chars []string) string {
	var b strings.Builder
	for _, char := range slices.Backward(chars) {
		b.WriteString(brackets.Replace(char))
	}
	return b.String()
}

//nolint:gochecknoglobals // a fixed replacer
var brackets = strings.NewReplacer("(", ")", ")", "(", "[", "]", "]", "[")

// variant is the text table of a file. Its value is the variant field of the cavena890 format data.
type variant string

const (
	latin        variant = "latin"
	scandinavian variant = "scandinavian"
	hebrew       variant = "hebrew"
	russian      variant = "russian"
	greek        variant = "greek"
	arabic       variant = "arabic"
	chinese      variant = "chinese"
)

// languages maps the language code of line 1 to a variant. The Arabic code counts for line 2.
//
//nolint:gochecknoglobals // lookup table of the language codes of Subtitle Edit
var languages = map[byte]variant{
	0x07: scandinavian, 0x1E: scandinavian, 0x28: scandinavian,
	0x8F: hebrew, 0x70: greek, 0x56: russian, 0x90: chinese, 0x91: chinese,
}

// fontPrefixes holds the font name prefix that also names a variant.
//
//nolint:gochecknoglobals // lookup table of the font names of Subtitle Edit
var fontPrefixes = map[variant]string{hebrew: "HEBNOA", greek: "GREEK", russian: "KYRIL", chinese: "CCKM44"}

// variantOf returns the variant of the language codes and the font names of header, in the order of Subtitle Edit.
func variantOf(header []byte) variant {
	language := languages[header[language1]]
	for _, v := range [...]variant{hebrew, greek, arabic, russian, chinese, scandinavian} {
		if names(header, language, v) {
			return v
		}
	}
	return latin
}

// names reports whether the language code of line 1 or a font name of header names v.
func names(header []byte, language, v variant) bool {
	fonts := [2][]byte{header[font1 : font1+fontSize], header[font2 : font2+fontSize]}
	if v == arabic {
		return header[language2] == arabicCode || string(fonts[0]) == "ARABIC"
	}
	prefix := []byte(fontPrefixes[v])
	return language == v || len(prefix) > 0 && (bytes.HasPrefix(fonts[0], prefix) || bytes.HasPrefix(fonts[1], prefix))
}

func (v variant) known() bool {
	switch v {
	case latin, scandinavian, hebrew, russian, greek, arabic, chinese:
		return true
	}
	return false
}

func (v variant) hasItalics() bool {
	return v == latin || v == scandinavian || v == chinese
}
