package orderedjson

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// maxDepth is the most arrays and objects inside each other. PHP json_decode with depth 512 fails at the 512th.
const maxDepth = 511

// The error texts of PHP json_decode.
const (
	msgSyntax    = "Syntax error"
	msgDepth     = "Maximum stack depth exceeded"
	msgControl   = "Control character error, possibly incorrectly encoded"
	msgSurrogate = "Single unpaired UTF-16 surrogate in unicode escape"
	msgUTF8      = "Malformed UTF-8 characters, possibly incorrectly encoded"
	msgState     = "State mismatch (invalid or malformed JSON)"
)

// SyntaxError is a decode failure. Its text is one of the PHP json_decode texts.
type SyntaxError struct {
	// Offset is the byte offset of the input where the error starts.
	Offset int
	// Msg is the PHP error text.
	Msg string
}

// Error returns the PHP text.
func (e *SyntaxError) Error() string { return e.Msg }

type decoder struct {
	data []byte
	pos  int
}

// Decode reads one JSON value with white space around it allowed. A byte of a string that does not start
// a valid UTF-8 sequence becomes U+FFFD. A repeated key keeps its first place and its last value.
func Decode(data []byte) (Value, error) {
	d := &decoder{data: data}
	d.space()
	v, err := d.value(0)
	if err != nil {
		return Value{}, err
	}
	d.space()
	if d.pos < len(d.data) {
		return Value{}, d.fail(d.outside())
	}
	return v, nil
}

func (d *decoder) fail(msg string) *SyntaxError { return &SyntaxError{Offset: d.pos, Msg: msg} }

func (d *decoder) space() {
	for d.pos < len(d.data) && strings.IndexByte(" \t\n\r", d.data[d.pos]) >= 0 {
		d.pos++
	}
}

// at reports whether the byte at pos is b.
func (d *decoder) at(b byte) bool { return d.pos < len(d.data) && d.data[d.pos] == b }

// outside returns the error text for the unexpected token at pos. PHP scans a string token before it
// finds the grammar error, so a bad string there gives the string error.
func (d *decoder) outside() string {
	if d.pos >= len(d.data) {
		return msgSyntax
	}
	b := d.data[d.pos]
	switch {
	case b == '"':
		if _, msg := (&decoder{data: d.data, pos: d.pos}).lexString(); msg != "" {
			return msg
		}
	case b < 0x20:
		return msgControl
	case b >= utf8.RuneSelf:
		if r, size := utf8.DecodeRune(d.data[d.pos:]); r == utf8.RuneError && size == 1 {
			return msgUTF8
		}
	}
	return msgSyntax
}

func (d *decoder) value(depth int) (Value, error) {
	if d.pos >= len(d.data) {
		return Value{}, d.fail(msgSyntax)
	}
	switch c := d.data[d.pos]; {
	case c == '{':
		return d.object(depth)
	case c == '[':
		return d.array(depth)
	case c == '"':
		s, err := d.str()
		return StringValue(s), err
	case c == '-' || isDigit(c):
		return d.number()
	case c == 't':
		return d.literal("true", BoolValue(true))
	case c == 'f':
		return d.literal("false", BoolValue(false))
	case c == 'n':
		return d.literal("null", Value{})
	}
	return Value{}, d.fail(d.outside())
}

func (d *decoder) literal(word string, v Value) (Value, error) {
	if strings.HasPrefix(string(d.data[d.pos:min(len(d.data), d.pos+len(word))]), word) {
		d.pos += len(word)
		return v, nil
	}
	return Value{}, d.fail(msgSyntax)
}

func (d *decoder) number() (Value, error) {
	start := d.pos
	if d.at('-') {
		d.pos++
	}
	switch {
	case d.at('0'):
		d.pos++
	case d.digits() == 0:
		return Value{}, d.fail(msgSyntax)
	}
	d.fraction()
	d.exponent()
	return Value{kind: Number, text: string(d.data[start:d.pos])}, nil
}

func (d *decoder) fraction() {
	if d.at('.') && d.pos+1 < len(d.data) && isDigit(d.data[d.pos+1]) {
		d.pos++
		d.digits()
	}
}

