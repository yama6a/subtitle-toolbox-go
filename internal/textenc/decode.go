// Package textenc turns text input into UTF-8 before a parser sees it, and picks the code page of text without a BOM.
package textenc

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

// The starts of the 2 warnings of Decode, so the CLI can add its hint after them without a copy of the text.
const (
	UTF16WarningPrefix    = "The content is UTF-16"
	DetectedWarningPrefix = "The content is not UTF-8. Detection picked "
)

const (
	utf16SampleBytes = 1024
	utf16MinZeros    = 0.4
	utf16MaxZeros    = 0.05
)

// Decoded is the UTF-8 content of an input and the encoding it was read in.
type Decoded struct {
	// Text is the content in UTF-8, or the input bytes when no decoder ran.
	Text []byte
	// Encoding is the name as the caller gave it, or the canonical name when Decode picked the encoding.
	Encoding string
	// Warning is the message of a guess that a lenient read reports, "" when Decode did not guess.
	Warning string
}

//nolint:gochecknoglobals // ordered BOM table. The UTF-32LE BOM starts with the UTF-16LE BOM
var unicodeBOMs = []struct {
	bom  string
	name string
}{
	{"\x00\x00\xFE\xFF", "UTF-32BE"},
	{"\xFF\xFE\x00\x00", "UTF-32LE"},
	{"\xFE\xFF", "UTF-16BE"},
	{"\xFF\xFE", "UTF-16LE"},
}

// Decode returns data as UTF-8 and the encoding it read. sourceEncoding is the name that the caller chose, "" for none.
// A BOM wins over sourceEncoding. Data that the encoding does not define returns a *ConvertError.
func Decode(data []byte, sourceEncoding string) (Decoded, error) {
	var chosen Encoding
	if sourceEncoding != "" {
		var err error
		if chosen, err = Lookup(sourceEncoding); err != nil {
			return Decoded{}, err
		}
	}
	if d, ok, err := fromBOM(data); ok {
		return d, err
	}
	if !isWide(sourceEncoding) {
		if name, ok := sniffUTF16(data); ok {
			return decodeAs(data, name, "The content is "+name+" without a BOM.")
		}
	}
	if sourceEncoding == "" {
		return guess(data), nil
	}
	return decodeChosen(data, chosen, sourceEncoding)
}

// decodeChosen keeps valid UTF-8 without a NUL byte, because zero bytes mark UTF-16 or UTF-32 without a BOM, whose
// ASCII text is also valid UTF-8.
func decodeChosen(data []byte, chosen Encoding, name string) (Decoded, error) {
	if chosen.Name == "UTF-8" || utf8.Valid(data) && bytes.IndexByte(data, 0) < 0 {
		return Decoded{Text: data, Encoding: "UTF-8"}, nil
	}
	text, ok := chosen.Decode(data)
	if !ok {
		return Decoded{}, &ConvertError{Encoding: name}
	}
	return Decoded{Text: text, Encoding: name}, nil
}

// fromBOM keeps content with a UTF-8 BOM, because the parse step removes it, and decodes content with another
// Unicode BOM. False means that data has no BOM.
func fromBOM(data []byte) (Decoded, bool, error) {
	if bytes.HasPrefix(data, []byte(utf8BOM)) {
		return Decoded{Text: data, Encoding: "UTF-8"}, true, nil
	}
	for _, b := range unicodeBOMs {
		if bytes.HasPrefix(data, []byte(b.bom)) {
			d, err := decodeAs(data[len(b.bom):], b.name, "")
			return d, true, err
		}
	}
	return Decoded{}, false, nil
}

// guess keeps data as UTF-8 when it is valid, holds a NUL byte or is mostly UTF-8. Else detection picks the code page.
func guess(data []byte) Decoded {
	if utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 || mostlyUTF8(data) {
		return Decoded{Text: data, Encoding: "UTF-8"}
	}
	name, text, ok := DetectCodePage(data)
	if !ok {
		return Decoded{Text: data, Encoding: "UTF-8"}
	}
	return Decoded{Text: text, Encoding: name, Warning: DetectedWarningPrefix + name + ". Set the source encoding if that is wrong."}
}

func decodeAs(data []byte, name, warning string) (Decoded, error) {
	e, err := Lookup(name)
	if err != nil {
		return Decoded{}, err
	}
	text, ok := e.Decode(data)
	if !ok {
		return Decoded{}, &ConvertError{Encoding: name}
	}
	return Decoded{Text: text, Encoding: name, Warning: warning}, nil
}

// isWide reports whether name starts as a UTF-16, UTF-32, UCS-2 or UCS-4 name, as iconv spells them.
func isWide(name string) bool {
	name = strings.ToUpper(name)
	for _, p := range []string{"UTF16", "UTF-16", "UTF32", "UTF-32", "UCS2", "UCS-2", "UCS4", "UCS-4"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// sniffUTF16 reads ASCII text in UTF-16 by its zero byte in each pair. UTF-32 has zero bytes on both sides.
func sniffUTF16(data []byte) (string, bool) {
	if len(data) < 2 || len(data)%2 != 0 {
		return "", false
	}
	sample := data[:min(len(data), utf16SampleBytes)]
	pairs := float64(len(sample) / 2)
	var zeros [2]int
	for i, b := range sample {
		if b == 0 {
			zeros[i%2]++
		}
	}
	even, odd := float64(zeros[0]), float64(zeros[1])
	switch {
	case odd >= utf16MinZeros*pairs && even <= utf16MaxZeros*pairs:
		return "UTF-16LE", true
	case even >= utf16MinZeros*pairs && odd <= utf16MaxZeros*pairs:
		return "UTF-16BE", true
	default:
		return "", false
	}
}

// mostlyUTF8 reports more valid multi-byte sequences than bad bytes, as in UTF-8 with a few broken bytes.
// Legacy text rarely forms valid UTF-8 sequences.
func mostlyUTF8(b []byte) bool {
	var valid, bad int
	for i := 0; i < len(b); {
		r, n := utf8.DecodeRune(b[i:])
		switch {
		case r == utf8.RuneError && n == 1:
			bad++
		case n > 1:
			valid++
		}
		i += n
	}
	return valid > bad
}
