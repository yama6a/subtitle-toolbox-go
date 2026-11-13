package dcp

import (
	"cmp"
	"crypto/md5" //nolint:gosec // a name-based UUID of version 3 is an MD5 hash by definition
	"encoding/hex"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/position"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const (
	smpte2014 = "http://www.smpte-ra.org/schemas/428-7/2014/DCST"
	xsdNS     = "http://www.w3.org/2001/XMLSchema"
	indent    = "  "
	// lineSpacing is the space between the baselines of 2 lines, and edge the distance of the outer line to the edge of
	// the screen, both in percent of the screen height.
	lineSpacing = 7
	edge        = 8
)

//nolint:gochecknoglobals // the font that DCP-o-matic writes
var defaultFont = []attr{
	{"AspectAdjust", "1.0"},
	{"Color", "FFFFFFFF"},
	{"Effect", "none"},
	{"EffectColor", "FF000000"},
	{"ID", "font"},
	{"Italic", "no"},
	{"Script", "normal"},
	{"Size", "42"},
	{"Underline", "no"},
	{"Weight", "normal"},
}

var (
	styleTag = regexp.MustCompile(`^<(/?)([biu])>$`)
	fontTag  = regexp.MustCompile(`^<font\b`)
)

// file is the part of the output that does not come from the cues.
type file struct {
	interop           bool
	namespace, prefix string
	rate              int
	attributes        []attr
	header            []headerEntry
	loadFonts         []loadFont
	font              []attr
	hasFont           bool
}

// headerEntry is one header element. A fill entry gets the UUID of the subtitles as its value.
type headerEntry struct {
	name, value string
	fill        bool
}

type loadFont struct {
	attrs []attr
	text  string
	fill  bool
}

type writer struct {
	file   file
	prefix string
	base   baseStyle
	cue    int
	err    *model.UnwritableError
}

// baseStyle is the style that the <Font> around the subtitles gives to every run.
type baseStyle struct {
	color        string
	bold, italic bool
	underline    bool
}

// Write writes s in the dialect that Read stored, or as SMPTE ST 428-7:2014 at 24 fps. A stored subtitle without
// FadeUpTime or FadeDownTime gets them. A middle position can be a decimal such as -3.5.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	s, err := writing.SkipImageCues(s, model.DCP, common)
	if err != nil {
		return WriteReport{}, err
	}
	body, err := writeDocument(s)
	if err != nil {
		return WriteReport{}, err
	}
	return WriteReport{}, writing.Write(w, model.DCP, body, common, false)
}

func writeDocument(s *model.Subtitle) ([]byte, error) {
	f := fileOf(s)
	wr := &writer{file: f, cue: -1, base: baseOf(f.font)}
	if f.interop {
		wr.file.rate = interopRate
	} else {
		wr.file.rate = cmp.Or(f.rate, defaultRate)
		if f.prefix != "" {
			wr.prefix = f.prefix + ":"
		}
	}

	subtitles := wr.subtitles(s)
	out := wr.assemble(subtitles)
	if wr.err == nil && !utf8.ValidString(out) {
		wr.fail()
	}
	if wr.err != nil {
		return nil, wr.err
	}
	if _, err := xmlraw.Parse(strings.NewReader(out), xmlraw.ParseOptions{}); err != nil {
		return nil, &model.UnwritableError{Format: model.DCP, CueIndex: -1, Reason: "The formatter wrote XML that is not well-formed."}
	}
	return []byte(out), nil
}

// subtitles returns the <Subtitle> elements of s, inside the <Font> around them when the file has one.
func (w *writer) subtitles(s *model.Subtitle) string {
	depth := 2
	if w.file.interop {
		depth = 1
	}
	inner := depth
	if w.file.hasFont {
		inner++
	}
	var content strings.Builder
	for i, c := range s.All() {
		w.cue = i
		content.WriteString(w.subtitle(c, i+1, inner))
	}
	w.cue = -1
	if !w.file.hasFont {
		return content.String()
	}
	pad := strings.Repeat(indent, depth)
	return pad + "<" + w.prefix + "Font" + w.attributes(w.file.font) + ">\n" + content.String() + pad + "</" + w.prefix + "Font>\n"
}

