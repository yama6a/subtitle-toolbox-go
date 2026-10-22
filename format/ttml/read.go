package ttml

import (
	"encoding/xml"
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

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

//nolint:gochecknoglobals // the timing attributes of TTML
var timingAttributes = []string{"begin", "end", "dur"}

var (
	multiplierRegexp = regexp.MustCompile(`[\t\n\v\f\r :]+`)
	frameLabelRegexp = regexp.MustCompile(`^\d{2,}:\d{2}:\d{2}:(\d{2})(?:\.\d+)?$`)
	collapseRegexp   = regexp.MustCompile(`[ \t\n]+`)
	lengthRegexp     = regexp.MustCompile(`^([+-]?\d+(?:\.\d+)?)(%|px|c)$`)
	rootExtentRegexp = regexp.MustCompile(`^[\t\n\v\f\r ]*\d+(?:\.\d+)?px[\t\n\v\f\r ]+(\d+(?:\.\d+)?)px[\t\n\v\f\r ]*$`)
)

type reader struct {
	st      *parsing.State
	ns      string
	root    *xmlraw.Node
	body    *xmlraw.Node
	styles  styles
	agents  map[string]string
	clock   clock
	cues    []model.Cue
	removed []bool
	// paragraph is the index of the current <p> in document order, also of the paragraphs that fail.
	paragraph int
	open      []openParagraph
}

// openParagraph is a paragraph without an end, which lenient mode ends at the next paragraph.
type openParagraph struct {
	cue, line, paragraph int
	lines                []string
}

// interval is the time interval of an element in seconds. hasEnd is false for an element without an end.
type interval struct {
	begin, end float64
	hasEnd     bool
}

// scope holds the values that an element inherits from its containers.
type scope struct {
	interval
	region, textAlign string
	hasRegion         bool
	hasTextAlign      bool
	preserveSpace     bool
	forced            *bool
	// properties holds the style properties of <body> and each <div>, outermost first.
	properties []properties
}

// run is one piece of the content of a paragraph: a line break, the start of a timed span, or text.
type run struct {
	lineBreak bool
	timed     bool
	span      interval
	text      string
	style     style
	agent     string
}

// Read reads TTML, IMSC and DFXP. It expects UTF-8, as stbox.Parse passes it. The reader ignores the encoding of
// the XML declaration.
func Read(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.TTML, err)
	}
	st := parsing.NewState(model.TTML, common)
	content, ok, err := st.Prepare(content, true)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}
	rd := &reader{st: st}
	if err := rd.read(string(content), s); err != nil {
		return nil, nil, err
	}
	return s, st.Warnings(), nil
}

func (r *reader) parseError(line int, msg string) *model.ParseError {
	return &model.ParseError{Format: model.TTML, Line: line, Msg: msg}
}

func (r *reader) read(content string, s *model.Subtitle) error {
	doc, err := r.load(content)
	if err != nil {
		return err
	}
	r.root, _ = doc.Root()
	r.ns = r.root.NamespaceURI()
	if localName(r.root) != "tt" || !slices.Contains([]string{nsTTML, nsDFXP, nsDFXP200604, ""}, r.ns) {
		return r.parseError(model.NoLine, "The root element is not a TTML <tt> element.")
	}
	if r.clock, err = r.readClock(); err != nil {
		return err
	}
	head := r.firstChild(r.root, "head")
	r.body = r.firstChild(r.root, "body")
	r.styles = newStyles(r.ns, head)
	r.agents = r.readAgents(head)
	if err := r.readFileData(s, head); err != nil {
		return err
	}
	if r.body != nil {
		if err := r.readContainer(r.body, scope{}, nil); err != nil {
			return err
		}
	}
	return r.addCues(s)
}

// readClock reads the timing parameters of the root. See https://www.w3.org/TR/ttml2/#parameter-attribute-frameRate.
func (r *reader) readClock() (clock, error) {
	frameRate, hasFrameRate := r.parameter("frameRate")
	guess, hundredths := 30.0, false
	if !hasFrameRate {
		var err error
		if guess, hundredths, err = r.guessFrameRate(); err != nil {
			return clock{}, err
		}
	}
	smpteFrameRate := guess
	if hasFrameRate {
		smpteFrameRate = phpFloat(frameRate)
	}
	if smpteFrameRate == 0 {
		smpteFrameRate = 30
	}
	c := clock{
		frameRate:      smpteFrameRate * r.frameRateMultiplier(),
		smpteFrameRate: smpteFrameRate,
		dropMode:       r.trimmedParameter("dropMode", "nonDrop"),
		smpteTimeBase:  r.trimmedParameter("timeBase", "") == "smpte",
		subFrameRate:   1,
		tickRate:       1,
		hundredths:     hundredths,
	}
	if hasFrameRate {
		c.tickRate = c.frameRate
	}
	if v, ok := r.parameter("tickRate"); ok && phpFloat(v) > 0 {
		c.tickRate = phpFloat(v)
	}
	if v, ok := r.parameter("subFrameRate"); ok && phpFloat(v) != 0 {
		c.subFrameRate = phpFloat(v)
	}
	return c, nil
}

