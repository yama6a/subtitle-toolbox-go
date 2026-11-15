package ttml

import (
	"slices"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/internal/ttmldoc"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/internal/xmlraw"
)

// isMetadataElement reports whether n is ttm:local in a metadata namespace, or has the literal name ttm:local with
// an undeclared prefix.
func isMetadataElement(n *xmlraw.Node, local string) bool {
	if n.Kind != xmlraw.ElementNode {
		return false
	}
	if slices.Contains(ttmldoc.MetadataNamespaces, n.NamespaceURI()) {
		return ttmldoc.LocalName(n) == local
	}
	return n.NamespaceURI() == "" && xmlraw.QName(n.Name) == "ttm:"+local
}

// familyAttr returns the value of the attribute local in one of namespaces, or of the attribute with the literal
// name prefix:local and an undeclared prefix. A file that does not declare the prefix still gets its value.
func familyAttr(n *xmlraw.Node, namespaces []string, prefix, local string) (string, bool) {
	for _, a := range ttmldoc.Attributes(n) {
		if slices.Contains(namespaces, n.AttrNamespaceURI(a)) && a.Name.Local == local || ttmldoc.UndeclaredName(n, a) == prefix+":"+local {
			return a.Value, true
		}
	}
	return "", false
}