// assemble returns the document around the subtitles. The Ids come from the hash of the subtitles, so the same
// subtitles give the same file.
func (w *writer) assemble(subtitles string) string {
	f := w.file
	id := uuid(subtitles)
	rootName := w.prefix + "SubtitleReel"
	if f.interop {
		rootName = "DCSubtitle"
	}
	var rootAttrs []attr
	if !f.interop {
		name := "xmlns"
		if f.prefix != "" {
			name += ":" + f.prefix
		}
		rootAttrs = append(rootAttrs, attr{name, f.namespace})
	}
	for _, a := range f.attributes {
		rootAttrs = setAttr(rootAttrs, a.name, a.value)
	}

	var out strings.Builder
	out.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<" + rootName + w.attributes(rootAttrs) + ">\n")
	for _, h := range f.header {
		value := h.value
		if h.fill {
			value = "urn:uuid:" + id
		}
		out.WriteString(indent + "<" + w.prefix + h.name + ">" + w.escape(value) + "</" + w.prefix + h.name + ">\n")
	}
	for _, l := range f.loadFonts {
		text := l.text
		if l.fill {
			text = "urn:uuid:" + uuid("font "+id)
		}
		out.WriteString(indent + w.loadFont(l.attrs, text) + "\n")
	}
	if f.interop {
		out.WriteString(subtitles)
	} else {
		out.WriteString(indent + "<" + w.prefix + "SubtitleList>\n" + subtitles + indent + "</" + w.prefix + "SubtitleList>\n")
	}
	out.WriteString("</" + rootName + ">\n")
	return out.String()
}

func (w *writer) loadFont(attrs []attr, text string) string {
	tag := w.prefix + "LoadFont"
	if text == "" {
		return "<" + tag + w.attributes(attrs) + "/>"
	}
	return "<" + tag + w.attributes(attrs) + ">" + w.escape(text) + "</" + tag + ">"
}

// fileOf returns the stored file data when its dialect is interop or smpte, else the defaults of a new file with the
// stored header on top, plus the title and the language of the metadata.
func fileOf(s *model.Subtitle) file {
	data, _ := s.FormatData(formatDataKey)
	var f file
	if dialect, _ := data.String("dialect"); dialect == "interop" || dialect == "smpte" {
		f = storedFile(data, dialect == "interop")
	} else {
		f = newFile(data)
	}

	field := "ContentTitleText"
	if f.interop {
		field = "MovieTitle"
	}
	if title, ok := s.Metadata(model.MetadataTitle); ok && (!f.interop || f.hasHeader(field)) {
		f.setHeader(field, title)
	}
	if language, ok := s.Metadata(model.MetadataLanguage); ok && !f.interop && f.hasHeader("Language") {
		f.setHeader("Language", language)
	}
	return f
}

func (f *file) hasHeader(name string) bool {
	return slices.ContainsFunc(f.header, func(h headerEntry) bool { return h.name == name })
}

func (f *file) setHeader(name, value string) {
	if i := slices.IndexFunc(f.header, func(h headerEntry) bool { return h.name == name }); i >= 0 {
		f.header[i] = headerEntry{name: name, value: value}
		return
	}
	f.header = append(f.header, headerEntry{name: name, value: value})
}

func storedFile(d *model.Data, interop bool) file {
	f := file{interop: interop, rate: defaultRate, namespace: smpte2014}
	if ns, ok := d.String("namespace"); ok {
		f.namespace = ns
	}
	f.prefix, _ = d.String("prefix")
	if n, ok := d.Int("timeCodeRate"); ok {
		f.rate = int(n)
	}
	f.attributes = attrsOf(d, "attributes")
	for _, h := range attrsOf(d, "header") {
		f.header = append(f.header, headerEntry{name: h.name, value: h.value})
	}
	f.loadFonts = loadFontsOf(d)
	if _, ok := d.Get("font"); ok {
		f.font, f.hasFont = attrsOf(d, "font"), true
	}
	return f
}

func newFile(d *model.Data) file {
	f := file{
		namespace:  smpte2014,
		rate:       defaultRate,
		attributes: []attr{{"xmlns:xs", xsdNS}},
		header: []headerEntry{
			{name: "Id", fill: true},
			{name: "ContentTitleText"},
			{name: "IssueDate", value: time.Now().UTC().Format("2006-01-02T15:04:05")},
			{name: "ReelNumber", value: "1"},
			{name: "Language", value: "en"},
			{name: "EditRate", value: strconv.Itoa(defaultRate) + " 1"},
			{name: "TimeCodeRate", value: strconv.Itoa(defaultRate)},
			{name: "StartTime", value: "00:00:00:00"},
		},
		loadFonts: []loadFont{{attrs: []attr{{"ID", "font"}}, fill: true}},
		font:      defaultFont,
		hasFont:   true,
	}
	for _, h := range attrsOf(d, "header") {
		f.setHeader(h.name, h.value)
	}
	return f
}

