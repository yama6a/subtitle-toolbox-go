package ttmlwrite

import (
	"encoding/xml"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/position"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const formatDataKey = string(model.TTML)

const invalidUTF8 = xmlraw.InvalidUTF8Reason

//nolint:gochecknoglobals // the root parameters that change the time model, which the media times of the output replace
var skippedRootParameters = []string{"timeBase", "clockMode", "dropMode", "markerMode"}

var paragraphIDRegexp = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

// Options holds what TTML and iTT output write differently.
type Options struct {
	Format         model.Format // Format of an UnwritableError: model.TTML or model.ITT
	RootAttributes []Attr       // written on <tt> after the stored root attributes, in this order
	Times          []Times      // nil: media times of the cues. Else one entry per cue, in cue order
}

// Attr is one attribute by its qualified name.
type Attr struct{ Name, Value string }

// Times holds the begin and end values of one paragraph.
type Times struct{ Begin, End string }

// writer holds the state of one write: the namespaces of the output, the head and the IDs it hands out.
type writer struct {
	opts      Options
	stripTags bool
	ns        string
	// prefixes holds the bound prefixes, "" for the default namespace, and uris their namespaces.
	prefixes      []string
	uris          map[string]string
	tts, ttm      string
	head          *xmlraw.Node
	styles        Sheet
	usedIDs       map[string]bool
	nextSuffix    map[string]int
	agentIDs      map[string]string
	regionIDs     map[string]string
	forcedRegions map[string]bool
	// cue is the index of the cue that the writer writes, -1 outside the cues.
	cue int
	err *model.UnwritableError
}

type div struct {
	attrs      []Attr
	first      int
	paragraphs []paragraph
}

type paragraph struct {
	id    string
	hasID bool
	rest  string
}

// Document returns the TTML document of s with LF line ends: the XML declaration, the tree and a final LF. It reads
// StripTags of common. The caller applies the line ending and the BOM with writing.Write.
func Document(s *model.Subtitle, common model.CommonWriteOptions, opts Options) ([]byte, error) {
	data, _ := s.FormatData(formatDataKey)
	w := &writer{
		opts:          opts,
		stripTags:     common.StripTags,
		uris:          map[string]string{},
		usedIDs:       map[string]bool{},
		nextSuffix:    map[string]int{},
		agentIDs:      map[string]string{},
		regionIDs:     map[string]string{},
		forcedRegions: map[string]bool{},
		cue:           -1,
	}
	if w.ns, _ = data.String("namespace"); w.ns == "" {
		w.ns = NSTTML
	}
	head, ok := data.String("head")
	if !ok {
		head = "<head/>"
	}
	if err := w.load(storedAttrs(data, "namespaces"), head); err != nil {
		return nil, err
	}
	if title, ok := s.Metadata(model.MetadataTitle); ok {
		w.addTitle(title)
	}
	w.styles = NewSheet(w.ns, w.head)

	// The cues add agents and regions to the head, so the head and the IDs of <tt>, <body> and <div> come after.
	divs := w.divs(s, data)
	out := w.assemble(s, data, divs)
	if w.err == nil && !utf8.ValidString(out) {
		w.fail()
	}
	if w.err != nil {
		return nil, w.err
	}
	return []byte(out), nil
}

// divs returns the paragraphs of the cues, with consecutive cues of the same div attributes in one div.
func (w *writer) divs(s *model.Subtitle, data *model.Data) []div {
	body := w.styleProperties(w.writable(storedAttrs(data, "body")))
	var divs []div
	for i, c := range s.All() {
		w.cue = i
		cueData, _ := c.FormatData(formatDataKey)
		divAttrs := w.writable(storedAttrs(cueData, "div"))
		containers := []Properties{body, w.styleProperties(divAttrs)}
		id, hasID := paragraphID(c)
		para := paragraph{id: id, hasID: hasID, rest: w.paragraph(containers, c, cueData, data.Len() == 0)}
		if n := len(divs); n > 0 && slices.Equal(divs[n-1].attrs, divAttrs) {
			divs[n-1].paragraphs = append(divs[n-1].paragraphs, para)
		} else {
			divs = append(divs, div{attrs: divAttrs, first: i, paragraphs: []paragraph{para}})
		}
	}
	w.cue = -1
	return divs
}

func (w *writer) assemble(s *model.Subtitle, data *model.Data, divs []div) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n<tt")
	b.WriteString(w.rootAttributes(s, storedAttrs(data, "attributes")))
	b.WriteString(">\n  " + Serialize(w.head) + "\n  <body")
	b.WriteString(w.formatAttributes(w.writable(storedAttrs(data, "body"))))
	b.WriteString(">\n")
	if len(divs) == 0 {
		b.WriteString("    <div/>\n")
	}
	for _, d := range divs {
		w.cue = d.first
		b.WriteString("    <div" + w.formatAttributes(d.attrs) + ">\n")
		for _, para := range d.paragraphs {
			b.WriteString("      <p")
			if para.hasID {
				b.WriteString(w.attribute("xml:id", w.unusedID(para.id)))
			}
			b.WriteString(para.rest + "\n")
		}
		b.WriteString("    </div>\n")
	}
	w.cue = -1
	b.WriteString("  </body>\n</tt>\n")
	return b.String()
}

