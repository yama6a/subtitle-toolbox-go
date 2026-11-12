package mcc

import (
	"bytes"
	"encoding/hex"
	"strconv"
	"strings"
	"unicode/utf8"
)

// emptyTriplet is the empty DTVCC triplet that the letters G to O repeat.
const emptyTriplet = "\xFA\x00\x00"

const maxEmptyRun = 9

// alias returns the bytes of an alias letter. G to O stand for 1 to 9 empty triplets.
//
// Adapted from Subtitle Edit, src/libse/SubtitleFormats/MacCaption10.cs and src/libse/Cea708/Smpte291M.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/58fad713d9c2b322784bc2fa52148e078dbc8759/src/libse/SubtitleFormats/MacCaption10.cs), MIT.
func alias(c byte) ([]byte, bool) {
	switch c {
	case 'P':
		return []byte{0xFB, 0x80, 0x80}, true
	case 'Q':
		return []byte{0xFC, 0x80, 0x80}, true
	case 'R':
		return []byte{0xFD, 0x80, 0x80}, true
	case 'S':
		return []byte{0x96, 0x69}, true
	case 'T':
		return []byte{0x61, 0x01}, true
	case 'U':
		return []byte{0xE1, 0, 0, 0}, true
	case 'Z':
		return []byte{0}, true
	}
	if c > 'F' && c <= 'F'+maxEmptyRun {
		return []byte(strings.Repeat(emptyTriplet, int(c-'F'))), true
	}
	return nil, false
}

// decodeHex decodes hex digit pairs in any case and alias letters. It returns an error message for a bad character.
func decodeHex(text string) ([]byte, string) {
	var out []byte
	half := -1
	for i := range len(text) {
		c := text[i]
		if d, ok := hexDigit(c); ok {
			if half < 0 {
				half = d
			} else {
				out = append(out, byte(half<<4|d))
				half = -1
			}
			continue
		}
		alias, ok := alias(c)
		if !ok || half >= 0 {
			r, _ := utf8.DecodeRuneInString(text[i:])
			return nil, "The MCC data has the invalid character " + strconv.Quote(string(r)) + " at position " + strconv.Itoa(i+1) + "."
		}
		out = append(out, alias...)
	}
	if half >= 0 {
		return nil, "The MCC data ends with half a byte."
	}
	return out, ""
}

func hexDigit(c byte) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10, true
	}
	return 0, false
}

// encodeHex writes bytes as upper-case hex digits and alias letters. U stays unused, as files disagree on its length.
func encodeHex(data []byte) string {
	var b strings.Builder
	for len(data) > 0 {
		run := 0
		for run < maxEmptyRun && 3*run < len(data) && bytes.HasPrefix(data[3*run:], []byte(emptyTriplet)) {
			run++
		}
		if run > 0 {
			b.WriteByte(byte('F' + run))
			data = data[3*run:]
			continue
		}
		if letter, n := aliasAt(data); n > 0 {
			b.WriteByte(letter)
			data = data[n:]
			continue
		}
		b.WriteString(strings.ToUpper(hex.EncodeToString(data[:1])))
		data = data[1:]
	}
	return b.String()
}

func aliasAt(data []byte) (byte, int) {
	for _, letter := range []byte("PQRSTZ") {
		b, _ := alias(letter)
		if bytes.HasPrefix(data, b) {
			return letter, len(b)
		}
	}
	return 0, 0
}
