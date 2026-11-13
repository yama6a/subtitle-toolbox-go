package dcp

import (
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/position"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const notWellFormed = "The content is not well-formed XML."

var (
	startRegexp  = xmlraw.StartRegexp(rootPattern) //nolint:gochecknoglobals // a compiled pattern
	decimalTime  = regexp.MustCompile(`^(\d+):(\d{1,2}):(\d{1,2})\.(\d+)$`)
	unitsTime    = regexp.MustCompile(`^(\d+):(\d{1,2}):(\d{1,2}):(\d{1,3})$`)
	timeCodeRate = regexp.MustCompile(`^\d+$`)
	editRate     = regexp.MustCompile(`^(\d+)[\t\n\v\f\r ]+(\d+)$`)
	colorRegexp  = regexp.MustCompile(`^[0-9a-f]{2}([0-9a-f]{6})$`)
	numberPrefix = regexp.MustCompile(`^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?`)
)

// textWhiteSpace removes the characters that a <Text> element does not show.
var textWhiteSpace = strings.NewReplacer("\r", "", "\n", "", "\t", "") //nolint:gochecknoglobals // a fixed replacer

// attr is one attribute by its local name. Prefixes and namespace declarations are not kept, as the PHP library does.
type attr struct {
	name, value string
}

// line is one <Text> element of a subtitle: its markup, its height on the screen and its attributes.
type line struct {
	text  string
	y     float64
	attrs []attr
}

type reader struct {
	st      *parsing.State
	interop bool
	rate    int
	cues    []model.Cue
}

// Read reads Interop and SMPTE DCP subtitles. It expects UTF-8, as stbox.Parse passes it. Text that is not valid
// UTF-8 makes the XML malformed, in both modes.
func Read(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.DCP, err)
	}
	st := parsing.NewState(model.DCP, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}
	rd := &reader{st: st}
	if err := rd.read(content, s); err != nil {
		return nil, nil, err
	}
	return s, st.Warnings(), nil
}

func parseError(line int, msg string) *model.ParseError {
	return &model.ParseError{Format: model.DCP, Line: line, Msg: msg}
}

func (rd *reader) read(content []byte, s *model.Subtitle) error {
	root, err := rd.parse(content)
	if err != nil {
		return err
	}
	switch root.Name.Local {
	case "DCSubtitle":
		rd.interop = true
	case "SubtitleReel":
	default:
		return parseError(model.NoLine, "The root element must be <DCSubtitle> or <SubtitleReel>, not <"+xmlraw.QName(root.Name)+">.")
	}
	rd.readMetadata(root, s)

	rd.rate = interopRate
	if !rd.interop {
		rd.rate = smpteRate(root)
	}
	if err := s.SetFormatData(formatDataKey, rd.fileData(root)); err != nil {
		return err
	}
	if err := rd.readElement(root, nil); err != nil {
		return err
	}
	parsing.AddParsed(s, rd.cues, nil)
	return nil
}

// parse returns the root element. The text before the XML is a warning in lenient mode.
func (rd *reader) parse(content []byte) (*xmlraw.Node, error) {
	if !utf8.Valid(content) {
		return nil, parseError(model.NoLine, notWellFormed)
	}
	text := string(content)
	repaired, skipped := xmlraw.SkipLeadingText(text, startRegexp)
	if skipped != "" {
		line, lines, msg := xmlraw.LeadingText(text, skipped)
		if !rd.st.Lenient() {
			return nil, parseError(line, msg)
		}
		rd.st.Warn(msg+" The parser skipped it.", line, model.NoBlock, lines, model.Repaired)
	}
	doc, err := xmlraw.Parse(strings.NewReader(repaired), xmlraw.ParseOptions{})
	if err != nil {
		return nil, parseError(model.NoLine, notWellFormed)
	}
	root, ok := doc.Root()
	if !ok {
		return nil, parseError(model.NoLine, notWellFormed)
	}
	return root, nil
}

func (rd *reader) readMetadata(root *xmlraw.Node, s *model.Subtitle) {
	titleName := "ContentTitleText"
	if rd.interop {
		titleName = "MovieTitle"
	}
	if title := childText(root, titleName); title != "" {
		s.SetMetadata(model.MetadataTitle, title)
	}
	// Interop names the language in words, such as "English", so only SMPTE gives a language tag.
	if language := childText(root, "Language"); !rd.interop && language != "" {
		s.SetMetadata(model.MetadataLanguage, language)
	}
}