func (r *reader) parameter(local string) (string, bool) {
	return familyAttr(r.root, parameterNamespaces, "ttp", local)
}

// guessFrameRate returns the default of 30 fps, or 50 or 60 fps in lenient mode when a frame label in the file is
// 30 or more. No frame rate fits a label of 60 or more, so lenient mode then reads the last field as hundredths of
// a second.
func (r *reader) guessFrameRate() (float64, bool, error) {
	label, expression, line := r.highestFrameLabel()
	if label == 0 {
		return 30, false, nil
	}
	msg := "The frame label " + strconv.FormatInt(label, 10) + " in \"" + expression +
		"\" is not below the default frame rate of 30, and the file has no ttp:frameRate."
	if !r.st.Lenient() {
		return 0, false, r.parseError(line, msg)
	}
	if label >= 60 {
		r.st.Warn(msg+" The parser read the last field as hundredths of a second.", line, model.NoBlock, nil, model.Repaired)
		return 30, true, nil
	}
	guess := 60
	if label < 50 {
		guess = 50
	}
	r.st.Warn(msg+" The parser read the frames at "+strconv.Itoa(guess)+" fps.", line, model.NoBlock, nil, model.Repaired)
	return float64(guess), false, nil
}

// readContainer reads <body> or a <div> and the paragraphs and divs in it.
// See https://www.w3.org/TR/ttml2/#timing-time-intervals.
func (r *reader) readContainer(container *xmlraw.Node, parent scope, divAttributes *model.Data) error {
	iv, err := r.interval(container, parent.interval)
	if err != nil {
		if !r.st.Lenient() {
			return err
		}
		iv = parent.interval
		r.st.Warn(err.Msg+" The parser ignored the timing of the "+localName(container)+".", container.Line, model.NoBlock, nil, model.Repaired)
	}
	sc := r.containerScope(container, parent, iv)
	if isElement(container, r.ns, "div") {
		divAttributes = mergeData(divAttributes, readAttributes(container, "begin", "end", "dur", "xml:id", "region"))
	}
	for _, child := range container.Children {
		var cerr error
		switch {
		case isElement(child, r.ns, "div"):
			cerr = r.readContainer(child, sc, divAttributes)
		case isElement(child, r.ns, "p"):
			cerr = r.readParagraphOf(child, sc, divAttributes)
		}
		if cerr != nil {
			return cerr
		}
	}
	return nil
}

func (r *reader) readParagraphOf(p *xmlraw.Node, sc scope, divAttributes *model.Data) error {
	c, open, err := r.readParagraph(p, sc)
	if err != nil {
		r.paragraph++
		return r.st.Fail(err, p.Line, r.paragraph-1, xmlLines(p))
	}
	r.paragraph++
	if divAttributes.Len() > 0 {
		data, _ := c.FormatData(formatDataKey)
		data = data.Clone()
		if data == nil {
			data = model.NewData()
		}
		data.Set("div", divAttributes.Clone())
		if err := c.SetFormatData(formatDataKey, data); err != nil {
			return fmt.Errorf("%s cue data: %w", model.TTML, err)
		}
	}
	r.endOpenParagraphs(c.Start)
	if open {
		r.open = append(r.open, openParagraph{cue: len(r.cues), line: p.Line, paragraph: r.paragraph - 1, lines: xmlLines(p)})
	}
	r.cues = append(r.cues, c)
	r.removed = append(r.removed, false)
	return nil
}

// readParagraph returns the cue of a paragraph, and true for a paragraph without an end, which lenient mode keeps
// open.
func (r *reader) readParagraph(p *xmlraw.Node, sc scope) (model.Cue, bool, *model.ParseError) {
	iv, err := r.paragraphInterval(p, sc)
	if err != nil {
		return model.Cue{}, false, err
	}
	region, hasRegion := sc.region, sc.hasRegion
	if v, ok := plainAttr(p, "region"); ok {
		region, hasRegion = v, true
	}
	runs := r.paragraphRuns(p, sc, region, hasRegion, iv)
	iv, open, err := r.paragraphEnd(p, iv, runs)
	if err != nil {
		return model.Cue{}, false, err
	}
	runs = slices.DeleteFunc(runs, func(rn run) bool {
		return rn.timed && (rn.span.begin <= iv.begin || !open && rn.span.begin >= iv.end)
	})
	start, end, err := r.orderedTimes(p, iv)
	if err != nil {
		return model.Cue{}, false, err
	}
	c := model.Cue{Start: start, End: end, Lines: r.runsToLines(runs), ID: xmlAttr(p, "id")}
	if err := r.setCueData(&c, p, sc, region, hasRegion); err != nil {
		return model.Cue{}, false, err
	}
	c.Forced = r.paragraphForced(p, sc, region, hasRegion)
	return c, open, nil
}

