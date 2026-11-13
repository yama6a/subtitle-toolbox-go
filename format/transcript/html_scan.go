package transcript

import (
	"bytes"
	"strings"
)

// htmlNames are the elements that the HTML transcript reads.
var htmlNames = [...]string{"cite", "time", "p"} //nolint:gochecknoglobals // lookup table

const (
	nameCite = iota
	nameTime
	nameP
)

// htmlElement is a cite, time or p element with its inner HTML.
type htmlElement struct {
	name   int
	offset int
	inner  string
}

func isHTMLSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r'
}

func indexByteFrom[T string | []byte](s T, c byte, from int) int {
	var i int
	switch v := any(s[from:]).(type) {
	case string:
		i = strings.IndexByte(v, c)
	case []byte:
		i = bytes.IndexByte(v, c)
	}
	if i < 0 {
		return -1
	}
	return from + i
}

// hasFoldedAt reports whether s holds name at i, compared in ASCII lower case. name is lower case.
func hasFoldedAt[T string | []byte](s T, i int, name string) bool {
	if i+len(name) > len(s) {
		return false
	}
	for k := range len(name) {
		if c := s[i+k]; c|0x20 != name[k] {
			return false
		}
	}
	return true
}

// startTagAt reports whether s[i] is "<", then name, then white space or ">".
func startTagAt[T string | []byte](s T, i int, name string) bool {
	j := i + 1 + len(name)
	return hasFoldedAt(s, i+1, name) && j < len(s) && (s[j] == '>' || isHTMLSpace(s[j]))
}

// scanner finds elements without an HTML parser. It remembers two facts, so that a start tag without a ">" or
// without an end tag costs one search for the whole input and not one for each such tag: the next ">" at or after
// the scan position, and for each name the position from which no end tag exists.
type scanner struct {
	s     string
	gt    int // index of the first ">" at or after the last search start, len(s) for none, -1 before the first search
	noEnd [len(htmlNames)]int
}

func scanElements(s string) []htmlElement {
	sc := scanner{s: s, gt: -1, noEnd: [len(htmlNames)]int{-1, -1, -1}}
	var out []htmlElement
	for i := indexByteFrom(s, '<', 0); i >= 0; {
		el, end, ok := sc.elementAt(i)
		if ok {
			out = append(out, el)
			i = indexByteFrom(s, '<', end)
			continue
		}
		i = indexByteFrom(s, '<', i+1)
	}
	return out
}

func (sc *scanner) nextGT(from int) int {
	if sc.gt < from {
		sc.gt = len(sc.s)
		if i := indexByteFrom(sc.s, '>', from); i >= 0 {
			sc.gt = i
		}
	}
	return sc.gt
}

// elementAt returns the element that starts at i and the index after its end tag.
func (sc *scanner) elementAt(i int) (htmlElement, int, bool) {
	for k, name := range htmlNames {
		if !startTagAt(sc.s, i, name) {
			continue
		}
		j := i + 1 + len(name)
		tagEnd := j
		if sc.s[j] != '>' {
			tagEnd = sc.nextGT(j)
			if tagEnd == len(sc.s) {
				return htmlElement{}, 0, false
			}
		}
		innerStart := tagEnd + 1
		closeAt, end, ok := sc.endTag(k, innerStart)
		if !ok {
			return htmlElement{}, 0, false
		}
		return htmlElement{name: k, offset: i, inner: sc.s[innerStart:closeAt]}, end, true
	}
	return htmlElement{}, 0, false
}

// endTagAt reports whether s holds "</", name, optional white space and ">" at i, and returns the index after it.
func endTagAt(s string, i int, name string) (int, bool) {
	if !hasFoldedAt(s, i+2, name) {
		return 0, false
	}
	j := i + 2 + len(name)
	for j < len(s) && isHTMLSpace(s[j]) {
		j++
	}
	return j + 1, j < len(s) && s[j] == '>'
}

// endTag finds the first "</", name, optional white space and ">" at or after from. It returns the index of the
// "</" and the index after the ">".
func (sc *scanner) endTag(k, from int) (closeAt, end int, ok bool) {
	if sc.noEnd[k] >= 0 && from >= sc.noEnd[k] {
		return 0, 0, false
	}
	name := htmlNames[k]
	for pos := from; ; {
		i := strings.Index(sc.s[pos:], "</")
		if i < 0 {
			break
		}
		i += pos
		if end, ok := endTagAt(sc.s, i, name); ok {
			return i, end, true
		}
		pos = i + 2
	}
	if sc.noEnd[k] < 0 {
		sc.noEnd[k] = from
	}
	return 0, 0, false
}
