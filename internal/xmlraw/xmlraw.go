// Package xmlraw holds an XML tree that keeps namespace prefixes as written, with the parse, recover and
// serialise rules of libxml2. The TTML reader stores parts of the tree as XML text, so the text must come out as
// the PHP library writes it.
package xmlraw

import (
	"encoding/xml"
	"iter"
	"strconv"
	"strings"
)

// XMLNamespace is the namespace that the prefix xml is always bound to.
const XMLNamespace = "http://www.w3.org/XML/1998/namespace"

// Prolog matches the white space, declarations, comments and DOCTYPE before the root element.
const Prolog = `(?s)\A(?:[\t\n\v\f\r ]|<\?.*?\?>|<!--.*?-->|<!DOCTYPE[^>]*>)*`

// PrologAnyCase is Prolog with <!DOCTYPE in any case, as the PHP signatures of SAMI and HTML transcripts read it.
const PrologAnyCase = `(?s)\A(?:[\t\n\v\f\r ]|<\?.*?\?>|<!--.*?-->|<![Dd][Oo][Cc][Tt][Yy][Pp][Ee][^>]*>)*`

// Kind is the kind of a Node.
type Kind uint8

// The kinds of nodes.
const (
	DocumentNode Kind = iota
	ElementNode
	TextNode
	CommentNode
	ProcInstNode
	DirectiveNode
	// CDATANode is a CDATA section. Its Data is the text between <![CDATA[ and ]]>.
	CDATANode
)

// Node is one node of a parsed document. Names keep their prefixes as written.
type Node struct {
	Kind Kind
	// Name is the name of an element, or the target of a processing instruction in Local. Space holds the prefix
	// as written, "" for none.
	Name xml.Name
	// Attrs holds the attributes in file order, xmlns declarations included, prefixes as written.
	Attrs []xml.Attr
	// Data is the text of a text, comment, processing instruction or directive node.
	Data     string
	Children []*Node
	Parent   *Node
	// Line is the line that libxml reports for an element: the line of the > that ends its start tag. libxml without
	// XML_PARSE_BIG_LINES stops at 65535. Line keeps counting.
	Line int
}

// SyntaxError is the first well-formedness error of a parse.
type SyntaxError struct {
	Line int
	Msg  string
}

// Error returns the line and the message.
func (e *SyntaxError) Error() string {
	return "xml: line " + strconv.Itoa(e.Line) + ": " + e.Msg
}

// QName returns the qualified name as written, such as "tts:color".
func QName(name xml.Name) string {
	if name.Space == "" {
		return name.Local
	}
	return name.Space + ":" + name.Local
}

// IsNamespaceDecl reports whether a is an xmlns declaration. libxml reads xmlns:p with a prefix p that is no
// NCName, such as xmlns:0, as a plain attribute without namespace.
func IsNamespaceDecl(a xml.Attr) bool {
	if a.Name.Space == "" {
		return a.Name.Local == "xmlns"
	}
	return a.Name.Space == "xmlns" && isNCNameStart(a.Name.Local[0])
}

func isNCNameStart(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
}

// Root returns the root element of a document node.
func (n *Node) Root() (*Node, bool) {
	for _, c := range n.Children {
		if c.Kind == ElementNode {
			return c, true
		}
	}
	return nil, false
}

// LookupPrefix returns the namespace that the xmlns declarations of n and its ancestors bind to prefix. The prefix
// "" is the default namespace, and "xml" is always bound.
func (n *Node) LookupPrefix(prefix string) (uri string, ok bool) {
	if prefix == "xml" {
		return XMLNamespace, true
	}
	for e := n; e != nil; e = e.Parent {
		for _, a := range e.Attrs {
			if IsNamespaceDecl(a) && (prefix == "" && a.Name.Space == "" || prefix != "" && a.Name.Space == "xmlns" && a.Name.Local == prefix) {
				return a.Value, true
			}
		}
	}
	return "", false
}

// NamespaceURI returns the namespace of an element, "" for none and for an undeclared prefix.
func (n *Node) NamespaceURI() string {
	uri, _ := n.LookupPrefix(n.Name.Space)
	return uri
}

// AttrNamespaceURI returns the namespace of the attribute a of n, "" for an unprefixed attribute and for an
// undeclared prefix.
func (n *Node) AttrNamespaceURI(a xml.Attr) string {
	if a.Name.Space == "" {
		return ""
	}
	uri, _ := n.LookupPrefix(a.Name.Space)
	return uri
}

// Text returns the text of all descendants.
func (n *Node) Text() string {
	var b strings.Builder
	n.appendText(&b)
	return b.String()
}

func (n *Node) appendText(b *strings.Builder) {
	for _, c := range n.Children {
		switch c.Kind {
		case TextNode, CDATANode:
			b.WriteString(c.Data)
		case ElementNode:
			c.appendText(b)
		case DocumentNode, CommentNode, ProcInstNode, DirectiveNode:
		}
	}
}

// Descendants yields the elements below n in document order.
func (n *Node) Descendants() iter.Seq[*Node] {
	return func(yield func(*Node) bool) {
		n.descend(yield)
	}
}

func (n *Node) descend(yield func(*Node) bool) bool {
	for _, c := range n.Children {
		if c.Kind != ElementNode {
			continue
		}
		if !yield(c) || !c.descend(yield) {
			return false
		}
	}
	return true
}

// RemoveChild removes the child c of n.
func (n *Node) RemoveChild(c *Node) {
	for i, child := range n.Children {
		if child == c {
			n.Children = append(n.Children[:i:i], n.Children[i+1:]...)
			c.Parent = nil
			return
		}
	}
}
