package sami

import (
	"strings"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

type nodeKind uint8

const (
	textNode nodeKind = iota
	elementNode
	commentNode
)

// node is a node of the tree that scan builds. An element has a lower-case name, attributes in file order and
// children. A text node holds decoded text, a comment node the text between <!-- and -->.
type node struct {
	kind     nodeKind
	name     string
	text     string
	attrs    []attr
	children []*node
}

// attr is an attribute of an element. An attribute without = has no value, which reads as "".
type attr struct {
	name     string
	value    string
	hasValue bool
}

// voidElements never get children.
//
//nolint:gochecknoglobals // a lookup table
var voidElements = map[string]bool{
	"br": true, "hr": true, "img": true, "input": true, "meta": true, "link": true, "area": true, "base": true,
	"basefont": true, "col": true, "frame": true, "isindex": true, "param": true, "embed": true,
}

// closedByP holds the elements that a <p> start tag closes while one of them is the innermost open element.
//
//nolint:gochecknoglobals // a lookup table
var closedByP = map[string]bool{
	"p": true, "b": true, "i": true, "u": true, "s": true, "strike": true, "tt": true, "big": true, "small": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
}

// endPriority returns the priority of an element for end tags. An end tag never closes an element of a higher
// priority.
func endPriority(name string) int {
	switch name {
	case "div":
		return 150
	case "td", "th":
		return 160
	case "tr":
		return 170
	case "thead", "tbody", "tfoot":
		return 180
	case "table":
		return 190
	case "head", "body":
		return 200
	case "html":
		return 220
	}
	return 100
}

func isDocumentElement(name string) bool {
	return name == "html" || name == "head" || name == "body"
}

type scanner struct {
	s     string
	i     int
	stack []*node
	// open holds the stack positions of the open elements by name, and high those by end priority above 100, so an
	// end tag finds its element and the elements that block it without a walk of the stack.
	open map[string][]int
	high map[int][]int
	// pending holds the text since the last node, so a long run of references does not copy the text each time.
	pending strings.Builder
}

// scan parses s as the content of <body> and returns the top-level nodes. It gives the tree of the libxml2 HTML
// parser of PHP for the markup that SAMI files hold. Lists, tables, forms and scripts follow more libxml2 rules.
//
// Behaviour reference only: libxml2, HTMLparser.c (https://gitlab.gnome.org/GNOME/libxml2/-/blob/master/HTMLparser.c).
func scan(s string) []*node {
	root := &node{kind: elementNode, name: "body"}
	sc := &scanner{s: s, stack: []*node{root}, open: map[string][]int{}, high: map[int][]int{}}
	for sc.i < len(s) {
		switch s[sc.i] {
		case '<':
			sc.markup()
		case '&':
			sc.text(sc.reference(s[sc.i:]))
		default:
			end := strings.IndexAny(s[sc.i:], "<&")
			if end < 0 {
				end = len(s) - sc.i
			}
			sc.text(s[sc.i : sc.i+end])
			sc.i += end
		}
	}
	sc.flush()
	return root.children
}

func (sc *scanner) current() *node {
	return sc.stack[len(sc.stack)-1]
}

func (sc *scanner) text(t string) {
	sc.pending.WriteString(t)
}

// flush adds the pending text as a text node. Each change of the tree calls it first, so 2 text nodes never meet.
func (sc *scanner) flush() {
	if sc.pending.Len() == 0 {
		return
	}
	sc.append(&node{kind: textNode, text: sc.pending.String()})
	sc.pending.Reset()
}

// markup reads the construct at sc.i, which starts with <.
func (sc *scanner) markup() {
	rest := sc.s[sc.i:]
	next := byte(0)
	if len(rest) > 1 {
		next = rest[1]
	}
	switch {
	case isASCIILetter(next):
		sc.i++
		sc.startTag()
	case next == '/' && len(rest) > 2 && isASCIILetter(rest[2]):
		sc.i += 2
		sc.endTag()
	case strings.HasPrefix(rest, "<!--"):
		sc.comment(rest[len("<!--"):])
	case next == '!' || next == '?':
		sc.skipPast('>')
	default:
		sc.text("<")
		sc.i++
	}
}

// comment reads the comment whose text starts body. A comment without --> runs to the end.
func (sc *scanner) comment(body string) {
	sc.flush()
	end := strings.Index(body, "-->")
	if end < 0 {
		sc.append(&node{kind: commentNode, text: body})
		sc.i = len(sc.s)
		return
	}
	sc.append(&node{kind: commentNode, text: body[:end]})
	sc.i += len("<!--") + end + len("-->")
}

func (sc *scanner) append(n *node) {
	cur := sc.current()
	cur.children = append(cur.children, n)
}

func (sc *scanner) skipPast(c byte) {
	if end := strings.IndexByte(sc.s[sc.i:], c); end >= 0 {
		sc.i += end + 1
	} else {
		sc.i = len(sc.s)
	}
}

func (sc *scanner) startTag() {
	name := markupcore.ASCIILower(sc.name())
	el := &node{kind: elementNode, name: name}
	selfClosed := sc.attributes(el)
	if isDocumentElement(name) {
		return
	}
	sc.flush()
	switch name {
	case "p":
		for len(sc.stack) > 1 && closedByP[sc.current().name] {
			sc.pop()
		}
	case "div":
		for len(sc.stack) > 1 && sc.current().name == "p" {
			sc.pop()
		}
	}
	sc.append(el)
	if !selfClosed && !voidElements[name] {
		sc.push(el)
	}
}

// attributes reads the attributes of a start tag up to its > and reports whether the tag ends with />.
func (sc *scanner) attributes(el *node) bool {
	for {
		sc.skipBlanks()
		if sc.i >= len(sc.s) {
			return false
		}
		switch {
		case sc.s[sc.i] == '>':
			sc.i++
			return false
		case strings.HasPrefix(sc.s[sc.i:], "/>"):
			sc.i += 2
			return true
		}
		if !isNameStart(sc.s[sc.i]) {
			sc.skipJunk()
			continue
		}
		a := attr{name: markupcore.ASCIILower(sc.name())}
		sc.skipBlanks()
		if sc.i < len(sc.s) && sc.s[sc.i] == '=' {
			sc.i++
			sc.skipBlanks()
			a.value, a.hasValue = sc.attributeValue(), true
		}
		if !hasAttr(el, a.name) {
			el.attrs = append(el.attrs, a)
		}
	}
}

func hasAttr(el *node, name string) bool {
	for _, a := range el.attrs {
		if a.name == name {
			return true
		}
	}
	return false
}

// skipJunk skips at least one byte, up to white space, > or />.
func (sc *scanner) skipJunk() {
	sc.i++
	for sc.i < len(sc.s) && !isBlank(sc.s[sc.i]) && sc.s[sc.i] != '>' && !strings.HasPrefix(sc.s[sc.i:], "/>") {
		sc.i++
	}
}

func (sc *scanner) attributeValue() string {
	if sc.i >= len(sc.s) {
		return ""
	}
	var raw string
	if q := sc.s[sc.i]; q == '"' || q == '\'' {
		rest := sc.s[sc.i+1:]
		end := strings.IndexByte(rest, q)
		if end < 0 {
			raw, sc.i = rest, len(sc.s)
		} else {
			raw, sc.i = rest[:end], sc.i+1+end+1
		}
	} else {
		start := sc.i
		for sc.i < len(sc.s) && !isBlank(sc.s[sc.i]) && sc.s[sc.i] != '>' {
			sc.i++
		}
		raw = sc.s[start:sc.i]
	}
	return decodeReferences(raw)
}

func (sc *scanner) endTag() {
	name := markupcore.ASCIILower(sc.name())
	sc.skipPast('>')
	if isDocumentElement(name) {
		return
	}
	positions := sc.open[name]
	if len(positions) == 0 {
		return
	}
	target := positions[len(positions)-1]
	priority := endPriority(name)
	for p, blockers := range sc.high {
		if p > priority && len(blockers) > 0 && blockers[len(blockers)-1] > target {
			return
		}
	}
	sc.flush()
	for len(sc.stack) > target {
		sc.pop()
	}
}

func (sc *scanner) push(el *node) {
	position := len(sc.stack)
	sc.stack = append(sc.stack, el)
	sc.open[el.name] = append(sc.open[el.name], position)
	if p := endPriority(el.name); p > 100 {
		sc.high[p] = append(sc.high[p], position)
	}
}

func (sc *scanner) pop() {
	el := sc.current()
	sc.stack = sc.stack[:len(sc.stack)-1]
	sc.open[el.name] = sc.open[el.name][:len(sc.open[el.name])-1]
	if p := endPriority(el.name); p > 100 {
		sc.high[p] = sc.high[p][:len(sc.high[p])-1]
	}
}

// name reads a tag or attribute name: letters, digits, :, -, _ and the dot.
func (sc *scanner) name() string {
	start := sc.i
	for sc.i < len(sc.s) && isNameByte(sc.s[sc.i]) {
		sc.i++
	}
	return sc.s[start:sc.i]
}

func (sc *scanner) skipBlanks() {
	for sc.i < len(sc.s) && isBlank(sc.s[sc.i]) {
		sc.i++
	}
}

// reference decodes the character reference at the start of s, which starts with &, and moves past it.
// A reference that does not decode stays as text.
func (sc *scanner) reference(s string) string {
	text, n := reference(s)
	sc.i += n
	return text
}

func decodeReferences(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	var b strings.Builder
	for s != "" {
		i := strings.IndexByte(s, '&')
		if i < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:i])
		text, n := reference(s[i:])
		b.WriteString(text)
		s = s[i+n:]
	}
	return b.String()
}