// endOpenParagraphs ends each paragraph without an end that begins before begin.
func (r *reader) endOpenParagraphs(begin time.Duration) {
	r.open = slices.DeleteFunc(r.open, func(open openParagraph) bool {
		c := &r.cues[open.cue]
		if c.Start >= begin {
			return false
		}
		c.End = begin
		r.st.Warn("The paragraph that begins at "+timecode.Seconds(c.Start)+"s has no end time. The parser ended it at the next paragraph at "+
			timecode.Seconds(begin)+"s.", open.line, open.paragraph, open.lines, model.Repaired)
		return true
	})
}

// absoluteInterval reads the times of a paragraph without the offset of its parent. Some files give the times of a
// paragraph in a timed div as absolute times, so the paragraph begins after the div ends.
func (r *reader) absoluteInterval(p *xmlraw.Node, begin, parentEnd float64) (interval, *model.ParseError) {
	msg := "The paragraph begins at " + timecode.Number(round3(begin)) + "s, but its parent ends at " + timecode.Number(round3(parentEnd)) + "s."
	absolute := interval{}
	if _, ok := plainAttr(p, "begin"); ok {
		var err *model.ParseError
		if absolute, err = r.interval(p, interval{}); err != nil {
			return interval{}, err
		}
	}
	if !r.st.Lenient() || !absolute.hasEnd || absolute.end <= absolute.begin {
		return interval{}, r.parseError(p.Line, msg)
	}
	r.st.Warn(msg+" The parser read its times as absolute: "+timecode.Number(round3(absolute.begin))+"s to "+
		timecode.Number(round3(absolute.end))+"s.", p.Line, r.paragraph, xmlLines(p), model.Repaired)
	return absolute, nil
}

// interval returns the interval of n inside the interval of its parent. The end is the earliest of the parent end,
// the parent begin plus the own end, and the own begin plus the duration.
func (r *reader) interval(n *xmlraw.Node, parent interval) (interval, *model.ParseError) {
	iv := interval{begin: parent.begin, end: parent.end, hasEnd: parent.hasEnd}
	if _, ok := plainAttr(n, "begin"); ok {
		var err *model.ParseError
		if iv.begin, err = r.offsetTime(parent.begin, n, "begin"); err != nil {
			return interval{}, err
		}
	}
	for _, attr := range []struct {
		name string
		base float64
	}{{"end", parent.begin}, {"dur", iv.begin}} {
		v, _ := plainAttr(n, attr.name)
		if strings.Trim(v, markupcore.PHPTrimSet) == "" {
			continue
		}
		t, err := r.offsetTime(attr.base, n, attr.name)
		if err != nil {
			return interval{}, err
		}
		if !iv.hasEnd || t < iv.end {
			iv.end, iv.hasEnd = t, true
		}
	}
	return iv, nil
}

// offsetTime returns base plus the time of the attribute. Lenient mode reads the forms of looseSeconds too.
func (r *reader) offsetTime(base float64, n *xmlraw.Node, attr string) (float64, *model.ParseError) {
	v, _ := plainAttr(n, attr)
	expression := strings.Trim(v, markupcore.PHPTrimSet)
	secs, err := r.boundedExpression(expression)
	if err == nil {
		return r.bounded(base+secs, expression, n.Line)
	}
	if !r.st.Lenient() {
		return 0, err
	}
	loose, ok := looseSeconds(expression)
	if !ok {
		return 0, err
	}
	t, berr := r.bounded(base+loose, expression, n.Line)
	if berr != nil {
		return 0, berr
	}
	block := model.NoBlock
	if r.isInParagraph(n) {
		block = r.paragraph
	}
	r.st.Warn("The time expression \""+expression+"\" is not valid. The parser read it as "+timecode.Number(loose)+"s.",
		n.Line, block, nil, model.Repaired)
	return t, nil
}

