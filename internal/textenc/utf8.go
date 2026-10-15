package textenc

import (
	"bytes"
	"unicode/utf8"
)

const utf8BOM = "\ufeff"

// InvalidUTF8Offset returns the offset of the first byte of b that is not part of a valid UTF-8 sequence, -1 for none.
func InvalidUTF8Offset(b []byte) int {
	for i := 0; i < len(b); {
		r, n := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && n == 1 {
			return i
		}
		i += n
	}
	return -1
}

// ReplaceInvalidUTF8 returns b with each byte that is not part of a valid UTF-8 sequence replaced by U+FFFD.
// bytes.ToValidUTF8 writes one U+FFFD for a run of bad bytes, so it does not fit.
func ReplaceInvalidUTF8(b []byte) []byte {
	i := InvalidUTF8Offset(b)
	if i < 0 {
		return b
	}
	out := append(make([]byte, 0, len(b)+8), b[:i]...)
	for i < len(b) {
		r, n := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && n == 1 {
			out = utf8.AppendRune(out, utf8.RuneError)
		} else {
			out = append(out, b[i:i+n]...)
		}
		i += n
	}
	return out
}

// TrimUTF8BOMs returns b without its leading UTF-8 BOMs. Joined files can start with 2 or more.
func TrimUTF8BOMs(b []byte) []byte {
	for bytes.HasPrefix(b, []byte(utf8BOM)) {
		b = b[len(utf8BOM):]
	}
	return b
}

// AddUTF8BOM returns b with one UTF-8 BOM at the start, unless b already starts with one.
func AddUTF8BOM(b []byte) []byte {
	if bytes.HasPrefix(b, []byte(utf8BOM)) {
		return b
	}
	return append([]byte(utf8BOM), b...)
}