// reference returns the text of the reference at the start of s and its length. A named reference needs a name of
// HTML 4.01 and its ;. A numeric reference decodes also without ;, as in libxml2. A numeric reference to a code point
// that XML does not allow decodes to nothing, as in libxml2. Any other & is text.
func reference(s string) (string, int) {
	if strings.HasPrefix(s, "&#") {
		return numericReference(s)
	}
	n := 1
	for n < len(s) && (isASCIILetter(s[n]) || n > 1 && isASCIIDigit(s[n])) {
		n++
	}
	if n == 1 || n >= len(s) || s[n] != ';' {
		return "&", 1
	}
	r, ok := entities()[s[1:n]]
	if !ok {
		return "&", 1
	}
	return string(r), n + 1
}

func numericReference(s string) (string, int) {
	n, base := 2, 10
	if len(s) > 2 && (s[2] == 'x' || s[2] == 'X') {
		n, base = 3, 16
	}
	start := n
	value, n := digits(s, n, base)
	if n == start {
		return "&", 1
	}
	if n < len(s) && s[n] == ';' {
		n++
	}
	if !isXMLChar(value) {
		return "", n
	}
	return string(rune(value)), n
}

// digits reads the digits of s from start on. The value stops growing above utf8.MaxRune.
func digits(s string, start, base int) (int, int) {
	value, n := 0, start
	for n < len(s) {
		d, ok := digitValue(s[n], base)
		if !ok {
			break
		}
		if value <= utf8.MaxRune {
			value = value*base + d
		}
		n++
	}
	return value, n
}