func (r *reader) boundedExpression(expression string) (float64, *model.ParseError) {
	secs, ok := r.clock.seconds(expression)
	if !ok {
		return 0, r.parseError(model.NoLine, "The time expression \""+expression+"\" is not valid.")
	}
	return r.bounded(secs, expression, model.NoLine)
}

// bounded returns secs when the time rounds to whole milliseconds below 100000 hours in size.
func (r *reader) bounded(secs float64, text string, line int) (float64, *model.ParseError) {
	if !(math.Abs(secs) < timecode.MaxTime.Seconds()) || toDuration(math.Abs(secs)) >= timecode.MaxTime {
		return 0, r.parseError(line, "The time \""+text+"\" is not below "+strconv.Itoa(timecode.MaxHours)+" hours.")
	}
	return secs, nil
}

func (r *reader) isInParagraph(n *xmlraw.Node) bool {
	for ; n != nil && n.Kind == xmlraw.ElementNode; n = n.Parent {
		if isElement(n, r.ns, "p") {
			return true
		}
	}
	return false
}

// collectRuns appends the runs of the content of n. A <span> with a time adds a timed span before its content.
func (r *reader) collectRuns(n *xmlraw.Node, st style, agent string, preserveSpace bool, iv interval, runs *[]run) {
	for _, child := range n.Children {
		switch {
		case child.Kind == xmlraw.TextNode:
			addTextRuns(runs, child.Data, st, agent, preserveSpace)
		case isElement(child, r.ns, "br"):
			*runs = append(*runs, run{lineBreak: true})
		case isElement(child, r.ns, "span"):
			spanInterval := iv
			if slices.ContainsFunc(timingAttributes, func(name string) bool { _, ok := plainAttr(child, name); return ok }) {
				// A bad span time drops only the word timestamp, so the text of the cue stays.
				if timed, err := r.interval(child, iv); err == nil {
					spanInterval = timed
					*runs = append(*runs, run{timed: true, span: timed})
				}
			}
			spanAgent := r.agentName(child)
			if spanAgent == "" {
				spanAgent = agent
			}
			r.collectRuns(child, st.apply(r.styles.ownProperties(child)), spanAgent, preservesSpace(child, preserveSpace), spanInterval, runs)
		}
	}
}

// timedSpansInterval returns the union of the timed spans, for a paragraph without an end of its own.
func timedSpansInterval(runs []run) (interval, bool) {
	var iv interval
	first := true
	for _, rn := range runs {
		if !rn.timed {
			continue
		}
		if first || rn.span.begin < iv.begin {
			iv.begin = rn.span.begin
		}
		first = false
		if rn.span.hasEnd && (!iv.hasEnd || rn.span.end > iv.end) {
			iv.end, iv.hasEnd = rn.span.end, true
		}
	}
	return iv, iv.hasEnd
}

// addTextRuns appends the runs of a text node. With xml:space="preserve", each LF becomes a line break.
// See https://www.w3.org/TR/ttml2/#content-attribute-space.
func addTextRuns(runs *[]run, text string, st style, agent string, preserveSpace bool) {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	parts := []string{text}
	if preserveSpace {
		parts = strings.Split(text, "\n")
	}
	for i, part := range parts {
		if i > 0 {
			*runs = append(*runs, run{lineBreak: true})
		}
		*runs = append(*runs, run{text: collapseRegexp.ReplaceAllLiteralString(part, " "), style: st, agent: agent})
	}
}

// runsToLines turns the runs into lines of core markup. A run that starts with a space loses it after text that
// ends with a space and at the start of a line, and the trailing spaces of a line go.
func (r *reader) runsToLines(runs []run) []string {
	var lines []string
	var line []run
	openAgent := ""
	for _, rn := range append(slices.Clip(runs), run{lineBreak: true}) {
		if !rn.lineBreak {
			line = append(line, rn)
			continue
		}
		endsWithSpace := true
		for i := range line {
			if endsWithSpace && strings.HasPrefix(line[i].text, " ") {
				line[i].text = line[i].text[1:]
			}
			if line[i].text != "" {
				endsWithSpace = strings.HasSuffix(line[i].text, " ")
			}
		}
		for i := len(line) - 1; i >= 0; i-- {
			line[i].text = strings.TrimRight(line[i].text, " ")
			if line[i].text != "" {
				break
			}
		}
		lines = append(lines, runsToMarkup(line, &openAgent))
		line = nil
	}
	return lines
}

