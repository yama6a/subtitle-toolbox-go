package ttml

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/internal/ttmldoc"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

// readFileData sets the metadata and the file data. It removes the title from head.
func (r *reader) readFileData(s *model.Subtitle, head *xmlraw.Node) error {
	if lang := ttmldoc.XMLAttr(r.root, "lang"); lang != "" {
		s.SetMetadata(model.MetadataLanguage, lang)
	}
	data := model.NewData(
		model.Field{Key: "namespace", Value: r.ns},
		model.Field{Key: "namespaces", Value: emptyListIfEmpty(r.readNamespaces())},
		model.Field{Key: "attributes", Value: emptyListIfEmpty(readAttributes(r.root, "xml:lang"))},
	)
	if head != nil {
		if title, ok := removeTitle(head); ok {
			s.SetMetadata(model.MetadataTitle, title)
		}
		data.Set("head", ttmldoc.Serialize(head))
	}
	if r.body != nil {
		if attrs := readAttributes(r.body, timingAttributes...); attrs.Len() > 0 {
			data.Set("body", attrs)
		}
	}
	if err := s.SetFormatData(formatDataKey, data); err != nil {
		return fmt.Errorf("%s file data: %w", model.TTML, err)
	}
	return nil
}

// readNamespaces returns the xmlns declarations of the root, sorted by prefix, without xml.
func (r *reader) readNamespaces() *model.Data {
	var prefixes []string
	uris := map[string]string{}
	for _, a := range r.root.Attrs {
		prefix, ok := ttmldoc.DeclaredPrefix(a)
		if _, seen := uris[prefix]; ok && !seen {
			prefixes = append(prefixes, prefix)
			uris[prefix] = a.Value
		}
	}
	sort.Strings(prefixes)
	out := model.NewData()
	for _, p := range prefixes {
		out.Set(p, uris[p])
	}
	return out
}

// readAttributes returns the attributes of n by name, without the names in skip and without ttm:agent in a
// metadata namespace. An attribute in the XML namespace is named xml: plus its local name, whatever its prefix.
func readAttributes(n *xmlraw.Node, skip ...string) *model.Data {
	out := model.NewData()
	for _, a := range ttmldoc.Attributes(n) {
		uri := n.AttrNamespaceURI(a)
		if slices.Contains(ttmldoc.MetadataNamespaces, uri) && a.Name.Local == "agent" {
			continue
		}
		name := xmlraw.QName(a.Name)
		if uri == xmlraw.XMLNamespace {
			name = "xml:" + a.Name.Local
		}
		if !slices.Contains(skip, name) {
			out.Set(name, a.Value)
		}
	}
	return out
}

// removeTitle removes the first ttm:title of head and the white space text right before it. It returns the
// trimmed title, false when it is empty or missing. The writer writes the title metadata in its place.
func removeTitle(head *xmlraw.Node) (string, bool) {
	for n := range head.Descendants() {
		if !isMetadataElement(n, "title") {
			continue
		}
		title := strings.Trim(n.Text(), markupcore.TrimSet)
		parent := n.Parent
		if i := slices.Index(parent.Children, n); i > 0 {
			if prev := parent.Children[i-1]; prev.Kind == xmlraw.TextNode && strings.Trim(prev.Data, markupcore.TrimSet) == "" {
				parent.RemoveChild(prev)
			}
		}
		parent.RemoveChild(n)
		return title, title != ""
	}
	return "", false
}

// emptyListIfEmpty returns an empty list for an empty object, so the format data
// of an empty object writes as an empty list.
func emptyListIfEmpty(d *model.Data) any {
	if d.Len() == 0 {
		return []any{}
	}
	return d
}

// mergeData returns a copy of outer with the fields of inner. An inner value replaces an outer value in its place.
func mergeData(outer, inner *model.Data) *model.Data {
	out := outer.Clone()
	if out == nil {
		out = model.NewData()
	}
	for _, k := range inner.Keys() {
		v, _ := inner.Get(k)
		out.Set(k, v)
	}
	return out
}
