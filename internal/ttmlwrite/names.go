// Package ttmlwrite writes TTML documents for format/ttml and format/itt. It also holds the TTML names, namespaces and style
// rules, which the reader of format/ttml uses too.
package ttmlwrite

import (
	"encoding/xml"
	"slices"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/xmlraw"
)

// LocalName returns the local name of an element as libxml gives it. An undeclared prefix stays part of the name,
// so <foo:p> without xmlns:foo is no <p>.
func LocalName(n *xmlraw.Node) string {
	if n.Name.Space != "" {
		if _, ok := n.LookupPrefix(n.Name.Space); !ok {
			return xmlraw.QName(n.Name)
		}
	}
	return n.Name.Local
}

// IsElement reports whether n is an element of local name in namespace ns.
func IsElement(n *xmlraw.Node, ns, local string) bool {
	return n.Kind == xmlraw.ElementNode && LocalName(n) == local && n.NamespaceURI() == ns
}

// Attributes returns the attributes of n without the xmlns declarations, which libxml keeps apart.
func Attributes(n *xmlraw.Node) []xml.Attr {
	return slices.DeleteFunc(slices.Clone(n.Attrs), xmlraw.IsNamespaceDecl)
}

// UndeclaredName returns the literal name of an attribute without a namespace, such as "tts:color" for an
// undeclared prefix, or "" for an attribute with a namespace.
func UndeclaredName(n *xmlraw.Node, a xml.Attr) string {
	if n.AttrNamespaceURI(a) != "" {
		return ""
	}
	return xmlraw.QName(a.Name)
}

// PlainAttr returns the value of the attribute with the qualified name, as DOM getAttribute reads it.
func PlainAttr(n *xmlraw.Node, name string) (string, bool) {
	for _, a := range Attributes(n) {
		if xmlraw.QName(a.Name) == name {
			return a.Value, true
		}
	}
	return "", false
}

// XMLAttr returns the value of the attribute local in the XML namespace, such as xml:id, or "".
func XMLAttr(n *xmlraw.Node, local string) string {
	for _, a := range Attributes(n) {
		if a.Name.Local == local && n.AttrNamespaceURI(a) == xmlraw.XMLNamespace {
			return a.Value
		}
	}
	return ""
}

// The root namespaces of TTML and of the 2 DFXP drafts.
const (
	NSTTML       = "http://www.w3.org/ns/ttml"
	NSDFXP       = "http://www.w3.org/2006/10/ttaf1"
	NSDFXP200604 = "http://www.w3.org/2006/04/ttaf1"
	NSIMSCStyle  = "http://www.w3.org/ns/ttml/profile/imsc1#styling"
)

//nolint:gochecknoglobals // lookup tables of namespaces
var (
	stylingNamespaces = []string{
		"http://www.w3.org/ns/ttml#styling",
		"http://www.w3.org/2006/10/ttaf1#style",
		"http://www.w3.org/2006/10/ttaf1#styling",
		"http://www.w3.org/2006/04/ttaf1#styling",
		"http://www.w3.org/2006/04/ttaf1#style",
	}
	ParameterNamespaces = []string{
		"http://www.w3.org/ns/ttml#parameter",
		"http://www.w3.org/2006/10/ttaf1#parameter",
		"http://www.w3.org/2006/04/ttaf1#parameter",
	}
	MetadataNamespaces = []string{
		"http://www.w3.org/ns/ttml#metadata",
		"http://www.w3.org/2006/10/ttaf1#metadata",
		"http://www.w3.org/2006/04/ttaf1#metadata",
	}
)

// Serialize returns the XML of n.
func Serialize(n *xmlraw.Node) string {
	var b strings.Builder
	_ = xmlraw.Serialize(&b, n) // a strings.Builder never fails
	return b.String()
}

// DeclaredPrefix returns the prefix that the xmlns declaration a declares, "" for the default namespace. The
// prefix xml and an attribute that is no declaration give false.
func DeclaredPrefix(a xml.Attr) (string, bool) {
	switch {
	case !xmlraw.IsNamespaceDecl(a):
		return "", false
	case a.Name.Space == "":
		return "", true
	default:
		return a.Name.Local, a.Name.Local != "xml"
	}
}
