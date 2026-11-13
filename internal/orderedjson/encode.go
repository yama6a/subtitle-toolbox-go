package orderedjson

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Errors of Encode.
var (
	ErrNotFinite   = errors.New("not finite")
	ErrInvalidUTF8 = errors.New("invalid UTF-8")
)

// Encode writes v in the form of PHP json_encode with JSON_UNESCAPED_SLASHES and JSON_UNESCAPED_UNICODE.
// Pretty output has a 4-space indent and ends with LF. Compact output has no white space.
func Encode(v Value, pretty bool) ([]byte, error) {
	e := &encoder{pretty: pretty}
	if err := e.value(v, 0); err != nil {
		return nil, err
	}
	if pretty {
		e.out = append(e.out, '\n')
	}
	return e.out, nil
}

// Block returns the compact JSON of v for the block lines of a parse warning. A number keeps its source text.
func Block(v Value) string {
	e := &encoder{lenient: true}
	_ = e.value(v, 0)
	return string(e.out)
}

type encoder struct {
	out    []byte
	pretty bool
	// lenient makes the encoder write U+FFFD for a bad byte and null for a number that is not finite.
	lenient bool
}

func (e *encoder) value(v Value, depth int) error {
	switch v.kind {
	case Null:
		e.out = append(e.out, "null"...)
	case Bool:
		e.out = strconv.AppendBool(e.out, v.flag)
	case Number:
		return e.number(v)
	case String:
		return e.str(v.text)
	case Array:
		return e.container('[', ']', len(v.items), depth, func(i int) error { return e.value(v.items[i], depth+1) })
	case Object:
		return e.container('{', '}', len(v.members), depth, func(i int) error { return e.member(v.members[i], depth+1) })
	}
	return nil
}

func (e *encoder) number(v Value) error {
	switch {
	case !v.flag:
		e.out = append(e.out, v.text...)
	case e.lenient:
		e.out = append(e.out, "null"...)
	default:
		return ErrNotFinite
	}
	return nil
}

func (e *encoder) member(m Member, depth int) error {
	if err := e.str(m.Key); err != nil {
		return err
	}
	e.out = append(e.out, ':')
	if e.pretty {
		e.out = append(e.out, ' ')
	}
	return e.value(m.Value, depth)
}

func (e *encoder) container(open, closer byte, n, depth int, item func(i int) error) error {
	e.out = append(e.out, open)
	for i := range n {
		if i > 0 {
			e.out = append(e.out, ',')
		}
		e.newline(depth + 1)
		if err := item(i); err != nil {
			return err
		}
	}
	if n > 0 {
		e.newline(depth)
	}
	e.out = append(e.out, closer)
	return nil
}

func (e *encoder) newline(depth int) {
	if !e.pretty {
		return
	}
	e.out = append(e.out, '\n')
	for range depth {
		e.out = append(e.out, "    "...)
	}
}

const hexDigits = "0123456789abcdef"

func (e *encoder) str(s string) error {
	e.out = append(e.out, '"')
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			e.out = appendASCII(e.out, c)
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1 && !e.lenient:
			return ErrInvalidUTF8
		case r == '\u2028' || r == '\u2029':
			e.out = append(e.out, `\u202`...)
			e.out = append(e.out, hexDigits[r-'\u2020'])
		default:
			e.out = utf8.AppendRune(e.out, r)
		}
		i += size
	}
	e.out = append(e.out, '"')
	return nil
}

const controlEscapes = "\b\f\n\r\t"

func appendASCII(out []byte, c byte) []byte {
	switch {
	case c == '"' || c == '\\':
		return append(out, '\\', c)
	case c >= 0x20:
		return append(out, c)
	}
	if i := strings.IndexByte(controlEscapes, c); i >= 0 {
		return append(out, '\\', "bfnrt"[i])
	}
	return append(out, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&15])
}
