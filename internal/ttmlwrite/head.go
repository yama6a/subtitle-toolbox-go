package ttmlwrite

import (
	"encoding/xml"
	"slices"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/position"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const badHead = "The stored TTML head is not a well-formed <head> element."

// load binds the prefixes of the output, parses the stored head inside a <tt> with the output declarations, and
// reads the IDs, agents and forced regions of the head.
func (w *writer) load(stored []Attr, headXML string) error {
	w.bindPrefixes(stored)
	declarations := w.declarations()
	if w.err != nil {
		return w.err
	}
	doc, err := xmlraw.Parse(strings.NewReader("<tt"+declarations+">"+headXML+"</tt>"), xmlraw.ParseOptions{})
	if err != nil {
		return &model.UnwritableError{Format: w.opts.Format, CueIndex: -1, Reason: badHead, Err: err}
	}
	root, _ := doc.Root()
	if len(root.Children) == 0 || root.Children[0].Kind != xmlraw.ElementNode || LocalName(root.Children[0]) != "head" {
		return &model.UnwritableError{Format: w.opts.Format, CueIndex: -1, Reason: badHead}
	}
	w.head = root.Children[0]
	for n := range w.head.Descendants() {
		w.readHeadElement(n)
	}
	return nil
}

// bindPrefixes binds the root namespace, the stored prefixes, and tts and ttm in the namespaces of the dialect when
// no prefix is bound to them.
func (w *writer) bindPrefixes(stored []Attr) {
	w.bind("", w.ns)
	for _, a := range stored {
		if a.Name != "" {
			w.bind(a.Name, a.Value)
		}
	}
	styling, metadata := 0, 0
	switch w.ns {
	case NSDFXP:
		styling, metadata = 1, 1
	case NSDFXP200604:
		styling, metadata = 3, 2
	}
	w.tts = w.bindPrefix("tts", stylingNamespaces, styling)
	w.ttm = w.bindPrefix("ttm", MetadataNamespaces, metadata)
	slices.Sort(w.prefixes)
}

// readHeadElement takes the xml:id of an element of the head, and reads a forced region and the names of an agent.
// The first region of an ID and the first agent of a name win.
func (w *writer) readHeadElement(n *xmlraw.Node) {
	id := XMLAttr(n, "id")
	if id != "" {
		w.usedIDs[id] = true
	}
	if forced, ok := imscForcedDisplay(n); ok && IsElement(n, w.ns, "region") {
		if _, seen := w.forcedRegions[id]; !seen {
			w.forcedRegions[id] = strings.Trim(forced, markupcore.PHPTrimSet) == "true"
		}
	}
	if LocalName(n) != "agent" || !slices.Contains(MetadataNamespaces, n.NamespaceURI()) {
		return
	}
	for name := range n.Descendants() {
		text := strings.Trim(name.Text(), markupcore.PHPTrimSet)
		if _, seen := w.agentIDs[text]; !seen && IsElement(name, n.NamespaceURI(), "name") {
			w.agentIDs[text] = id
		}
	}
}

// imscForcedDisplay returns the itts:forcedDisplay attribute of n in the IMSC styling namespace.
func imscForcedDisplay(n *xmlraw.Node) (string, bool) {
	for _, a := range Attributes(n) {
		if a.Name.Local == "forcedDisplay" && n.AttrNamespaceURI(a) == NSIMSCStyle {
			return a.Value, true
		}
	}
	return "", false
}

func (w *writer) bind(prefix, uri string) {
	if _, ok := w.uris[prefix]; !ok {
		w.prefixes = append(w.prefixes, prefix)
	}
	w.uris[prefix] = uri
}

// bindPrefix returns the first prefix that is bound to one of uris. Else it binds prefix, or prefix with the first
// free number from 2, to uris[preferred].
func (w *writer) bindPrefix(prefix string, uris []string, preferred int) string {
	for _, p := range w.prefixes {
		if p != "" && slices.Contains(uris, w.uris[p]) {
			return p
		}
	}
	candidate := prefix
	for i := 2; w.isBound(candidate); i++ {
		candidate = prefix + strconv.Itoa(i)
	}
	w.bind(candidate, uris[preferred])
	return candidate
}

func (w *writer) isBound(prefix string) bool {
	_, ok := w.uris[prefix]
	return ok
}

// bindIMSCStyling returns the prefix of the IMSC styling namespace, and binds itts when no prefix is bound to it.
func (w *writer) bindIMSCStyling() string {
	prefix := w.bindPrefix("itts", []string{NSIMSCStyle}, 0)
	slices.Sort(w.prefixes)
	return prefix
}

func (w *writer) declarations() string {
	var b strings.Builder
	for _, p := range w.prefixes {
		name := "xmlns"
		if p != "" {
			name += ":" + p
		}
		b.WriteString(w.attribute(name, w.uris[p]))
	}
	return b.String()
}

func (w *writer) addTitle(title string) {
	el := newElement(w.ttm, "title")
	appendChild(el, &xmlraw.Node{Kind: xmlraw.TextNode, Data: w.xmlText(title)})
	var first *xmlraw.Node
	if len(w.head.Children) > 0 {
		first = w.head.Children[0]
	}
	insertChild(w.head, el, first, 2)
	reconcile(el, w.uris[w.ttm])
}

// agentID returns the ID of the agent of name, and adds a new agent before the first styling or layout child of
// the head for a new name.
func (w *writer) agentID(name string) string {
	if id, ok := w.agentIDs[name]; ok {
		return id
	}
	id := w.unusedID("agent" + strconv.Itoa(len(w.agentIDs)+1))
	nameElement := newElement(w.ttm, "name", xml.Attr{Name: xml.Name{Local: "type"}, Value: "full"})
	appendChild(nameElement, &xmlraw.Node{Kind: xmlraw.TextNode, Data: w.xmlText(name)})
	agent := newElement(w.ttm, "agent", xmlIDAttr(id), xml.Attr{Name: xml.Name{Local: "type"}, Value: "person"})
	appendChild(agent, nameElement)

	if !w.agentBeforeKnown {
		for _, c := range w.head.Children {
			if c.Kind == xmlraw.ElementNode && (LocalName(c) == "styling" || LocalName(c) == "layout") {
				w.agentBefore = c
				break
			}
		}
		w.agentBeforeKnown = true
	}
	insertChild(w.head, agent, w.agentBefore, 2)
	reconcile(agent, w.uris[w.ttm])
	w.agentIDs[name] = id
	return id
}

// regionID returns the ID of the region of an alignment, and adds the region at the end of the last layout of the
// head on first use. The region has the shape of the IMSC 1.1 examples with a safe area of 10%.
func (w *writer) regionID(a model.Alignment) string {
	return w.addRegion(regionName(a), "10% 10%", "80% 80%", a)
}

// offsetRegionID returns the ID of the region whose anchor for alignment a is the point o, and adds the region on
// first use. The region reaches from the point to the edges of the screen, so it stays inside the root container.
func (w *writer) offsetRegionID(a model.Alignment, o model.Offset) string {
	o = model.Offset{X: position.Round(o.X), Y: position.Round(o.Y)}
	originX, extentX := span(o.X, a.Column()-1)
	originY, extentY := span(o.Y, 2-a.Row())
	name := regionName(a) + "-" + position.Format(o.X) + "-" + position.Format(o.Y)
	return w.addRegion(name, originX+" "+originY, extentX+" "+extentY, a)
}

// span returns the origin and the extent in percent of a region on one axis. Side 0 anchors at the start of the
// region, 1 at its middle and 2 at its end. The anchor sits at v percent.
func span(v float64, side int) (origin, extent string) {
	var start, length float64
	switch side {
	case 0:
		start, length = v, 100-v
	case 1:
		half := min(v, 100-v)
		start, length = v-half, 2*half
	default:
		start, length = 0, v
	}
	return position.Format(start) + "%", position.Format(length) + "%"
}

func regionName(a model.Alignment) string {
	name := a.String()
	return markupcore.ASCIILower(name[:1]) + name[1:]
}

// addRegion adds a region to the layout unless name has one, and returns its ID.
func (w *writer) addRegion(name, origin, extent string, a model.Alignment) string {
	if id, ok := w.regionIDs[name]; ok {
		return id
	}
	var layout *xmlraw.Node
	for _, c := range w.head.Children {
		if IsElement(c, w.ns, "layout") {
			layout = c
		}
	}
	if layout == nil {
		layout = newElement("", "layout")
		insertChild(w.head, layout, nil, 2)
		if w.agentBefore == nil {
			w.agentBefore = layout
		}
		reconcile(layout, w.ns)
	}

	id := w.unusedID(name)
	style := func(local, value string) xml.Attr {
		return xml.Attr{Name: xml.Name{Space: w.tts, Local: local}, Value: value}
	}
	region := newElement("", "region", xmlIDAttr(id), style("origin", origin), style("extent", extent),
		style("displayAlign", [...]string{"after", "center", "before"}[a.Row()]), style("textAlign", columnName(a)))
	insertChild(layout, region, nil, 3)
	reconcile(region, w.ns)
	reconcileAttributes(region, w.tts, w.uris[w.tts])
	w.regionIDs[name] = id
	return id
}

func columnName(a model.Alignment) string {
	return [...]string{"left", "center", "right"}[a.Column()-1]
}

// unusedID returns id, or id with the first free suffix _2, _3 and so on, and marks the result as taken. The
// suffixes below the last one tried for id stay taken, so the search starts there.
func (w *writer) unusedID(id string) string {
	if !w.usedIDs[id] {
		w.usedIDs[id] = true
		return id
	}
	i := max(2, w.nextSuffix[id])
	for w.usedIDs[id+"_"+strconv.Itoa(i)] {
		i++
	}
	w.nextSuffix[id] = i
	candidate := id + "_" + strconv.Itoa(i)
	w.usedIDs[candidate] = true
	return candidate
}

func newElement(prefix, local string, attrs ...xml.Attr) *xmlraw.Node {
	return &xmlraw.Node{Kind: xmlraw.ElementNode, Name: xml.Name{Space: prefix, Local: local}, Attrs: attrs}
}

func xmlIDAttr(id string) xml.Attr {
	return xml.Attr{Name: xml.Name{Space: "xml", Local: "id"}, Value: id}
}

func appendChild(parent, child *xmlraw.Node) {
	child.Parent = parent
	parent.Children = append(parent.Children, child)
}

// insertChild inserts el at depth with an indent of 2 spaces per level, before the node before, or at the end for
// nil. An anchor with white space text before it moves to that text, so the new element goes after the indent of
// the anchor.
func insertChild(parent, el, before *xmlraw.Node, depth int) {
	indent := func(level int) *xmlraw.Node {
		return &xmlraw.Node{Kind: xmlraw.TextNode, Data: "\n" + strings.Repeat("  ", level)}
	}
	if len(parent.Children) == 0 {
		appendChild(parent, indent(depth-1))
	}
	if before == nil {
		if last := parent.Children[len(parent.Children)-1]; isBlankText(last) {
			before = last
		} else {
			before = indent(depth - 1)
			appendChild(parent, before)
		}
	} else if i := indexFromEnd(parent.Children, before); i > 0 && isBlankText(parent.Children[i-1]) {
		before = parent.Children[i-1]
	}
	i := indexFromEnd(parent.Children, before)
	in := indent(depth)
	in.Parent, el.Parent = parent, parent
	parent.Children = slices.Insert(parent.Children, i, in, el)
}

// indexFromEnd returns the index of n in nodes, searching from the end, because the nodes that the writer
// inserts before sit near the end. It returns -1 when n is absent.
func indexFromEnd(nodes []*xmlraw.Node, n *xmlraw.Node) int {
	for i := len(nodes) - 1; i >= 0; i-- {
		if nodes[i] == n {
			return i
		}
	}
	return -1
}

// reconcile gives a new element in the namespace uri the prefix and the xmlns declaration that libxml gives it when
// PHP inserts it. Without its prefix, the element takes the prefix of the nearest declaration of uri. With its
// prefix, the element keeps it, and declares it when the nearest declaration of uri has another prefix.
func reconcile(el *xmlraw.Node, uri string) {
	prefix, found := searchNamespace(el.Parent, uri, el.Parent)
	if found && (el.Name.Space == "" || prefix == el.Name.Space) {
		el.Name.Space, _ = searchNamespace(el, uri, el)
		return
	}
	addDeclaration(el, el.Name.Space, uri)
}

// reconcileAttributes declares prefix on el for its attributes in the namespace uri, unless the nearest declaration
// of uri has the same prefix.
func reconcileAttributes(el *xmlraw.Node, prefix, uri string) {
	if found, ok := searchNamespace(el.Parent, uri, el.Parent); !ok || found != prefix {
		addDeclaration(el, prefix, uri)
	}
}

func addDeclaration(el *xmlraw.Node, prefix, uri string) {
	name := xml.Name{Local: "xmlns"}
	if prefix != "" {
		name = xml.Name{Space: "xmlns", Local: prefix}
	}
	el.Attrs = append(el.Attrs, xml.Attr{Name: name, Value: uri})
}

// searchNamespace returns the prefix of the nearest declaration of uri that is in scope at orig, as libxml
// xmlSearchNsByHref finds it: on each element from start up, its declarations in order, then its own namespace,
// except on orig.
func searchNamespace(start *xmlraw.Node, uri string, orig *xmlraw.Node) (string, bool) {
	for e := start; e != nil && e.Kind == xmlraw.ElementNode; e = e.Parent {
		for _, a := range e.Attrs {
			if prefix, ok := DeclaredPrefix(a); ok && a.Value == uri && inScope(orig, prefix, uri) {
				return prefix, true
			}
		}
		if e != orig && e.NamespaceURI() == uri && inScope(orig, e.Name.Space, uri) {
			return e.Name.Space, true
		}
	}
	return "", false
}

func inScope(n *xmlraw.Node, prefix, uri string) bool {
	bound, ok := n.LookupPrefix(prefix)
	return ok && bound == uri
}

func isBlankText(n *xmlraw.Node) bool {
	return n.Kind == xmlraw.TextNode && strings.Trim(n.Data, markupcore.PHPTrimSet) == ""
}