// exponent reads e or E, an optional sign and digits. Without digits it reads nothing.
func (d *decoder) exponent() {
	if !d.at('e') && !d.at('E') {
		return
	}
	p := d.pos + 1
	if p < len(d.data) && (d.data[p] == '+' || d.data[p] == '-') {
		p++
	}
	if p < len(d.data) && isDigit(d.data[p]) {
		d.pos = p
		d.digits()
	}
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func (d *decoder) digits() int {
	start := d.pos
	for d.pos < len(d.data) && isDigit(d.data[d.pos]) {
		d.pos++
	}
	return d.pos - start
}

// empty reads the closer of a container that has no items. A closer of the other kind is a state mismatch.
func (d *decoder) empty(closer, other byte) (bool, error) {
	d.space()
	switch {
	case d.at(closer):
		d.pos++
		return true, nil
	case d.at(other):
		return false, d.fail(msgState)
	}
	return false, nil
}

// next reads the comma or the closer after an item. It reports true after the closer.
func (d *decoder) next(closer, other byte) (bool, error) {
	d.space()
	switch {
	case d.at(','):
		d.pos++
		return false, nil
	case d.at(closer):
		d.pos++
		return true, nil
	case d.at(other):
		return false, d.fail(msgState)
	}
	return false, d.fail(d.outside())
}

func (d *decoder) array(depth int) (Value, error) {
	if depth == maxDepth {
		return Value{}, d.fail(msgDepth)
	}
	d.pos++
	v := Value{kind: Array, items: []Value{}}
	if done, err := d.empty(']', '}'); done || err != nil {
		return v, err
	}
	for {
		d.space()
		item, err := d.value(depth + 1)
		if err != nil {
			return Value{}, err
		}
		v.items = append(v.items, item)
		done, err := d.next(']', '}')
		if err != nil {
			return Value{}, err
		}
		if done {
			return v, nil
		}
	}
}

func (d *decoder) object(depth int) (Value, error) {
	if depth == maxDepth {
		return Value{}, d.fail(msgDepth)
	}
	d.pos++
	v := Value{kind: Object, members: []Member{}}
	if done, err := d.empty('}', ']'); done || err != nil {
		return v, err
	}
	var ix keyIndex
	for {
		m, err := d.member(depth)
		if err != nil {
			return Value{}, err
		}
		v.members = ix.set(v.members, m)
		done, err := d.next('}', ']')
		if err != nil {
			return Value{}, err
		}
		if done {
			return v, nil
		}
	}
}

func (d *decoder) member(depth int) (Member, error) {
	d.space()
	if !d.at('"') {
		return Member{}, d.fail(d.outside())
	}
	key, err := d.str()
	if err != nil {
		return Member{}, err
	}
	d.space()
	if !d.at(':') {
		return Member{}, d.fail(d.outside())
	}
	d.pos++
	d.space()
	v, err := d.value(depth + 1)
	return Member{Key: key, Value: v}, err
}

func (d *decoder) str() (string, error) {
	s, msg := d.lexString()
	if msg != "" {
		return "", d.fail(msg)
	}
	return s, nil
}

// lexString reads the string that starts with the quote at pos. It returns an error text and leaves pos
// at the error, or returns the string and leaves pos after the closing quote.
func (d *decoder) lexString() (string, string) {
	d.pos++
	start := d.pos
	var buf []byte
	for {
		d.skipPlain()
		if d.pos >= len(d.data) {
			return "", msgControl
		}
		c := d.data[d.pos]
		if c == '"' {
			s := string(d.data[start:d.pos])
			if buf != nil {
				s = string(append(buf, d.data[start:d.pos]...))
			}
			d.pos++
			return s, ""
		}
		if c < 0x20 {
			return "", msgControl
		}
		buf = append(buf, d.data[start:d.pos]...)
		if c == '\\' {
			var msg string
			if buf, msg = d.escape(buf); msg != "" {
				return "", msg
			}
		} else {
			buf = utf8.AppendRune(buf, utf8.RuneError)
			d.pos++
		}
		start = d.pos
	}
}

// skipPlain moves pos over the bytes of a string that need no change: printable ASCII except the quote
// and the backslash, and valid UTF-8 sequences.
func (d *decoder) skipPlain() {
	for d.pos < len(d.data) {
		c := d.data[d.pos]
		if c >= utf8.RuneSelf {
			r, size := utf8.DecodeRune(d.data[d.pos:])
			if r == utf8.RuneError && size == 1 {
				return
			}
			d.pos += size
			continue
		}
		if c < 0x20 || c == '"' || c == '\\' {
			return
		}
		d.pos++
	}
}

const simpleEscapes, simpleValues = "\"\\/bfnrt", "\"\\/\b\f\n\r\t"

// escape reads the escape at pos and appends its text to buf. On failure it returns an error text.
func (d *decoder) escape(buf []byte) ([]byte, string) {
	if d.pos+1 >= len(d.data) {
		d.pos = len(d.data)
		return buf, msgSyntax
	}
	c := d.data[d.pos+1]
	if c == 'u' {
		return d.unicode(buf)
	}
	i := strings.IndexByte(simpleEscapes, c)
	if i < 0 {
		d.pos++
		return buf, msgSyntax
	}
	d.pos += 2
	return append(buf, simpleValues[i]), ""
}

// unicode reads the u escape at pos. A high surrogate needs a low surrogate escape right after it.
func (d *decoder) unicode(buf []byte) ([]byte, string) {
	r, ok := d.hex4(d.pos + 2)
	if !ok {
		d.pos += 2
		return buf, msgSyntax
	}
	switch {
	case r >= 0xD800 && r < 0xDC00:
		low, ok := d.lowSurrogate(d.pos + 6)
		if !ok {
			return buf, msgSurrogate
		}
		r = 0x10000 + (r-0xD800)<<10 + (low - 0xDC00)
		d.pos += 12
	case r >= 0xDC00 && r <= 0xDFFF:
		return buf, msgSurrogate
	default:
		d.pos += 6
	}
	return utf8.AppendRune(buf, r), ""
}

// lowSurrogate reads a low surrogate escape at p.
func (d *decoder) lowSurrogate(p int) (rune, bool) {
	if p+1 >= len(d.data) || d.data[p] != '\\' || d.data[p+1] != 'u' {
		return 0, false
	}
	low, ok := d.hex4(p + 2)
	return low, ok && low >= 0xDC00 && low <= 0xDFFF
}

func (d *decoder) hex4(p int) (rune, bool) {
	if p+4 > len(d.data) {
		return 0, false
	}
	n, err := strconv.ParseUint(string(d.data[p:p+4]), 16, 32)
	return rune(n), err == nil
}
