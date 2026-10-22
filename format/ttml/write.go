package ttml

import (
	"encoding/xml"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/position"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const invalidUTF8 = "The output cannot hold text that is not valid UTF-8. Read a file in another encoding with its source encoding."

//nolint:gochecknoglobals // the root parameters that change the time model, which the media times of the output replace
var skippedRootParameters = []string{"timeBase", "clockMode", "dropMode", "markerMode"}

// paragraphIDRegexp ends with \n? because PHP ends the pattern with $, which also matches before a final LF.
var paragraphIDRegexp = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*\n?$`)

// WriteOptions holds the settings of the TTML writer. It has no fields yet.
type WriteOptions struct{}

// profile holds what TTML and iTT output write differently.
type profile struct {
	format model.Format
	// rootAttributes go after the stored attributes of <tt>.
	rootAttributes []attr
	timestamp      func(time.Duration) string
}

// attr is one stored attribute by its qualified name.
type attr struct {
	name, value string
}

// writer holds the state of one write: the namespaces of the output, the head and the IDs it hands out.
type writer struct {
	profile   profile
	stripTags bool
	ns        string
	// prefixes holds the bound prefixes, "" for the default namespace, and uris their namespaces.
	prefixes      []string
	uris          map[string]string
	tts, ttm      string
	head          *xmlraw.Node
	styles        styles
	usedIDs       map[string]bool
	nextSuffix    map[string]int
	agentIDs      map[string]string
	regionIDs     map[model.Alignment]string
	forcedRegions map[string]bool
	// cue is the index of the cue that the writer writes, -1 outside the cues.
	cue int
	err *model.UnwritableError
}

type div struct {
	attrs      []attr
	first      int
	paragraphs []paragraph
}

type paragraph struct {
	id    string
	hasID bool
	rest  string
}

// Write writes s as TTML. It keeps the namespace, head and attributes that Read stored.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	body, err := writeDocument(s, common, profile{format: model.TTML, timestamp: markupcore.CoreTimestamp})
	if err != nil {
		return err
	}
	return writing.Write(w, model.TTML, body, common, false)
}

// writeDocument returns the TTML document of s with LF line ends.
func writeDocument(s *model.Subtitle, common model.CommonWriteOptions, p profile) ([]byte, error) {
	data, _ := s.FormatData(formatDataKey)
	w := &writer{
		profile:       p,
		stripTags:     common.StripTags,
		uris:          map[string]string{},
		usedIDs:       map[string]bool{},
		nextSuffix:    map[string]int{},
		agentIDs:      map[string]string{},
		regionIDs:     map[model.Alignment]string{},
		forcedRegions: map[string]bool{},
		cue:           -1,
	}
	if w.ns, _ = data.String("namespace"); w.ns == "" {
		w.ns = nsTTML
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
	w.styles = newStyles(w.ns, w.head)

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
		containers := []properties{body, w.styleProperties(divAttrs)}
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
	b.WriteString(">\n  " + serialize(w.head) + "\n  <body")
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
func storedAttrs(d *model.Data, key string) []attr {
	o, _ := d.Object(key)
	out := make([]attr, 0, o.Len())
	for _, k := range o.Keys() {
		v, _ := o.String(k)
		out = append(out, attr{k, v})
	}
	return out
}

func paragraphID(c model.Cue) (string, bool) {
	return c.ID, c.ID != "" && paragraphIDRegexp.MatchString(c.ID)
}

func (w *writer) rootAttributes(s *model.Subtitle, stored []attr) string {
	lang, _ := s.Metadata(model.MetadataLanguage)
	out := w.declarations() + w.attribute("xml:lang", lang)
	for _, a := range stored {
		prefix, local := splitName(a.name)
		parameter := prefix != "" && slices.Contains(parameterNamespaces, w.uris[prefix])
		if w.isWritable(a.name) && (!parameter || !slices.Contains(skippedRootParameters, local)) {
			out += w.formatAttribute(a)
		}
	}
	for _, a := range w.profile.rootAttributes {
		out += w.attribute(a.name, a.value)
	}
	return out
}

// paragraph returns the <p> without "<p" and without xml:id, so the caller adds the ID in output order.
func (w *writer) paragraph(containers []properties, c model.Cue, cueData *model.Data, defaultRegion bool) string {
	alignment, _ := model.AlignmentFromNumpad(c.Alignment.Numpad())
	out := w.attribute("begin", w.profile.timestamp(c.Start)) + w.attribute("end", w.profile.timestamp(c.End))

	stored := storedAttrs(cueData, "attributes")
	realigned := !position.AlignmentUnchanged(cueData, alignment)
	if realigned {
		stored = slices.DeleteFunc(stored, func(a attr) bool { return a.name == "region" })
	}
	stored = slices.DeleteFunc(w.writable(stored), func(a attr) bool {
		return slices.Contains([]string{"xml:id", "begin", "end", "dur"}, a.name)
	})
	if realigned {
		stored = w.withTextAlign(containers, stored, alignment)
	}
	forced := w.forcedDisplay(stored, cueData, c.Forced)
	out += w.formatAttributes(stored)
	region, hasRegion := value(stored, "region")
	if !hasRegion && (alignment != model.BottomCenter || defaultRegion || realigned) {
		out += w.attribute("region", w.regionID(alignment))
	}
	out += forced

	text := markupcore.RubyAsText(strings.Join(c.Lines, "\n"))
	if w.stripTags {
		return out + ">" + w.text(markupcore.StripAllTags(text)) + "</p>"
	}
	inherited := style{}.apply(w.styles.regionProperties(region, hasRegion))
	for _, props := range containers {
		inherited = inherited.apply(props)
	}
	inherited = inherited.apply(w.styleProperties(stored))
	agent, hasAgent, content := w.content(text, inherited)
	if hasAgent {
		out += w.attribute(w.ttm+":agent", w.agentID(agent))
	}
	return out + ">" + content + "</p>"
}

// withTextAlign sets tts:textAlign to the column of a, when the styles of the containers or of the paragraph set
// another text alignment, because that wins over the region.
func (w *writer) withTextAlign(containers []properties, stored []attr, a model.Alignment) []attr {
	textAlign, ok := "", false
	for _, props := range slices.Concat(containers, []properties{w.styleProperties(stored)}) {
		if v, found := props["textAlign"]; found {
			textAlign, ok = v, true
		}
	}
	if !ok || strings.Trim(textAlign, markupcore.PHPTrimSet) == columnName(a) {
		return stored
	}
	name := w.tts + ":textAlign"
	if i := slices.IndexFunc(stored, func(at attr) bool { return at.name == name }); i >= 0 {
		stored[i].value = columnName(a)
		return stored
	}
	return append(stored, attr{name, columnName(a)})
}

// forcedDisplay writes the forced flag where the file does not give it already. A stored itts:forcedDisplay of
// the paragraph changes in place. Else the flag comes from the div or the region, and a different flag gives a new
// attribute, which forcedDisplay returns.
func (w *writer) forcedDisplay(stored []attr, cueData *model.Data, forced bool) string {
	i := w.forcedIndex(stored)
	var given bool
	if i >= 0 {
		given = strings.Trim(stored[i].value, markupcore.PHPTrimSet) == "true"
	} else {
		div := storedAttrs(cueData, "div")
		if j := w.forcedIndex(div); j >= 0 {
			given = strings.Trim(div[j].value, markupcore.PHPTrimSet) == "true"
		} else {
			region, _ := value(stored, "region")
			given = w.forcedRegions[region]
		}
	}
	if given == forced {
		return ""
	}
	if i >= 0 {
		stored[i].value = strconv.FormatBool(forced)
		return ""
	}
	return w.attribute(w.bindIMSCStyling()+":forcedDisplay", strconv.FormatBool(forced))
}

// forcedIndex returns the index of the forcedDisplay attribute whose prefix is bound to the IMSC styling namespace,
// or -1.
func (w *writer) forcedIndex(attrs []attr) int {
	return slices.IndexFunc(attrs, func(a attr) bool {
		prefix, local := splitName(a.name)
		return local == "forcedDisplay" && prefix != "" && w.uris[prefix] == nsIMSCStyle
	})
}

// styleProperties returns the style properties of a detached <p> with the attributes, so prefixes and style
// references resolve as in the reader.
func (w *writer) styleProperties(attrs []attr) properties {
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
		switch prefix, local := splitName(a.name); prefix {
		case "":
			p.Attrs = append(p.Attrs, xml.Attr{Name: xml.Name{Local: a.name}, Value: a.value})
		case "xml":
		default:
			p.Attrs = append(p.Attrs, xml.Attr{Name: xml.Name{Space: prefix, Local: local}, Value: a.value})
		}
	}
	return w.styles.ownProperties(p)
}

// writable returns the attributes that the output can hold: no prefix, the prefix xml, or a prefix that the output
// declares. An attribute whose prefix the input declared on the element itself is lost.
func (w *writer) writable(attrs []attr) []attr {
	var out []attr
	for _, a := range attrs {
		if w.isWritable(a.name) {
			out = append(out, a)
		}
	}
	return out
}

func (w *writer) isWritable(name string) bool {
	prefix, _, found := strings.Cut(name, ":")
	return !found || prefix == "xml" || prefix != "" && w.isBound(prefix)
}

func (w *writer) formatAttributes(attrs []attr) string {
	var b strings.Builder
	for _, a := range attrs {
		b.WriteString(w.formatAttribute(a))
	}
	return b.String()
}

func (w *writer) formatAttribute(a attr) string {
	if a.name == "xml:id" {
		return w.attribute(a.name, w.unusedID(a.value))
	}
	return w.attribute(a.name, a.value)
}

func (w *writer) attribute(name, value string) string {
	return " " + name + `="` + attributeEscaper.Replace(w.xmlText(value)) + `"`
}

//nolint:gochecknoglobals // the escapes of htmlspecialchars with ENT_XML1, which PHP applies
var (
	attributeEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	textEscaper      = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\n", "<br/>")
)

// text returns content text as TTML: entities decoded, the characters that XML forbids removed, escaped, and each
// LF as <br/>.
func (w *writer) text(s string) string {
	return textEscaper.Replace(w.xmlText(markupcore.DecodeEntities(s)))
}

// xmlText removes the characters that XML 1.0 forbids: the C0 controls except tab, LF and CR, U+FFFE and U+FFFF.
// Text that is not valid UTF-8 fails the write.
func (w *writer) xmlText(s string) string {
	if !utf8.ValidString(s) {
		w.fail()
		return ""
	}
	return strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' || r == 0xfffe || r == 0xffff {
			return -1
		}
		return r
	}, s)
}

func (w *writer) fail() {
	if w.err == nil {
		w.err = &model.UnwritableError{Format: w.profile.format, CueIndex: w.cue, Reason: invalidUTF8, Err: model.ErrInvalidUTF8}
	}
}

// splitName returns the prefix of a qualified name, "" for none, and the local name.
func splitName(name string) (prefix, local string) {
	if prefix, local, ok := strings.Cut(name, ":"); ok {
		return prefix, local
	}
	return "", name
}

func value(attrs []attr, name string) (string, bool) {
	if i := slices.IndexFunc(attrs, func(a attr) bool { return a.name == name }); i >= 0 {
		return attrs[i].value, true
	}
	return "", false
}