// runsToMarkup writes the runs of one line. A speaker tag stays open across lines, as in WebVTT.
func runsToMarkup(runs []run, openAgent *string) string {
	var b strings.Builder
	var styled []markupcore.Run
	for _, rn := range runs {
		if rn.timed {
			b.WriteString(markupcore.StyleRuns(styled, false) + "<" + markupcore.CoreTimestamp(toDuration(rn.span.begin)) + ">")
			styled = nil
			continue
		}
		if strings.Trim(rn.text, markupcore.PHPTrimSet) != "" && rn.agent != *openAgent {
			b.WriteString(markupcore.StyleRuns(styled, false))
			if *openAgent != "" {
				b.WriteString("</v>")
			}
			if rn.agent != "" {
				b.WriteString(markupcore.VoiceTag(rn.agent, ""))
			}
			styled = nil
			*openAgent = rn.agent
		}
		styled = append(styled, rn.style.run(rn.text))
	}
	return b.String() + markupcore.StyleRuns(styled, false)
}

// alignment returns the alignment of a region and a text alignment. Start and end depend on the text direction,
// so they give no alignment. See https://www.w3.org/TR/ttml2/#style-attribute-displayAlign.
func (r *reader) alignment(region string, hasRegion bool, textAlign string, hasTextAlign bool) (model.Alignment, bool) {
	if _, ok := r.styles.regions[region]; hasRegion && !ok || !hasRegion && len(r.styles.regions) > 0 {
		return model.BottomCenter, false
	}
	props := r.styles.regionProperties(region, hasRegion)
	if !hasTextAlign {
		textAlign = propOr(props, "textAlign", "start")
	}
	column := map[string]int{"left": 1, "center": 2, "right": 3}[strings.Trim(textAlign, markupcore.PHPTrimSet)]
	row, ok := r.row(props)
	if column == 0 || !ok {
		return model.BottomCenter, false
	}
	return model.AlignmentAt(row, column)
}

func propOr(props properties, name, fallback string) string {
	if v, ok := props[name]; ok {
		return v
	}
	return fallback
}

// verticalLength returns the second length of an origin or extent value as a fraction of the root container height.
func (r *reader) verticalLength(value string, auto float64) (float64, bool) {
	parts := splitSpace(value)
	if len(parts) == 1 && parts[0] == "auto" {
		return auto, true
	}
	if len(parts) != 2 {
		return 0, false
	}
	m := lengthRegexp.FindStringSubmatch(parts[1])
	if m == nil {
		return 0, false
	}
	length := phpFloat(m[1])
	switch m[2] {
	case "%":
		return length / 100, true
	case "c":
		return r.cellLength(length)
	default:
		return r.pixelLength(length)
	}
}

// agentName returns the name of the first agent of the ttm:agent attribute of n, or the ID without a named agent.
func (r *reader) agentName(n *xmlraw.Node) string {
	v, _ := familyAttr(n, metadataNamespaces, "ttm", "agent")
	ids := splitSpace(v)
	if len(ids) == 0 {
		return ""
	}
	if name, ok := r.agents[ids[0]]; ok {
		return name
	}
	return ids[0]
}

// readAgents maps the xml:id of each agent in head to the trimmed text of its first name with text. A later agent
// of the same ID wins. See https://www.w3.org/TR/ttml2/#metadata-vocabulary-agent.
func (r *reader) readAgents(head *xmlraw.Node) map[string]string {
	agents := map[string]string{}
	if head == nil {
		return agents
	}
	for n := range head.Descendants() {
		if !isMetadataElement(n, "agent") {
			continue
		}
		for _, c := range n.Children {
			if isMetadataElement(c, "name") {
				if name := strings.Trim(c.Text(), markupcore.PHPTrimSet); name != "" {
					agents[xmlAttr(n, "id")] = name
					break
				}
			}
		}
	}
	return agents
}

// forcedDisplay reads itts:forcedDisplay of n, else of the styles that n references. The last reference that sets
// it wins. See https://www.w3.org/TR/ttml-imsc1.1/#forceddisplay.
func (r *reader) forcedDisplay(n *xmlraw.Node) (bool, bool) {
	return r.forcedDisplayAt(n, 0)
}

func (r *reader) forcedDisplayAt(n *xmlraw.Node, depth int) (bool, bool) {
	for _, a := range attributes(n) {
		if n.AttrNamespaceURI(a) == nsIMSCStyle && a.Name.Local == "forcedDisplay" || undeclaredName(n, a) == "itts:forcedDisplay" {
			return strings.Trim(a.Value, markupcore.PHPTrimSet) == "true", true
		}
	}
	forced, found := false, false
	if depth < maxStyleDepth {
		for _, id := range styleIDs(n) {
			if ref, ok := r.styles.styles[id]; ok {
				if f, ok := r.forcedDisplayAt(ref, depth+1); ok {
					forced, found = f, true
				}
			}
		}
	}
	return forced, found
}