// smpteRate returns TimeCodeRate when it is a whole number from 1 to 1000, else EditRate rounded when it is a pair of
// numbers above 0 and the result is from 1 to 1000, else 24.
func smpteRate(root *xmlraw.Node) int {
	if t := childText(root, "TimeCodeRate"); timeCodeRate.MatchString(t) {
		if n, err := strconv.Atoi(t); err == nil && n >= 1 && n <= maxRate {
			return n
		}
	}
	if n, ok := editRateOf(childText(root, "EditRate")); ok {
		return n
	}
	return defaultRate
}

func editRateOf(text string) (int, bool) {
	m := editRate.FindStringSubmatch(text)
	if m == nil {
		return 0, false
	}
	a, errA := strconv.Atoi(m[1])
	b, errB := strconv.Atoi(m[2])
	if errA != nil || errB != nil || a <= 0 || b <= 0 {
		return 0, false
	}
	n := int(math.Round(float64(a) / float64(b)))
	return n, n >= 1 && n <= maxRate
}

// fileData returns the parts of the file around the subtitles that the writer writes back: the root, the header
// elements, the <LoadFont> elements and the attributes of the first <Font> around the subtitles.
func (rd *reader) fileData(root *xmlraw.Node) *model.Data {
	data := model.NewData(model.Field{Key: "dialect", Value: "smpte"})
	if rd.interop {
		data.Set("dialect", "interop")
	} else {
		data.Set("namespace", root.NamespaceURI())
		data.Set("prefix", root.Name.Space)
		data.Set("timeCodeRate", int64(rd.rate))
	}
	attrs := attributes(root)
	for _, a := range root.Attrs {
		if a.Name.Space == "xmlns" && a.Name.Local == "xs" {
			attrs = setAttr(attrs, "xmlns:xs", a.Value)
		}
	}
	data.Set("attributes", attrData(attrs))

	header, loadFonts := headerOf(root)
	data.Set("header", attrData(header))
	data.Set("loadFonts", loadFonts)

	list := root
	if l := firstChild(root, "SubtitleList"); l != nil && !rd.interop {
		list = l
	}
	if font := firstChild(list, "Font"); font != nil {
		data.Set("font", attrData(attributes(font)))
	}
	return data
}

// headerOf returns the children of the root that are no subtitles: the elements without child elements as header
// entries, and the <LoadFont> elements as format data.
func headerOf(root *xmlraw.Node) (header []attr, loadFonts []any) {
	loadFonts = []any{}
	for _, child := range elements(root) {
		switch child.Name.Local {
		case "Font", "Subtitle", "SubtitleList":
		case "LoadFont":
			loadFonts = append(loadFonts, model.NewData(
				model.Field{Key: "attributes", Value: attrData(attributes(child))},
				model.Field{Key: "text", Value: child.Text()},
			))
		default:
			if len(elements(child)) == 0 {
				header = setAttr(header, child.Name.Local, child.Text())
			}
		}
	}
	return header, loadFonts
}

// attrData returns the attributes as format data. An empty list is an empty list, as PHP stores an empty array.
func attrData(attrs []attr) any {
	if len(attrs) == 0 {
		return []any{}
	}
	d := model.NewData()
	for _, a := range attrs {
		d.Set(a.name, a.value)
	}
	return d
}

// attributes returns the attributes of n by local name, without namespace declarations. A repeated name keeps its first
// place and its last value.
func attributes(n *xmlraw.Node) []attr {
	var out []attr
	for _, a := range n.Attrs {
		switch {
		case xmlraw.IsNamespaceDecl(a):
		case a.Name.Space == "xmlns":
			// libxml reads xmlns:5xs, a declaration with no NCName, as a plain attribute with that whole name.
			out = setAttr(out, "xmlns:"+a.Name.Local, a.Value)
		default:
			out = setAttr(out, a.Name.Local, a.Value)
		}
	}
	return out
}

