package mcc

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// emptyTriplet is the empty DTVCC triplet that the letters G to O repeat.
const emptyTriplet = "\xFA\x00\x00"

const maxEmptyRun = 9

// aliases holds the bytes of each alias letter. G to O stand for 1 to 9 empty triplets.
//
// Adapted from Subtitle Edit, src/libse/SubtitleFormats/MacCaption10.cs and src/libse/Cea708/Smpte291M.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/58fad713d9c2b322784bc2fa52148e078dbc8759/src/libse/SubtitleFormats/MacCaption10.cs), MIT.
//
//nolint:gochecknoglobals // the alias table of MCC, read-only
var aliases = newAliases()

func newAliases() (table [256]string) {
	table['P'] = "\xFB\x80\x80"
	table['Q'] = "\xFC\x80\x80"
	table['R'] = "\xFD\x80\x80"
	table['S'] = "\x96\x69"
	table['T'] = "\x61\x01"
	table['U'] = "\xE1\x00\x00\x00"
	table['Z'] = "\x00"
	for n := 1; n <= maxEmptyRun; n++ {
		table['F'+n] = strings.Repeat(emptyTriplet, n)
	}
	return table
}

// decodeHex decodes hex digit pairs in any case and alias letters. It returns an error message for a bad character.
//
// Adapted from Subtitle Edit, src/libse/SubtitleFormats/MacCaption10.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/58fad713d9c2b322784bc2fa52148e078dbc8759/src/libse/SubtitleFormats/MacCaption10.cs), MIT.
func decodeHex(text string) ([]byte, string) {
	out := make([]byte, 0, len(text)/2+2)
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
		if aliases[c] == "" || half >= 0 {
			r, _ := utf8.DecodeRuneInString(text[i:])
			return nil, "The MCC data has the invalid character " + strconv.Quote(string(r)) + " at position " + strconv.Itoa(i+1) + "."
		}
		out = append(out, aliases[c]...)
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
	return string(appendHex(nil, data))
}

// appendHex appends the text of data to dst. Runs of empty triplets take the longest letter first.
//
// Adapted from Subtitle Edit, src/libse/SubtitleFormats/MacCaption10.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/58fad713d9c2b322784bc2fa52148e078dbc8759/src/libse/SubtitleFormats/MacCaption10.cs), MIT.
func appendHex(dst, data []byte) []byte {
	const digits = "0123456789ABCDEF"
	for i := 0; i < len(data); {
		rest := data[i:]
		if run := emptyRun(rest); run > 0 {
			dst = append(dst, byte('F'+run))
			i += 3 * run
			continue
		}
		if letter, n := aliasAt(rest); n > 0 {
			dst = append(dst, letter)
			i += n
			continue
		}
		dst = append(dst, digits[rest[0]>>4], digits[rest[0]&15])
		i++
	}
	return dst
}

// emptyRun returns the number of empty triplets at the start of data, 9 at most.
func emptyRun(data []byte) int {
	run := 0
	for run < maxEmptyRun && len(data) >= 3*run+3 && data[3*run] == 0xFA && data[3*run+1] == 0 && data[3*run+2] == 0 {
		run++
	}
	return run
}

// fixedRuns lists the letters for fixed byte runs that the writer uses.
//
//nolint:gochecknoglobals // the alias table of MCC, read-only
var fixedRuns = [...]struct {
	letter byte
	bytes  string
}{{'P', "\xFB\x80\x80"}, {'Q', "\xFC\x80\x80"}, {'R', "\xFD\x80\x80"}, {'S', "\x96\x69"}, {'T', "\x61\x01"}, {'Z', "\x00"}}

// aliasAt returns the letter and the length of the fixed byte run at the start of data.
func aliasAt(data []byte) (byte, int) {
	for _, run := range fixedRuns {
		if len(data) >= len(run.bytes) && string(data[:len(run.bytes)]) == run.bytes {
			return run.letter, len(run.bytes)
		}
	}
	return 0, 0
}