// regionForcedDisplay reads itts:forcedDisplay of a region, else of its first nested <style> that sets it.
func (r *reader) regionForcedDisplay(id string, ok bool) (bool, bool) {
	region, found := r.styles.regions[id]
	if !ok || !found {
		return false, false
	}
	if forced, ok := r.forcedDisplay(region); ok {
		return forced, true
	}
	for _, c := range region.Children {
		if isElement(c, r.ns, "style") {
			if forced, ok := r.forcedDisplay(c); ok {
				return forced, true
			}
		}
	}
	return false, false
}

// hasForcedSpan reports whether a span of n is forced, with inheritance inside the spans.
func (r *reader) hasForcedSpan(n *xmlraw.Node, inherited bool) bool {
	for _, c := range n.Children {
		if !isElement(c, r.ns, "span") {
			continue
		}
		forced, ok := r.forcedDisplay(c)
		if !ok {
			forced = inherited
		}
		if forced || r.hasForcedSpan(c, forced) {
			return true
		}
	}
	return false
}

// readNamespaces returns the xmlns declarations of the root, sorted by prefix, without xml.
func (r *reader) readNamespaces() *model.Data {
	var prefixes []string
	uris := map[string]string{}
	for _, a := range r.root.Attrs {
		prefix, ok := declaredPrefix(a)
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

func isNameStart(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
}

func (r *reader) firstChild(n *xmlraw.Node, local string) *xmlraw.Node {
	for _, c := range n.Children {
		if isElement(c, r.ns, local) {
			return c
		}
	}
	return nil
}

// readAttributes returns the attributes of n by name, without the names in skip and without ttm:agent in a
// metadata namespace. An attribute in the XML namespace is named xml: plus its local name, whatever its prefix.
func readAttributes(n *xmlraw.Node, skip ...string) *model.Data {
	out := model.NewData()
	for _, a := range attributes(n) {
		uri := n.AttrNamespaceURI(a)
		if slices.Contains(metadataNamespaces, uri) && a.Name.Local == "agent" {
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
		title := strings.Trim(n.Text(), markupcore.PHPTrimSet)
		parent := n.Parent
		if i := slices.Index(parent.Children, n); i > 0 {
			if prev := parent.Children[i-1]; prev.Kind == xmlraw.TextNode && strings.Trim(prev.Data, markupcore.PHPTrimSet) == "" {
				parent.RemoveChild(prev)
			}
		}
		parent.RemoveChild(n)
		return title, title != ""
	}
	return "", false
}

func preservesSpace(n *xmlraw.Node, inherited bool) bool {
	space := xmlAttr(n, "space")
	if space == "" {
		return inherited
	}
	return space == "preserve"
}

func serialize(n *xmlraw.Node) string {
	var b strings.Builder
	_ = xmlraw.Serialize(&b, n) // a strings.Builder never fails
	return b.String()
}

// xmlLines returns the lines of the serialised element, trimmed, without the empty ones, for warnings.
func xmlLines(n *xmlraw.Node) []string {
	var lines []string
	for line := range strings.SplitSeq(serialize(n), "\n") {
		if line = strings.Trim(line, markupcore.PHPTrimSet); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// phpArray returns an empty list for an empty object, as PHP stores both as an empty array, so the format data
// writes as the PHP library writes it.
func phpArray(d *model.Data) any {
	if d.Len() == 0 {
		return []any{}
	}
	return d
}

// toDuration rounds seconds to whole nanoseconds, then to whole milliseconds, half away from zero.
func toDuration(secs float64) time.Duration {
	return time.Duration(math.Round(secs * 1e9)).Round(time.Millisecond)
}

func round3(f float64) float64 {
	return math.Round(f*1000) / 1000
}

// readFileData sets the metadata and the file data. It removes the title from head.
func (r *reader) readFileData(s *model.Subtitle, head *xmlraw.Node) error {
	if lang := xmlAttr(r.root, "lang"); lang != "" {
		s.SetMetadata(model.MetadataLanguage, lang)
	}
	data := model.NewData(
		model.Field{Key: "namespace", Value: r.ns},
		model.Field{Key: "namespaces", Value: phpArray(r.readNamespaces())},
		model.Field{Key: "attributes", Value: phpArray(readAttributes(r.root, "xml:lang"))},
	)
	if head != nil {
		if title, ok := removeTitle(head); ok {
			s.SetMetadata(model.MetadataTitle, title)
		}
		data.Set("head", serialize(head))
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

// addCues fails the paragraphs that are still open and adds the other cues to s.
func (r *reader) addCues(s *model.Subtitle) error {
	for _, open := range r.open {
		r.removed[open.cue] = true
		msg := "The paragraph that begins at " + timecode.Seconds(r.cues[open.cue].Start) + "s has no end time."
		if err := r.st.Fail(r.parseError(open.line, msg), open.line, open.paragraph, open.lines); err != nil {
			return err
		}
	}
	var cues []model.Cue
	for i, c := range r.cues {
		if !r.removed[i] {
			cues = append(cues, c)
		}
	}
	s.AddCues(cues...)
	return nil
}

// frameRateMultiplier returns a divided by b of ttp:frameRateMultiplier="a b", else 1.
func (r *reader) frameRateMultiplier() float64 {
	v, ok := r.parameter("frameRateMultiplier")
	if !ok {
		return 1
	}
	parts := multiplierRegexp.Split(strings.Trim(v, markupcore.PHPTrimSet), -1)
	if len(parts) != 2 || phpFloat(parts[0]) <= 0 || phpFloat(parts[1]) <= 0 {
		return 1
	}
	return phpFloat(parts[0]) / phpFloat(parts[1])
}

func (r *reader) trimmedParameter(local, fallback string) string {
	if v, ok := r.parameter(local); ok {
		return strings.Trim(v, markupcore.PHPTrimSet)
	}
	return fallback
}

// highestFrameLabel returns the highest frame label of 30 or more in the begin, end and dur attributes of the
// document, with its expression and line. The first one wins a tie. It returns 0 without one.
func (r *reader) highestFrameLabel() (label int64, expression string, line int) {
	for n := range r.root.Parent.Descendants() {
		for _, a := range attributes(n) {
			if a.Name.Space != "" || !slices.Contains(timingAttributes, a.Name.Local) {
				continue
			}
			v := strings.Trim(a.Value, markupcore.PHPTrimSet)
			if m := frameLabelRegexp.FindStringSubmatch(v); m != nil && phpInt(m[1]) >= 30 && phpInt(m[1]) > label {
				label, expression, line = phpInt(m[1]), v, n.Line
			}
		}
	}
	return label, expression, line
}

// containerScope returns the scope that container gives its children.
func (r *reader) containerScope(container *xmlraw.Node, parent scope, iv interval) scope {
	sc := parent
	sc.interval = iv
	sc.preserveSpace = preservesSpace(container, parent.preserveSpace)
	if region, ok := plainAttr(container, "region"); ok {
		sc.region, sc.hasRegion = region, true
	}
	props := r.styles.ownProperties(container)
	if align, ok := props["textAlign"]; ok {
		sc.textAlign, sc.hasTextAlign = align, true
	}
	if forced, ok := r.forcedDisplay(container); ok {
		sc.forced = &forced
	}
	if len(props) > 0 {
		sc.properties = append(slices.Clip(parent.properties), props)
	}
	return sc
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

// paragraphRuns returns the runs of a paragraph with the styles of its region, its containers and its own.
func (r *reader) paragraphRuns(p *xmlraw.Node, sc scope, region string, hasRegion bool, iv interval) []run {
	st := style{}.apply(r.styles.regionProperties(region, hasRegion))
	for _, props := range sc.properties {
		st = st.apply(props)
	}
	st = st.apply(r.styles.ownProperties(p))
	var runs []run
	r.collectRuns(p, st, r.agentName(p), preservesSpace(p, sc.preserveSpace), iv, &runs)
	return runs
}

// paragraphEnd returns the interval of a paragraph without an end: the union of its timed spans, or in lenient
// mode its begin, with true for a paragraph that stays open.
func (r *reader) paragraphEnd(p *xmlraw.Node, iv interval, runs []run) (interval, bool, *model.ParseError) {
	if iv.hasEnd {
		return iv, false, nil
	}
	if spans, ok := timedSpansInterval(runs); ok {
		return spans, false, nil
	}
	if !r.st.Lenient() {
		return interval{}, false, r.parseError(p.Line, "The paragraph that begins at "+timecode.Number(iv.begin)+"s has no end time.")
	}
	return interval{begin: iv.begin, end: iv.begin, hasEnd: true}, true, nil
}

// setCueData sets the cue data and the alignment of the cue of paragraph p.
func (r *reader) setCueData(c *model.Cue, p *xmlraw.Node, sc scope, region string, hasRegion bool) *model.ParseError {
	attrs := readAttributes(p, "begin", "end", "dur", "xml:id")
	if _, ok := plainAttr(p, "region"); !ok && sc.hasRegion {
		attrs.Set("region", sc.region)
	}
	textAlign, hasTextAlign := sc.textAlign, sc.hasTextAlign
	if v, ok := r.styles.ownProperties(p)["textAlign"]; ok {
		textAlign, hasTextAlign = v, true
	}
	data := model.NewData()
	if attrs.Len() > 0 {
		data.Set("attributes", attrs)
	}
	if a, ok := r.alignment(region, hasRegion, textAlign, hasTextAlign); ok {
		c.Alignment = a
		data.Set("alignment", int64(a.Numpad()))
	}
	if data.Len() == 0 {
		return nil
	}
	if err := c.SetFormatData(formatDataKey, data); err != nil {
		return r.parseError(p.Line, err.Error())
	}
	return nil
}

// row returns the grid row of the anchor of a region: below 1/3 of the height the top row, above 2/3 the bottom row.
func (r *reader) row(props properties) (int, bool) {
	origin, originOK := r.verticalLength(propOr(props, "origin", "auto"), 0)
	extent, extentOK := r.verticalLength(propOr(props, "extent", "auto"), 1)
	if !originOK || !extentOK {
		return 0, false
	}
	var anchor float64
	switch strings.Trim(propOr(props, "displayAlign", "before"), markupcore.PHPTrimSet) {
	case "before":
		anchor = origin
	case "center":
		anchor = origin + extent/2
	case "after":
		anchor = origin + extent
	default:
		return 0, false
	}
	switch {
	case anchor < 1.0/3:
		return 2, true
	case anchor > 2.0/3:
		return 0, true
	default:
		return 1, true
	}
}

// cellLength returns a length in cells as a fraction of the height, by the rows of ttp:cellResolution.
func (r *reader) cellLength(length float64) (float64, bool) {
	v, ok := r.parameter("cellResolution")
	if !ok {
		v = "32 15"
	}
	resolution := spaceRegexp.Split(strings.Trim(v, markupcore.PHPTrimSet), -1)
	if len(resolution) != 2 || phpInt(resolution[1]) <= 0 {
		return 0, false
	}
	return length / float64(phpInt(resolution[1])), true
}

// pixelLength returns a length in pixels as a fraction of the height of tts:extent on <tt>, else on <body>.
func (r *reader) pixelLength(length float64) (float64, bool) {
	extent, ok := stylingAttributes(r.root)["extent"]
	if !ok && r.body != nil {
		extent, ok = stylingAttributes(r.body)["extent"]
	}
	m := rootExtentRegexp.FindStringSubmatch(extent)
	if !ok || m == nil || phpFloat(m[1]) <= 0 {
		return 0, false
	}
	return length / phpFloat(m[1]), true
}

// declaredPrefix returns the prefix that the xmlns declaration a declares, "" for the default namespace. A prefix
// that is no NCName, such as "0", and the prefix xml give false.
func declaredPrefix(a xml.Attr) (string, bool) {
	switch {
	case a.Name.Space == "" && a.Name.Local == "xmlns":
		return "", true
	case a.Name.Space == "xmlns" && a.Name.Local != "xml" && isNameStart(a.Name.Local[0]):
		return a.Name.Local, true
	default:
		return "", false
	}
}

// paragraphInterval returns the interval of a paragraph. A paragraph that begins at or after the end of its parent
// reads again without the offset of the parent in lenient mode.
func (r *reader) paragraphInterval(p *xmlraw.Node, sc scope) (interval, *model.ParseError) {
	iv, err := r.interval(p, sc.interval)
	if err != nil || !sc.hasEnd || iv.begin < sc.end {
		return iv, err
	}
	return r.absoluteInterval(p, iv.begin, sc.end)
}

// orderedTimes returns the cue times of a paragraph. An end before the begin fails, or lenient mode swaps them.
func (r *reader) orderedTimes(p *xmlraw.Node, iv interval) (time.Duration, time.Duration, *model.ParseError) {
	start, end := toDuration(iv.begin), toDuration(iv.end)
	if end >= start {
		return start, end, nil
	}
	start, end, err := r.st.OrderedTimes(start, end, p.Line, r.paragraph, xmlLines(p), true)
	var perr *model.ParseError
	if errors.As(err, &perr) {
		return 0, 0, perr
	}
	return start, end, nil
}

// paragraphForced reports whether the cue of a paragraph is forced: by the paragraph, else its nearest container,
// else its region, or by one of its spans.
func (r *reader) paragraphForced(p *xmlraw.Node, sc scope, region string, hasRegion bool) bool {
	forced, ok := r.forcedDisplay(p)
	if !ok && sc.forced != nil {
		forced, ok = *sc.forced, true
	}
	if !ok {
		forced, _ = r.regionForcedDisplay(region, hasRegion)
	}
	return forced || r.hasForcedSpan(p, forced)
}