func digitValue(c byte, base int) (int, bool) {
	switch {
	case isASCIIDigit(c):
		return int(c - '0'), true
	case base == 16 && 'a' <= c && c <= 'f':
		return int(c-'a') + 10, true
	case base == 16 && 'A' <= c && c <= 'F':
		return int(c-'A') + 10, true
	}
	return 0, false
}

func isXMLChar(r int) bool {
	return r == '\t' || r == '\n' || r == '\r' || 0x20 <= r && r <= 0xD7FF || 0xE000 <= r && r <= 0xFFFD ||
		0x10000 <= r && r <= utf8.MaxRune
}

func isASCIILetter(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

func isASCIIDigit(c byte) bool {
	return '0' <= c && c <= '9'
}

func isNameStart(c byte) bool {
	return isASCIILetter(c) || c == '_' || c == ':' || c == '.'
}

func isNameByte(c byte) bool {
	return isNameStart(c) || isASCIIDigit(c) || c == '-'
}

func isBlank(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

var serialEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;") //nolint:gochecknoglobals // a replacer holds no state

// serialize writes n as the saveHTML of libxml2 writes a node.
func serialize(b *strings.Builder, n *node) {
	switch n.kind {
	case textNode:
		b.WriteString(serialEscaper.Replace(n.text))
	case commentNode:
		b.WriteString("<!--" + n.text + "-->")
	case elementNode:
		b.WriteString("<" + n.name)
		for _, a := range n.attrs {
			b.WriteString(" " + a.name)
			if a.hasValue {
				b.WriteString("=" + quoteAttribute(a.value))
			}
		}
		b.WriteByte('>')
		if voidElements[n.name] {
			return
		}
		for _, c := range n.children {
			serialize(b, c)
		}
		b.WriteString("</" + n.name + ">")
	}
}

func quoteAttribute(v string) string {
	v = serialEscaper.Replace(v)
	switch {
	case !strings.Contains(v, `"`):
		return `"` + v + `"`
	case !strings.Contains(v, "'"):
		return "'" + v + "'"
	default:
		return `"` + strings.ReplaceAll(v, `"`, "&quot;") + `"`
	}
}
