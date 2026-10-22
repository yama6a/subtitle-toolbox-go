package ttml

import (
	"encoding/xml"
	"slices"

	"github.com/yama6a/subtitle-toolbox-go/internal/xmlraw"
)

// localName returns the local name of an element as libxml gives it. An undeclared prefix stays part of the name,
// so <foo:p> without xmlns:foo is no <p>.
func localName(n *xmlraw.Node) string {
	if n.Name.Space != "" {
		if _, ok := n.LookupPrefix(n.Name.Space); !ok {
			return xmlraw.QName(n.Name)
		}
	}
	return n.Name.Local
}

// isElement reports whether n is an element of local name in namespace ns.
func isElement(n *xmlraw.Node, ns, local string) bool {
	return n.Kind == xmlraw.ElementNode && localName(n) == local && n.NamespaceURI() == ns
}

// isMetadataElement reports whether n is ttm:local in a metadata namespace, or has the literal name ttm:local with
// an undeclared prefix.
func isMetadataElement(n *xmlraw.Node, local string) bool {
	if n.Kind != xmlraw.ElementNode {
		return false
	}
	if slices.Contains(metadataNamespaces, n.NamespaceURI()) {
		return localName(n) == local
	}
	return n.NamespaceURI() == "" && xmlraw.QName(n.Name) == "ttm:"+local
}

// attributes returns the attributes of n without the xmlns declarations, which libxml keeps apart.
func attributes(n *xmlraw.Node) []xml.Attr {
	return slices.DeleteFunc(slices.Clone(n.Attrs), xmlraw.IsNamespaceDecl)
}

// undeclaredName returns the literal name of an attribute without a namespace, such as "tts:color" for an
// undeclared prefix, or "" for an attribute with a namespace.
func undeclaredName(n *xmlraw.Node, a xml.Attr) string {
	if n.AttrNamespaceURI(a) != "" {
		return ""
	}
	return xmlraw.QName(a.Name)
}

// plainAttr returns the value of the attribute with the qualified name, as DOM getAttribute reads it.
func plainAttr(n *xmlraw.Node, name string) (string, bool) {
	for _, a := range attributes(n) {
		if xmlraw.QName(a.Name) == name {
			return a.Value, true
		}
	}
	return "", false
}

// xmlAttr returns the value of the attribute local in the XML namespace, such as xml:id, or "".
func xmlAttr(n *xmlraw.Node, local string) string {
	for _, a := range attributes(n) {
		if a.Name.Local == local && n.AttrNamespaceURI(a) == xmlraw.XMLNamespace {
			return a.Value
		}
	}
	return ""
}

// familyAttr returns the value of the attribute local in one of namespaces, or of the attribute with the literal
// name prefix:local and an undeclared prefix. A file that does not declare the prefix still gets its value.
func familyAttr(n *xmlraw.Node, namespaces []string, prefix, local string) (string, bool) {
	for _, a := range attributes(n) {
		if slices.Contains(namespaces, n.AttrNamespaceURI(a)) && a.Name.Local == local || undeclaredName(n, a) == prefix+":"+local {
			return a.Value, true
		}
	}
	return "", false
}