// storedAttrs returns the string fields of the object at key in order. An empty list, as PHP stores an empty
// object, gives none.
func storedAttrs(d *model.Data, key string) []Attr {
	o, _ := d.Object(key)
	out := make([]Attr, 0, o.Len())
	for _, k := range o.Keys() {
		v, _ := o.String(k)
		out = append(out, Attr{k, v})
	}
	return out
}

func paragraphID(c model.Cue) (string, bool) {
	return c.ID, c.ID != "" && paragraphIDRegexp.MatchString(c.ID)
}

func (w *writer) rootAttributes(s *model.Subtitle, stored []Attr) string {
	lang, _ := s.Metadata(model.MetadataLanguage)
	out := w.declarations() + w.attribute("xml:lang", lang)
	for _, a := range stored {
		prefix, local := splitName(a.Name)
		parameter := prefix != "" && slices.Contains(ParameterNamespaces, w.uris[prefix])
		if w.isWritable(a.Name) && (!parameter || !slices.Contains(skippedRootParameters, local)) {
			out += w.formatAttribute(a)
		}
	}
	for _, a := range w.opts.RootAttributes {
		out += w.attribute(a.Name, a.Value)
	}
	return out
}

// paragraph returns the <p> without "<p" and without xml:id, so the caller adds the ID in output order.
func (w *writer) paragraph(containers []Properties, c model.Cue, cueData *model.Data, defaultRegion bool) string {
	alignment, _ := model.AlignmentFromNumpad(c.Position.Alignment.Numpad())
	begin, end := markupcore.CoreTimestamp(c.Start), markupcore.CoreTimestamp(c.End)
	if w.opts.Times != nil {
		begin, end = w.opts.Times[w.cue].Begin, w.opts.Times[w.cue].End
	}
	out := w.attribute("begin", begin) + w.attribute("end", end)

	stored := storedAttrs(cueData, "attributes")
	realigned := !position.AlignmentUnchanged(cueData, alignment)
	if realigned {
		stored = slices.DeleteFunc(stored, func(a Attr) bool { return a.Name == "region" })
	}
	stored = slices.DeleteFunc(w.writable(stored), func(a Attr) bool {
		return slices.Contains([]string{"xml:id", "begin", "end", "dur"}, a.Name)
	})
	if realigned {
		stored = w.withTextAlign(containers, stored, alignment)
	}
	forced := w.forcedDisplay(stored, cueData, c.Forced)
	out += w.formatAttributes(stored)
	region, hasRegion := value(stored, "region")
	if !hasRegion {
		out += w.regionAttribute(alignment, c.Position.Offset, defaultRegion || realigned)
	}
	out += forced

	text := markupcore.RubyAsText(strings.Join(c.TaggedLines(), "\n"))
	if w.stripTags {
		return out + ">" + w.text(markupcore.StripAllTags(text)) + "</p>"
	}
	inherited := Style{}.Apply(w.styles.RegionProperties(region, hasRegion))
	for _, props := range containers {
		inherited = inherited.Apply(props)
	}
	inherited = inherited.Apply(w.styleProperties(stored))
	agent, hasAgent, content := w.content(text, inherited)
	if hasAgent {
		out += w.attribute(w.ttm+":agent", w.agentID(agent))
	}
	return out + ">" + content + "</p>"
}

// regionAttribute writes the region of a cue without a stored region: the offset region when the offset fits the
// alignment, else the region of the alignment. A bottom center cue gets none unless always is set.
func (w *writer) regionAttribute(a model.Alignment, o *model.Offset, always bool) string {
	switch {
	case position.OffsetFits(a, o):
		return w.attribute("region", w.offsetRegionID(a, *o))
	case a != model.BottomCenter || always:
		return w.attribute("region", w.regionID(a))
	default:
		return ""
	}
}

// withTextAlign sets tts:textAlign to the column of a, when the styles of the containers or of the paragraph set
// another text alignment, because that wins over the region.
func (w *writer) withTextAlign(containers []Properties, stored []Attr, a model.Alignment) []Attr {
	textAlign, ok := "", false
	for _, props := range slices.Concat(containers, []Properties{w.styleProperties(stored)}) {
		if v, found := props["textAlign"]; found {
			textAlign, ok = v, true
		}
	}
	if !ok || strings.Trim(textAlign, markupcore.PHPTrimSet) == columnName(a) {
		return stored
	}
	name := w.tts + ":textAlign"
	if i := slices.IndexFunc(stored, func(at Attr) bool { return at.Name == name }); i >= 0 {
		stored[i].Value = columnName(a)
		return stored
	}
	return append(stored, Attr{name, columnName(a)})
}