// attrsOf returns the object at key as attributes in stored order, nil for a missing key or an empty list.
func attrsOf(d *model.Data, key string) []attr {
	obj, ok := d.Object(key)
	if !ok {
		return nil
	}
	return dataAttrs(obj)
}

func dataAttrs(d *model.Data) []attr {
	keys := d.Keys()
	out := make([]attr, 0, len(keys))
	for _, k := range keys {
		v, _ := d.String(k)
		out = append(out, attr{k, v})
	}
	return out
}

func loadFontsOf(d *model.Data) []loadFont {
	list, _ := d.List("loadFonts")
	var out []loadFont
	for _, item := range list {
		obj, ok := item.(*model.Data)
		if !ok {
			continue
		}
		text, hasText := obj.String("text")
		out = append(out, loadFont{attrs: attrsOf(obj, "attributes"), text: text, fill: !hasText})
	}
	return out
}

// uuid returns the name-based UUID of version 3 of name: its MD5 hash in the groups 8-4-4-4-12.
func uuid(name string) string {
	sum := md5.Sum([]byte(name)) //nolint:gosec // see the import
	h := hex.EncodeToString(sum[:])
	variant, _ := strconv.ParseUint(h[16:17], 16, 8)
	return h[0:8] + "-" + h[8:12] + "-3" + h[13:16] + "-" + strconv.FormatUint(variant&3|8, 16) + h[17:20] + "-" + h[20:32]
}

func (w *writer) subtitle(c model.Cue, number, depth int) string {
	cueData, _ := c.FormatData(formatDataKey)
	attrs := attrsOf(cueData, "subtitle")
	if attrs == nil {
		attrs = []attr{{"SpotNumber", ""}, {"TimeIn", ""}, {"TimeOut", ""}}
	}
	attrs = setAttr(attrs, "SpotNumber", strconv.Itoa(number))
	attrs = setAttr(attrs, "TimeIn", w.timeCode(c.Start))
	attrs = setAttr(attrs, "TimeOut", w.timeCode(c.End))
	fade := "00:00:00:00"
	if w.file.interop {
		fade = "20"
	}
	for _, name := range []string{"FadeUpTime", "FadeDownTime"} {
		if !slices.ContainsFunc(attrs, func(a attr) bool { return a.name == name }) {
			attrs = append(attrs, attr{name, fade})
		}
	}

	lines := w.visibleLines(c.Lines)
	texts := w.textAttributes(c, cueData, len(lines))
	pad := strings.Repeat(indent, depth)
	var out strings.Builder
	out.WriteString(pad + "<" + w.prefix + "Subtitle" + w.attributes(attrs) + ">\n")
	for i, l := range lines {
		out.WriteString(pad + indent + "<" + w.prefix + "Text" + w.attributes(texts[i]) + ">" + w.runs(l) + "</" + w.prefix + "Text>\n")
	}
	out.WriteString(pad + "</" + w.prefix + "Subtitle>\n")
	return out.String()
}

// visibleLines returns the lines with ruby as text and the open tags carried over, without the lines that show no text.
func (w *writer) visibleLines(lines []string) []string {
	ruby := make([]string, len(lines))
	for i, l := range lines {
		ruby[i] = markupcore.RubyAsText(l)
	}
	return slices.DeleteFunc(markupcore.CarryOpenTags(ruby), func(l string) bool {
		return markupcore.VisibleText(l) == ""
	})
}

// textAttributes returns the attributes of the <Text> elements of a cue with count lines. The stored attributes win
// while the cue keeps its line count and its alignment. Else an offset in the cell of the alignment places the lines
// around its height, else the alignment does.
func (w *writer) textAttributes(c model.Cue, cueData *model.Data, count int) [][]attr {
	if stored, ok := storedTexts(cueData, count); ok && alignmentOf(stored) == c.Position.Alignment {
		return stored
	}
	a := c.Position.Alignment
	var offset *model.Offset
	if position.OffsetFits(a, c.Position.Offset) {
		offset = c.Position.Offset
	}
	out := make([][]attr, count)
	for i := range count {
		out[i] = w.positioned(a, offset, i, count)
	}
	return out
}

