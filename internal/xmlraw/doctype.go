package xmlraw

import (
	"bytes"
	"strings"
)

// directive is a declaration with the directive text that encoding/xml returns for the changed bytes, and the
// text of the original bytes.
type directive struct {
	changed, original string
}

// spaceProcessingInstructions returns the offset of the > that closes a declaration such as <!DOCTYPE ...>, or the
// length of s without one. The internal subset in [ and ] and the quoted strings can hold >. A comment of the subset can hold quotes and brackets. It turns the
// quotes, < and > in each processing instruction of the subset into spaces, because encoding/xml reads them as
// strings and nesting, and then misses the end of the declaration
// (https://github.com/yama6a/subtitle-toolbox-go/issues/181). The directive gives the text back.
func spaceProcessingInstructions(s []byte) (int, directive, bool) {
	d := &declaration{s: s}
	for j := 0; j < len(s); j++ {
		if d.quote != 0 {
			if s[j] == d.quote {
				d.quote = 0
			}
			continue
		}
		var end bool
		if j, end = d.step(j); end {
			dir, changed := d.directive(j)
			return j, dir, changed
		}
	}
	return len(s), directive{}, false
}

type declaration struct {
	s      []byte
	depth  int
	quote  byte
	spaced []spacedByte
}

// step reads the markup at j outside quotes. It returns the last offset that it read, and true at the end of the
// declaration.
func (d *declaration) step(j int) (int, bool) {
	switch c := d.s[j]; {
	case bytes.HasPrefix(d.s[j:], []byte("<!--")):
		return j + skipPast(d.s[j:], "<!--", "-->"), false
	case d.depth > 0 && bytes.HasPrefix(d.s[j:], []byte("<?")):
		n := skipPast(d.s[j:], "<?", "?>")
		d.spaced = spaceInstruction(d.s[:j+n+1], j, d.spaced)
		return j + n, false
	case c == '"', c == '\'':
		d.quote = c
	case c == '[':
		d.depth++
	case c == ']':
		d.depth--
	case c == '>' && d.depth <= 0:
		return j, true
	}
	return j, false
}

// directive returns the directive of the declaration that ends at end, and false when no byte changed.
func (d *declaration) directive(end int) (directive, bool) {
	if d.spaced == nil {
		return directive{}, false
	}
	decl := d.s[:end+1]
	original := bytes.Clone(decl)
	for _, sb := range d.spaced {
		original[sb.at] = sb.b
	}
	return directive{changed: directiveText(decl, decl), original: directiveText(decl, original)}, true
}

// spacedByte is a byte that spaceInstruction turned into a space, at its offset in the declaration.
type spacedByte struct {
	at int
	b  byte
}

// spaceInstruction spaces the quotes, < and > between <? at start and the ?> that ends s.
func spaceInstruction(s []byte, start int, spaced []spacedByte) []spacedByte {
	end := len(s)
	if bytes.HasSuffix(s, []byte("?>")) {
		end -= 2
	}
	for k := start + 2; k < end; k++ {
		if !strings.ContainsRune(`'"<>`, rune(s[k])) {
			continue
		}
		spaced = append(spaced, spacedByte{k, s[k]})
		s[k] = ' '
	}
	return spaced
}

// directiveText returns the text of the declaration decl as encoding/xml returns it, without <! and >, and with a
// space for each comment outside quotes. It takes the bytes from text, which has the same length as decl.
func directiveText(decl, text []byte) string {
	var b []byte
	var quote byte
	for j := 2; j < len(decl)-1; j++ {
		c := decl[j]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case bytes.HasPrefix(decl[j:], []byte("<!--")):
			j += skipPast(decl[j:], "<!--", "-->")
			b = append(b, ' ')
			continue
		case c == '"', c == '\'':
			quote = c
		}
		b = append(b, text[j])
	}
	return string(b)
}
