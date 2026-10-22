package xmlraw

import (
	"encoding/xml"
	"io"
	"slices"
)

// Serialize writes n as libxml xmlNodeDump writes it without formatting. An element writes its own xmlns
// declarations first and no declaration of an ancestor. An element without children writes as <name/>. Text and
// attribute values are UTF-8 with the characters of the tables below escaped. A document writes its children.
func Serialize(w io.Writer, n *Node) error {
	_, err := w.Write(appendNode(nil, n))
	return err
}

func appendNode(dst []byte, n *Node) []byte {
	switch n.Kind {
	case DocumentNode:
		for _, c := range n.Children {
			dst = appendNode(dst, c)
		}
	case ElementNode:
		dst = appendElement(dst, n)
	case TextNode:
		dst = appendEscaped(dst, n.Data, textEscapes)
	case CommentNode:
		dst = append(append(append(dst, "<!--"...), n.Data...), "-->"...)
	case ProcInstNode:
		dst = append(append(dst, "<?"...), n.Name.Local...)
		if n.Data != "" {
			dst = append(append(dst, ' '), n.Data...)
		}
		dst = append(dst, "?>"...)
	case DirectiveNode:
		dst = append(append(append(dst, "<!"...), n.Data...), '>')
	}
	return dst
}

func appendElement(dst []byte, n *Node) []byte {
	name := QName(n.Name)
	dst = append(append(dst, '<'), name...)
	decls := slices.DeleteFunc(slices.Clone(n.Attrs), func(a xml.Attr) bool { return !IsNamespaceDecl(a) })
	others := slices.DeleteFunc(slices.Clone(n.Attrs), IsNamespaceDecl)
	for _, a := range slices.Concat(decls, others) {
		dst = append(append(append(dst, ' '), QName(a.Name)...), `="`...)
		dst = append(appendEscaped(dst, a.Value, attrEscapes), '"')
	}
	if len(n.Children) == 0 {
		return append(dst, "/>"...)
	}
	dst = append(dst, '>')
	for _, c := range n.Children {
		dst = appendNode(dst, c)
	}
	return append(append(append(dst, "</"...), name...), '>')
}

var (
	textEscapes = map[byte]string{'&': "&amp;", '<': "&lt;", '>': "&gt;", '\r': "&#13;"} //nolint:gochecknoglobals // a lookup table
	attrEscapes = map[byte]string{                                                       //nolint:gochecknoglobals // a lookup table
		'&': "&amp;", '<': "&lt;", '>': "&gt;", '"': "&quot;", '\n': "&#10;", '\r': "&#13;", '\t': "&#9;",
	}
)

func appendEscaped(dst []byte, s string, escapes map[byte]string) []byte {
	for i := range len(s) {
		if e, ok := escapes[s[i]]; ok {
			dst = append(dst, e...)
		} else {
			dst = append(dst, s[i])
		}
	}
	return dst
}