// storedTexts returns the stored <Text> attributes of a cue when it has count lines, count above 0.
func storedTexts(cueData *model.Data, count int) ([][]attr, bool) {
	list, ok := cueData.List("texts")
	if !ok || count == 0 || len(list) != count {
		return nil, false
	}
	stored := make([][]attr, count)
	for i, item := range list {
		if obj, ok := item.(*model.Data); ok {
			stored[i] = dataAttrs(obj)
		}
	}
	return stored, true
}

// positioned returns the attributes of line i of count lines that the alignment a places. An offset moves the anchor edge
// of the lines: the bottom line of a bottom row, the top line of a top row, the middle of a middle row. The other
// lines stay on the same side of the anchor, so they stay on the screen. Without an offset the anchor edge is 8 %
// from the screen edge.
func (w *writer) positioned(a model.Alignment, offset *model.Offset, i, count int) []attr {
	vAlign, hAlign, vPosition, hPosition := "Valign", "Halign", "Vposition", "Hposition"
	if w.file.interop {
		vAlign, hAlign, vPosition, hPosition = "VAlign", "HAlign", "VPosition", "HPosition"
	}
	var attrs []attr
	switch a.Column() {
	case 1:
		attrs = append(attrs, attr{hAlign, "left"})
		if offset != nil {
			attrs = append(attrs, attr{hPosition, formatPosition(offset.X)})
		}
	case 3:
		attrs = append(attrs, attr{hAlign, "right"})
		if offset != nil {
			attrs = append(attrs, attr{hPosition, formatPosition(100 - offset.X)})
		}
	default:
		if offset != nil && position.Round(offset.X) != 50 {
			attrs = append(attrs, attr{hPosition, formatPosition(offset.X - 50)})
		}
	}
	align, pos := vertical(a.Row(), offset, i, count)
	return append(attrs, attr{vAlign, align}, attr{vPosition, formatPosition(pos)})
}

// vertical returns Valign and the position of line i of count lines in the row. Row is 0 bottom, 1 middle or 2 top.
func vertical(row int, offset *model.Offset, i, count int) (string, float64) {
	anchor := float64(edge)
	switch row {
	case 0:
		if offset != nil {
			anchor = 100 - offset.Y
		}
		return "bottom", anchor + float64(count-1-i)*lineSpacing
	case 2:
		if offset != nil {
			anchor = offset.Y
		}
		return "top", anchor + float64(i)*lineSpacing
	default:
		centre := 0.0
		if offset != nil {
			centre = offset.Y - 50
		}
		return "center", centre + (float64(i)-float64(count-1)/2)*lineSpacing
	}
}

func formatPosition(v float64) string {
	if s := position.Format(v); s != "-0" {
		return s
	}
	return "0"
}

// runs writes the text of a line, with a <Font> element for each run whose style differs from the base style.
func (w *writer) runs(line string) string {
	var st runStyle
	var out strings.Builder
	for i, token := range markupcore.SplitTags(markupcore.WebVTTColorsToFont(line)) {
		if i%2 == 1 {
			st.apply(markupcore.ASCIILower(token))
			continue
		}
		text := markupcore.DecodeEntities(token)
		if text == "" {
			continue
		}
		escaped := w.escape(text)
		if attrs := w.differences(st); len(attrs) > 0 {
			escaped = "<" + w.prefix + "Font" + w.attributes(attrs) + ">" + escaped + "</" + w.prefix + "Font>"
		}
		out.WriteString(escaped)
	}
	return out.String()
}

// runStyle is the style of the text at one point of a line: the nesting depth of <b>, <i> and <u>, and the colors of
// the open <font> tags as 6 lower-case hex digits, "" for a <font> without a color.
type runStyle struct {
	bold, italic, underline int
	colors                  []string
}