// forcedDisplay writes the forced flag where the file does not give it already. A stored itts:forcedDisplay of
// the paragraph changes in place. Else the flag comes from the div or the region, and a different flag gives a new
// attribute, which forcedDisplay returns.
func (w *writer) forcedDisplay(stored []Attr, cueData *model.Data, forced bool) string {
	i := w.forcedIndex(stored)
	var given bool
	if i >= 0 {
		given = strings.Trim(stored[i].Value, markupcore.PHPTrimSet) == "true"
	} else {
		div := storedAttrs(cueData, "div")
		if j := w.forcedIndex(div); j >= 0 {
			given = strings.Trim(div[j].Value, markupcore.PHPTrimSet) == "true"
		} else {
			region, _ := value(stored, "region")
			given = w.forcedRegions[region]
		}
	}
	if given == forced {
		return ""
	}
	if i >= 0 {
		stored[i].Value = strconv.FormatBool(forced)
		return ""
	}
	return w.attribute(w.bindIMSCStyling()+":forcedDisplay", strconv.FormatBool(forced))
}

// forcedIndex returns the index of the forcedDisplay attribute whose prefix is bound to the IMSC styling namespace,
// or -1.
func (w *writer) forcedIndex(attrs []Attr) int {
	return slices.IndexFunc(attrs, func(a Attr) bool {
		prefix, local := splitName(a.Name)
		return local == "forcedDisplay" && prefix != "" && w.uris[prefix] == NSIMSCStyle
	})
}

// styleProperties returns the style properties of a detached <p> with the attributes, so prefixes and style
// references resolve as in the reader.
func (w *writer) styleProperties(attrs []Attr) Properties {
	scope := &xmlraw.Node{Kind: xmlraw.ElementNode, Name: xml.Name{Local: "tt"}}
	for _, p := range w.prefixes {
		name := xml.Name{Local: "xmlns"}
		if p != "" {
			name = xml.Name{Space: "xmlns", Local: p}
		}
		scope.Attrs = append(scope.Attrs, xml.Attr{Name: name, Value: w.uris[p]})
	}
	p := &xmlraw.Node{Kind: xmlraw.ElementNode, Name: xml.Name{Local: "p"}, Parent: scope}
	for _, a := range attrs {
		switch prefix, local := splitName(a.Name); prefix {
		case "":
			p.Attrs = append(p.Attrs, xml.Attr{Name: xml.Name{Local: a.Name}, Value: a.Value})
		case "xml":
		default:
			p.Attrs = append(p.Attrs, xml.Attr{Name: xml.Name{Space: prefix, Local: local}, Value: a.Value})
		}
	}
	return w.styles.OwnProperties(p)
}

// writable returns the attributes that the output can hold: no prefix, the prefix xml, or a prefix that the output
// declares. An attribute whose prefix the input declared on the element itself is lost.
func (w *writer) writable(attrs []Attr) []Attr {
	var out []Attr
	for _, a := range attrs {
		if w.isWritable(a.Name) {
			out = append(out, a)
		}
	}
	return out
}

func (w *writer) isWritable(name string) bool {
	prefix, _, found := strings.Cut(name, ":")
	return !found || prefix == "xml" || prefix != "" && w.isBound(prefix)
}

func (w *writer) formatAttributes(attrs []Attr) string {
	var b strings.Builder
	for _, a := range attrs {
		b.WriteString(w.formatAttribute(a))
	}
	return b.String()
}

func (w *writer) formatAttribute(a Attr) string {
	if a.Name == "xml:id" {
		return w.attribute(a.Name, w.unusedID(a.Value))
	}
	return w.attribute(a.Name, a.Value)
}

func (w *writer) attribute(name, value string) string {
	return " " + name + `="` + attributeEscaper.Replace(w.xmlText(value)) + `"`
}

// The attribute escapes add TAB, LF and CR to those of PHP htmlspecialchars, because a reader turns them into
// spaces (https://github.com/yama6a/subtitle-toolbox-php/issues/1071).
//
//nolint:gochecknoglobals // lookup tables of the escapes
var (
	attributeEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "\t", "&#9;", "\n", "&#10;", "\r", "&#13;")
	textEscaper      = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\n", "<br/>")
)

// text returns content text as TTML: entities decoded, the characters that XML forbids removed, escaped, and each
// LF as <br/>.
func (w *writer) text(s string) string {
	return textEscaper.Replace(w.xmlText(markupcore.DecodeEntities(s)))
}

// xmlText removes the characters that XML 1.0 forbids. Text that is not valid UTF-8 fails the write.
func (w *writer) xmlText(s string) string {
	out, ok := xmlraw.Text(s)
	if !ok {
		w.fail()
	}
	return out
}

func (w *writer) fail() {
	if w.err == nil {
		w.err = &model.UnwritableError{Format: w.opts.Format, CueIndex: w.cue, Reason: invalidUTF8, Err: model.ErrInvalidUTF8}
	}
}

// splitName returns the prefix of a qualified name, "" for none, and the local name.
func splitName(name string) (prefix, local string) {
	if prefix, local, ok := strings.Cut(name, ":"); ok {
		return prefix, local
	}
	return "", name
}

func value(attrs []Attr, name string) (string, bool) {
	if i := slices.IndexFunc(attrs, func(a Attr) bool { return a.Name == name }); i >= 0 {
		return attrs[i].Value, true
	}
	return "", false
}
