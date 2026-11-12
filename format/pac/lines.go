package pac

import (
	"bytes"
	"strings"
)

// Adapted from Subtitle Edit, src/libse/SubtitleFormats/Pac.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/a0dfbfe65f0517ef85b1e69f1606bca93f68ac3c/src/libse/SubtitleFormats/Pac.cs), MIT.

const (
	lineStart      = 0xFE
	lineEnd        = 0x03
	codeStart      = 0x1F
	doubleByteCode = "\x1FW16."
	unicodeCode    = "\x1F\xEF\xBB\xBF"
	unicodeEnd     = '.'
	lineHeader     = 3 // FEh, the alignment byte and one more byte
	commentCode    = 3 // 1Fh, "C" and a digit
)

const (
	alignRight  = 0
	alignLeft   = 1
	alignCenter = 2
	alignMask   = 0x03
)

type runMode uint8

const (
	singleMode runMode = iota
	doubleMode
	unicodeMode
)

// run is a part of a text line in one mode.
type run struct {
	mode runMode
	data []byte
}

// line is a text line of a record: its alignment byte and its runs.
type line struct {
	alignment byte
	runs      []run
}

type codeKind uint8

const (
	textByte codeKind = iota // an ordinary byte
	lineCode                 // FEh, which starts a line
	modeCode                 // a switch to double-byte or Unicode mode, or back to single-byte mode
	skipCode                 // a 3-byte code without text
	stopCode                 // a comment, which ends the text
)

// classify reads the code at text[i]. It returns the kind, the length of the code and the mode that follows it.
func classify(text []byte, i int, mode runMode) (codeKind, int, runMode) {
	rest := text[i:]
	switch {
	case rest[0] == lineStart:
		return lineCode, lineHeader, singleMode
	case bytes.HasPrefix(rest, []byte(doubleByteCode)):
		return modeCode, len(doubleByteCode), doubleMode
	case bytes.HasPrefix(rest, []byte(unicodeCode)):
		return modeCode, len(unicodeCode), unicodeMode
	case mode == unicodeMode && rest[0] == unicodeEnd:
		return modeCode, 1, singleMode
	case bytes.HasPrefix(rest, []byte{codeStart, 'C'}):
		if len(rest) > 2 && rest[2] >= '0' && rest[2] <= '9' {
			return skipCode, commentCode, mode
		}
		return stopCode, 0, mode
	}
	return textByte, 1, mode
}

// splitLines splits the text of a record into lines. Bytes before the first line start are not text.
// An FEh at the start of a character starts a line, also in double-byte text, where GBK and CP949 use FEh as a lead
// byte of a few rare characters, because Subtitle Edit reads the files the same way. A trail byte FEh does not.
func splitLines(text []byte) []line {
	first := bytes.IndexByte(text, lineStart)
	if first < 0 {
		return nil
	}
	var lines []line
	mode := singleMode
	for i := first; i < len(text); {
		kind, n, next := classify(text, i, mode)
		switch kind {
		case lineCode:
			var alignment byte
			if i+1 < len(text) {
				alignment = text[i+1]
			}
			lines = append(lines, line{alignment: alignment})
		case textByte:
			n = lines[len(lines)-1].addCode(mode, text[i:])
		case stopCode:
			return lines
		case modeCode, skipCode:
		}
		mode = next
		i += n
	}
	return lines
}

// addCode appends the character at the start of rest and returns its length. In double-byte mode a character is
// a pair, so its second byte is never a line start, also when it is FEh. 00h escapes the next byte.
func (l *line) addCode(mode runMode, rest []byte) int {
	n := 1
	if mode == doubleMode && (rest[0] == 0 || rest[0] >= 0x80) && len(rest) > 1 {
		n = 2
	}
	for _, b := range rest[:n] {
		l.add(mode, b)
	}
	return n
}

// add appends b to the last run of l when it has the same mode, else starts a new run.
func (l *line) add(mode runMode, b byte) {
	if n := len(l.runs); n == 0 || l.runs[n-1].mode != mode {
		l.runs = append(l.runs, run{mode: mode})
	}
	last := &l.runs[len(l.runs)-1]
	last.data = append(last.data, b)
}

const (
	arabicLatin = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	hebrewLatin = "0123456789abcdefghijklmnopqrstuvwxyz"
)

// rightToLeft swaps the parentheses and brackets and reverses each run of the characters in latin, as Subtitle
// Edit does for Arabic and Hebrew lines. Doing it twice gives the line back.
func rightToLeft(text, latin string) string {
	runes := []rune(strings.NewReplacer("(", ")", ")", "(", "[", "]", "]", "[").Replace(text))
	for i := 0; i < len(runes); {
		if !strings.ContainsRune(latin, runes[i]) {
			i++
			continue
		}
		j := i
		for j < len(runes) && strings.ContainsRune(latin, runes[j]) {
			j++
		}
		for a, b := i, j-1; a < b; a, b = a+1, b-1 {
			runes[a], runes[b] = runes[b], runes[a]
		}
		i = j
	}
	return string(runes)
}