// apply changes the style by a tag in lower case. Other tags change nothing.
func (st *runStyle) apply(tag string) {
	switch m := styleTag.FindStringSubmatch(tag); {
	case m != nil:
		delta := 1
		if m[1] == "/" {
			delta = -1
		}
		switch m[2] {
		case "b":
			st.bold = max(0, st.bold+delta)
		case "i":
			st.italic = max(0, st.italic+delta)
		default:
			st.underline = max(0, st.underline+delta)
		}
	case fontTag.MatchString(tag):
		color := fontColor(tag)
		if color == "" && len(st.colors) > 0 {
			color = st.colors[len(st.colors)-1]
		}
		st.colors = append(st.colors, color)
	case tag == "</font>":
		st.colors = st.colors[:max(0, len(st.colors)-1)]
	}
}

// differences returns the attributes of the inner <Font> for text in style st: Color, Italic, Weight and Underlined
// or Underline, each only when it differs from the base style.
func (w *writer) differences(st runStyle) []attr {
	var attrs []attr
	color := "ffffff"
	if n := len(st.colors); n > 0 && st.colors[n-1] != "" {
		color = st.colors[n-1]
	}
	if color != w.base.color {
		attrs = append(attrs, attr{"Color", "FF" + strings.ToUpper(color)})
	}
	if (st.italic > 0) != w.base.italic {
		attrs = append(attrs, attr{"Italic", yesNo(st.italic > 0)})
	}
	if (st.bold > 0) != w.base.bold {
		weight := "normal"
		if st.bold > 0 {
			weight = "bold"
		}
		attrs = append(attrs, attr{"Weight", weight})
	}
	if (st.underline > 0) != w.base.underline {
		name := "Underline"
		if w.file.interop {
			name = "Underlined"
		}
		attrs = append(attrs, attr{name, yesNo(st.underline > 0)})
	}
	return attrs
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// fontColor returns the color of a <font> tag as 6 lower-case hex digits, "" for none.
func fontColor(tag string) string {
	if attr, ok := markupcore.FontColor(tag); ok {
		if hex, ok := markupcore.ColorHex(attr); ok {
			return hex[1:]
		}
	}
	return ""
}

func baseOf(font []attr) baseStyle {
	values := map[string]string{}
	for _, a := range font {
		values[markupcore.ASCIILower(a.name)] = markupcore.ASCIILower(strings.Trim(a.value, markupcore.PHPTrimSet))
	}
	b := baseStyle{color: "ffffff", bold: values["weight"] == "bold"}
	if m := colorRegexp.FindStringSubmatch(values["color"]); m != nil {
		b.color = m[1]
	}
	switch values["italic"] {
	case "yes", "left", "right":
		b.italic = true
	}
	underline, ok := values["underlined"]
	if !ok {
		underline = values["underline"]
	}
	b.underline = underline == "yes"
	return b
}

// timeCode writes HH:MM:SS:EE in rate units per second. EE has 3 digits for Interop and for a rate above 100. A time
// that rounds to 100000 hours stops at the last unit below the limit, so the output reads back.
func (w *writer) timeCode(d time.Duration) string {
	rate := int64(w.file.rate)
	units := model.FrameRate{Num: rate, Den: 1}.Frames(max(d, 0).Round(time.Millisecond))
	units = min(units, int64(timecode.MaxHours)*3600*rate-1)
	whole, frame := units/rate, units%rate
	width := 2
	if w.file.interop || rate > 100 {
		width = 3
	}
	return pad(whole/3600, 2) + ":" + pad(whole/60%60, 2) + ":" + pad(whole%60, 2) + ":" + pad(frame, width)
}

func pad(n int64, width int) string {
	s := strconv.FormatInt(n, 10)
	if len(s) < width {
		s = strings.Repeat("0", width-len(s)) + s
	}
	return s
}

func (w *writer) attributes(attrs []attr) string {
	var b strings.Builder
	for _, a := range attrs {
		b.WriteString(" " + a.name + `="` + w.escape(a.value) + `"`)
	}
	return b.String()
}

var escaper = strings.NewReplacer( //nolint:gochecknoglobals // a fixed replacer
	"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

// escape removes the characters that XML forbids, then escapes &, <, > and ". Text that is not valid UTF-8 fails the write.
func (w *writer) escape(s string) string {
	out, ok := xmlraw.Text(s)
	if !ok {
		w.fail()
	}
	return escaper.Replace(out)
}

func (w *writer) fail() {
	if w.err == nil {
		w.err = &model.UnwritableError{Format: model.DCP, CueIndex: w.cue, Reason: xmlraw.InvalidUTF8Reason, Err: model.ErrInvalidUTF8}
	}
}