func setAttr(attrs []attr, name, value string) []attr {
	if i := slices.IndexFunc(attrs, func(a attr) bool { return a.name == name }); i >= 0 {
		attrs[i].value = value
		return attrs
	}
	return append(attrs, attr{name, value})
}

func elements(n *xmlraw.Node) []*xmlraw.Node {
	var out []*xmlraw.Node
	for _, c := range n.Children {
		if c.Kind == xmlraw.ElementNode {
			out = append(out, c)
		}
	}
	return out
}

func firstChild(n *xmlraw.Node, local string) *xmlraw.Node {
	for _, c := range elements(n) {
		if c.Name.Local == local {
			return c
		}
	}
	return nil
}

// childText returns the trimmed text of the first child element with the local name, "" for none.
func childText(n *xmlraw.Node, local string) string {
	if c := firstChild(n, local); c != nil {
		return strings.Trim(c.Text(), markupcore.PHPTrimSet)
	}
	return ""
}

// rawAttr returns the value of the attribute of n that has the name and no prefix, "" for none.
func rawAttr(n *xmlraw.Node, name string) string {
	for _, a := range n.Attrs {
		if a.Name.Space == "" && a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// readElement reads the <Subtitle> elements below n, with the font attributes of the <Font> elements around them.
func (rd *reader) readElement(n *xmlraw.Node, font map[string]string) error {
	for _, child := range elements(n) {
		var err error
		switch child.Name.Local {
		case "Font":
			err = rd.readElement(child, mergeFont(child, font))
		case "SubtitleList":
			err = rd.readElement(child, font)
		case "Subtitle":
			err = rd.addCue(child, font)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// addCue reads a <Subtitle>. A parse error of the subtitle fails the read in strict mode and skips it in lenient mode.
func (rd *reader) addCue(sub *xmlraw.Node, font map[string]string) error {
	c, ok, err := rd.cue(sub, font)
	var perr *model.ParseError
	switch {
	case errors.As(err, &perr):
		return rd.st.Fail(perr, sub.Line, model.NoBlock, nil)
	case err != nil:
		return err
	case ok:
		rd.cues = append(rd.cues, c)
	}
	return nil
}

func (rd *reader) cue(sub *xmlraw.Node, font map[string]string) (model.Cue, bool, error) {
	start, err := rd.time(rawAttr(sub, "TimeIn"), sub.Line)
	if err != nil {
		return model.Cue{}, false, err
	}
	end, err := rd.time(rawAttr(sub, "TimeOut"), sub.Line)
	if err != nil {
		return model.Cue{}, false, err
	}
	start, end, err = rd.st.OrderedTimes(start, end, sub.Line, model.NoBlock, nil, true)
	if err != nil {
		return model.Cue{}, false, err
	}

	var lines []line
	collectTexts(sub, font, &lines)
	if len(lines) == 0 {
		for d := range sub.Descendants() {
			if d.Name.Local == "Image" {
				return model.Cue{}, false, parseError(sub.Line, "The subtitle at "+rawAttr(sub, "TimeIn")+" is an image. The parser reads only text subtitles.")
			}
		}
		return model.Cue{}, false, nil
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].y < lines[j].y })
	lines = slices.DeleteFunc(lines, func(l line) bool { return l.text == "" })
	if len(lines) == 0 {
		return model.Cue{}, false, nil
	}

	texts := make([][]attr, len(lines))
	cueLines := make([]string, len(lines))
	textData := make([]any, len(lines))
	for i, l := range lines {
		texts[i], cueLines[i], textData[i] = l.attrs, l.text, attrData(l.attrs)
	}
	c := model.Cue{Start: start, End: end, Lines: cueLines}
	c.Position.Alignment = alignmentOf(texts)
	c.Position.Offset = offsetOf(c.Position.Alignment, texts)
	data := model.NewData(
		model.Field{Key: "subtitle", Value: attrData(attributes(sub))},
		model.Field{Key: "texts", Value: textData},
	)
	if err := c.SetFormatData(formatDataKey, data); err != nil {
		return model.Cue{}, false, err
	}
	return c, true, nil
}

// collectTexts collects each <Text> element below n as a line.
func collectTexts(n *xmlraw.Node, font map[string]string, lines *[]line) {
	for _, child := range elements(n) {
		switch child.Name.Local {
		case "Font":
			collectTexts(child, mergeFont(child, font), lines)
		case "Text":
			var runs []markupcore.Run
			collectRuns(child, font, &runs)
			attrs := attributes(child)
			*lines = append(*lines, line{
				text:  strings.Trim(markupcore.StyleRuns(runs, true), markupcore.PHPTrimSet),
				y:     screenY(attrs),
				attrs: attrs,
			})
		}
	}
}

func collectRuns(n *xmlraw.Node, font map[string]string, runs *[]markupcore.Run) {
	for _, child := range n.Children {
		switch child.Kind {
		case xmlraw.TextNode, xmlraw.CDATANode:
			if text := textWhiteSpace.Replace(child.Data); text != "" {
				*runs = append(*runs, runOf(text, font))
			}
		case xmlraw.ElementNode:
			collectElementRuns(child, font, runs)
		case xmlraw.DocumentNode, xmlraw.CommentNode, xmlraw.ProcInstNode, xmlraw.DirectiveNode:
		}
	}
}

func collectElementRuns(n *xmlraw.Node, font map[string]string, runs *[]markupcore.Run) {
	switch n.Name.Local {
	case "Font":
		collectRuns(n, mergeFont(n, font), runs)
	case "Space":
		*runs = append(*runs, runOf(" ", font))
	case "Ruby":
		base, annotation := childText(n, "Rb"), childText(n, "Rt")
		if annotation != "" {
			base += " (" + annotation + ")"
		}
		*runs = append(*runs, runOf(base, font))
	default:
		collectRuns(n, font, runs)
	}
}

// mergeFont returns font with the attributes of the <Font> element n. Names and values are lower case and values are
// trimmed, so Interop Underlined and SMPTE Underline both count.
func mergeFont(n *xmlraw.Node, font map[string]string) map[string]string {
	out := make(map[string]string, len(font)+len(n.Attrs))
	for k, v := range font {
		out[k] = v
	}
	for _, a := range attributes(n) {
		out[markupcore.ASCIILower(a.name)] = markupcore.ASCIILower(strings.Trim(a.value, markupcore.PHPTrimSet))
	}
	return out
}

func runOf(text string, font map[string]string) markupcore.Run {
	run := markupcore.Run{Text: text, Bold: font["weight"] == "bold"}
	if m := colorRegexp.FindStringSubmatch(font["color"]); m != nil && m[1] != "ffffff" {
		run.Color = "#" + m[1]
	}
	switch font["italic"] {
	case "yes", "left", "right":
		run.Italic = true
	}
	underline, ok := font["underlined"]
	if !ok {
		underline = font["underline"]
	}
	run.Underline = underline == "yes"
	return run
}

// textAttr returns the attribute of a <Text> element whose name equals name in lower case, trimmed. Interop writes
// VAlign and SMPTE writes Valign.
func textAttr(attrs []attr, name string) (string, bool) {
	for _, a := range attrs {
		if markupcore.ASCIILower(a.name) == name {
			return strings.Trim(a.value, markupcore.PHPTrimSet), true
		}
	}
	return "", false
}

// screenY returns the height of a line on the screen in percent, from 0 at the top to 100 at the bottom. VPosition
// counts from the edge that VAlign names, or down from the middle for "center", the default.
func screenY(attrs []attr) float64 {
	v, _ := textAttr(attrs, "vposition")
	pos := phpFloat(v)
	align, ok := textAttr(attrs, "valign")
	if !ok {
		align = "center"
	}
	switch markupcore.ASCIILower(align) {
	case "top":
		return pos
	case "bottom":
		return 100 - pos
	default:
		return 50 + pos
	}
}

// phpFloat returns the number at the start of s, 0 for none, as the PHP cast (float) reads it.
func phpFloat(s string) float64 {
	f, _ := strconv.ParseFloat(numberPrefix.FindString(s), 64)
	return f
}

// alignmentOf returns the alignment of the <Text> attributes of one cue, top line first. The third of the screen that
// holds the mean height of the lines gives the row, and HAlign of the top line gives the column.
func alignmentOf(texts [][]attr) model.Alignment {
	y := meanY(texts)
	row := 1
	switch {
	case y < 100.0/3:
		row = 2
	case y > 200.0/3:
		row = 0
	}
	h, _ := textAttr(texts[0], "halign")
	column := 2
	switch markupcore.ASCIILower(h) {
	case "left":
		column = 1
	case "right":
		column = 3
	}
	a, _ := model.AlignmentAt(row, column)
	return a
}

func meanY(texts [][]attr) float64 {
	var sum float64
	for _, t := range texts {
		sum += screenY(t)
	}
	return sum / float64(len(texts))
}

// offsetOf returns the offset of a cue: the anchor edge of its alignment. The top line gives Y of a top row, the bottom
// line gives Y of a bottom row, and the mean height gives Y of a middle row. HPosition counts from the edge that HAlign
// names, so it gives X. It returns nil when the point does not lie in the cell of a. A top or bottom row whose edge line
// lies within 10 % of the screen edge, which is the usual DCP place, has no offset unless X differs from the default.
func offsetOf(a model.Alignment, texts [][]attr) *model.Offset {
	y, x := anchorY(a, texts), anchorX(a, texts[0])
	if math.IsNaN(x) || math.IsNaN(y) {
		return nil
	}
	o := model.Offset{X: min(max(x, 0), 100), Y: min(max(y, 0), 100)}
	if !position.OffsetFits(a, &o) {
		return nil
	}
	nearEdge := a.Row() == 2 && o.Y <= edgeBand || a.Row() == 0 && o.Y >= 100-edgeBand
	if nearEdge && position.Round(o.X) == position.DefaultAnchor(a).X {
		return nil
	}
	return position.ReadOffset(a, o)
}

func anchorY(a model.Alignment, texts [][]attr) float64 {
	switch a.Row() {
	case 2:
		return screenY(texts[0])
	case 0:
		return screenY(texts[len(texts)-1])
	default:
		return meanY(texts)
	}
}

// anchorX returns the horizontal anchor of the top line: HPosition counts from the edge that HAlign names. A line
// without HPosition has the default anchor of the column.
func anchorX(a model.Alignment, top []attr) float64 {
	hp, ok := textAttr(top, "hposition")
	if !ok {
		return position.DefaultAnchor(a).X
	}
	x := phpFloat(hp)
	switch a.Column() {
	case 1:
		return x
	case 3:
		return 100 - x
	default:
		return 50 + x
	}
}

// time reads HH:MM:SS:EE, where EE counts rate units per second, or Interop HH:MM:SS.sss with a decimal fraction.
func (rd *reader) time(value string, lineNumber int) (time.Duration, error) {
	value = strings.Trim(value, markupcore.PHPTrimSet)
	var d time.Duration
	var hours string
	if m := decimalTime.FindStringSubmatch(value); m != nil && rd.interop {
		hours = m[1]
		if h, ok := belowLimit(hours); ok {
			fraction, _ := strconv.ParseFloat("0."+m[4], 64)
			d = timecode.FromSeconds(float64(h)*3600 + float64(atoi(m[2]))*60 + float64(atoi(m[3])) + fraction)
		}
	} else if m := unitsTime.FindStringSubmatch(value); m != nil && atoi(m[4]) < rd.rate {
		hours = m[1]
		if h, ok := belowLimit(hours); ok {
			rate := model.FrameRate{Num: int64(rd.rate), Den: 1}
			d = rate.ClockDuration(model.Timecode{Hours: h, Minutes: atoi(m[2]), Seconds: atoi(m[3]), Frames: atoi(m[4])}).Round(time.Millisecond)
		}
	} else {
		return 0, parseError(lineNumber, `The time "`+value+`" is not a DCP time code at `+timecode.Number(float64(rd.rate))+" units per second.")
	}
	if _, ok := belowLimit(hours); !ok {
		d = timecode.MaxTime
	}
	return parsing.BoundedTime(d, value, lineNumber)
}

// belowLimit returns the hours when they are below the limit of cue times.
func belowLimit(digits string) (int, bool) {
	h, err := strconv.Atoi(digits)
	return h, err == nil && h < timecode.MaxHours
}

func atoi(digits string) int {
	n, _ := strconv.Atoi(digits)
	return n
}
