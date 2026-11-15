package ttml

import (
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/internal/ttmldoc"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

//nolint:gochecknoglobals // the timing attributes of TTML
var timingAttributes = []string{"begin", "end", "dur"}

var (
	spaceRegexp      = regexp.MustCompile(`[\t\n\v\f\r ]+`)
	frameLabelRegexp = regexp.MustCompile(`^\d{2,}:\d{2}:\d{2}:(\d{2})(?:\.\d+)?$`)
	collapseRegexp   = regexp.MustCompile(`[ \t\n]+`)
	lengthRegexp     = regexp.MustCompile(`^([+-]?\d+(?:\.\d+)?)(%|px|c)$`)
	rootExtentRegexp = regexp.MustCompile(`^[\t\n\v\f\r ]*(\d+(?:\.\d+)?)px[\t\n\v\f\r ]+(\d+(?:\.\d+)?)px[\t\n\v\f\r ]*$`)
)

type reader struct {
	st      *parsing.State
	ns      string
	root    *xmlraw.Node
	body    *xmlraw.Node
	styles  ttmldoc.Sheet
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
	properties []ttmldoc.Properties
}

// run is one piece of the content of a paragraph: a line break, the start of a timed span, or text.
type run struct {
	lineBreak bool
	timed     bool
	span      interval
	text      string
	style     ttmldoc.Style
	agent     string
	agentEnd  bool
}

// Read reads TTML, IMSC and DFXP. It expects UTF-8, as stbox.Parse passes it. The reader ignores the encoding of
// the XML declaration.
func Read(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	st, content, ok, err := parsing.ReadText(r, model.TTML, common, true)
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
	if ttmldoc.LocalName(r.root) != "tt" || !slices.Contains([]string{ttmldoc.NSTTML, ttmldoc.NSDFXP, ttmldoc.NSDFXP200604, ""}, r.ns) {
		return r.parseError(model.NoLine, "The root element is not a TTML <tt> element.")
	}
	if r.clock, err = r.readClock(); err != nil {
		return err
	}
	head := r.firstChild(r.root, "head")
	r.body = r.firstChild(r.root, "body")
	r.styles = ttmldoc.NewSheet(r.ns, head)
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
		smpteFrameRate = ttmldoc.LeadingFloat(frameRate)
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
	if v, ok := r.parameter("tickRate"); ok && ttmldoc.LeadingFloat(v) > 0 {
		c.tickRate = ttmldoc.LeadingFloat(v)
	}
	if v, ok := r.parameter("subFrameRate"); ok && ttmldoc.LeadingFloat(v) != 0 {
		c.subFrameRate = ttmldoc.LeadingFloat(v)
	}
	return c, nil
}

// frameRateMultiplier returns a divided by b of ttp:frameRateMultiplier="a b", else 1.
func (r *reader) frameRateMultiplier() float64 {
	v, ok := r.parameter("frameRateMultiplier")
	if !ok {
		return 1
	}
	a, b := ttmldoc.Multiplier(v)
	return a / b
}

func (r *reader) parameter(local string) (string, bool) {
	return familyAttr(r.root, ttmldoc.ParameterNamespaces, "ttp", local)
}

func (r *reader) trimmedParameter(local, fallback string) string {
	if v, ok := r.parameter(local); ok {
		return strings.Trim(v, markupcore.TrimSet)
	}
	return fallback
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

// highestFrameLabel returns the highest frame label of 30 or more in the begin, end and dur attributes of the
// document, with its expression and line. The first one wins a tie. It returns 0 without one.
func (r *reader) highestFrameLabel() (label int64, expression string, line int) {
	for n := range r.root.Parent.Descendants() {
		for _, a := range ttmldoc.Attributes(n) {
			if a.Name.Space != "" || !slices.Contains(timingAttributes, a.Name.Local) {
				continue
			}
			v := strings.Trim(a.Value, markupcore.TrimSet)
			if m := frameLabelRegexp.FindStringSubmatch(v); m != nil && leadingInt(m[1]) >= 30 && leadingInt(m[1]) > label {
				label, expression, line = leadingInt(m[1]), v, n.Line
			}
		}
	}
	return label, expression, line
}

func (r *reader) firstChild(n *xmlraw.Node, local string) *xmlraw.Node {
	for _, c := range n.Children {
		if ttmldoc.IsElement(c, r.ns, local) {
			return c
		}
	}
	return nil
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
				if name := strings.Trim(c.Text(), markupcore.TrimSet); name != "" {
					agents[ttmldoc.XMLAttr(n, "id")] = name
					break
				}
			}
		}
	}
	return agents
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
		r.st.Warn(err.Msg+" The parser ignored the timing of the "+ttmldoc.LocalName(container)+".", container.Line, model.NoBlock, nil, model.Repaired)
	}
	sc := r.containerScope(container, parent, iv)
	if ttmldoc.IsElement(container, r.ns, "div") {
		divAttributes = mergeData(divAttributes, readAttributes(container, "begin", "end", "dur", "xml:id", "region"))
	}
	for _, child := range container.Children {
		var cerr error
		switch {
		case ttmldoc.IsElement(child, r.ns, "div"):
			cerr = r.readContainer(child, sc, divAttributes)
		case ttmldoc.IsElement(child, r.ns, "p"):
			cerr = r.readParagraphOf(child, sc, divAttributes)
		}
		if cerr != nil {
			return cerr
		}
	}
	return nil
}

// containerScope returns the scope that container gives its children.
func (r *reader) containerScope(container *xmlraw.Node, parent scope, iv interval) scope {
	sc := parent
	sc.interval = iv
	sc.preserveSpace = preservesSpace(container, parent.preserveSpace)
	if region, ok := ttmldoc.PlainAttr(container, "region"); ok {
		sc.region, sc.hasRegion = region, true
	}
	props := r.styles.OwnProperties(container)
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
	if v, ok := ttmldoc.PlainAttr(p, "region"); ok {
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
	c := model.Cue{Start: start, End: end, Lines: r.runsToLines(runs), ID: ttmldoc.XMLAttr(p, "id")}
	if err := r.setCueData(&c, p, sc, region, hasRegion); err != nil {
		return model.Cue{}, false, err
	}
	c.Forced = r.paragraphForced(p, sc, region, hasRegion)
	return c, open, nil
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

// absoluteInterval reads the times of a paragraph without the offset of its parent. Some files give the times of a
// paragraph in a timed div as absolute times, so the paragraph begins after the div ends.
func (r *reader) absoluteInterval(p *xmlraw.Node, begin, parentEnd float64) (interval, *model.ParseError) {
	msg := "The paragraph begins at " + timecode.Number(round3(begin)) + "s, but its parent ends at " + timecode.Number(round3(parentEnd)) + "s."
	absolute := interval{}
	if _, ok := ttmldoc.PlainAttr(p, "begin"); ok {
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

func (r *reader) isInParagraph(n *xmlraw.Node) bool {
	for ; n != nil && n.Kind == xmlraw.ElementNode; n = n.Parent {
		if ttmldoc.IsElement(n, r.ns, "p") {
			return true
		}
	}
	return false
}

// paragraphRuns returns the runs of a paragraph with the styles of its region, its containers and its own.
func (r *reader) paragraphRuns(p *xmlraw.Node, sc scope, region string, hasRegion bool, iv interval) []run {
	st := ttmldoc.Style{}.Apply(r.styles.RegionProperties(region, hasRegion))
	for _, props := range sc.properties {
		st = st.Apply(props)
	}
	st = st.Apply(r.styles.OwnProperties(p))
	var runs []run
	r.collectRuns(p, st, r.agentName(p), preservesSpace(p, sc.preserveSpace), iv, &runs)
	return runs
}

// agentName returns the name of the first agent of the ttm:agent attribute of n, or the ID without a named agent.
func (r *reader) agentName(n *xmlraw.Node) string {
	v, _ := familyAttr(n, ttmldoc.MetadataNamespaces, "ttm", "agent")
	ids := ttmldoc.SplitSpace(v)
	if len(ids) == 0 {
		return ""
	}
	if name, ok := r.agents[ids[0]]; ok {
		return name
	}
	return ids[0]
}

func preservesSpace(n *xmlraw.Node, inherited bool) bool {
	space := ttmldoc.XMLAttr(n, "space")
	if space == "" {
		return inherited
	}
	return space == "preserve"
}

// collectRuns appends the runs of the content of n. A <span> with a time adds a timed span before its content.
func (r *reader) collectRuns(n *xmlraw.Node, st ttmldoc.Style, agent string, preserveSpace bool, iv interval, runs *[]run) {
	for _, child := range n.Children {
		switch {
		case child.Kind == xmlraw.TextNode, child.Kind == xmlraw.CDATANode:
			addTextRuns(runs, child.Data, st, agent, preserveSpace)
		case ttmldoc.IsElement(child, r.ns, "br"):
			*runs = append(*runs, run{lineBreak: true})
		case ttmldoc.IsElement(child, r.ns, "span"):
			spanInterval := iv
			if slices.ContainsFunc(timingAttributes, func(name string) bool { _, ok := ttmldoc.PlainAttr(child, name); return ok }) {
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
			r.collectRuns(child, st.Apply(r.styles.OwnProperties(child)), spanAgent, preservesSpace(child, preserveSpace), spanInterval, runs)
		}
	}
}

// addTextRuns appends the runs of a text node. With xml:space="preserve", each LF becomes a line break.
// See https://www.w3.org/TR/ttml2/#content-attribute-space.
func addTextRuns(runs *[]run, text string, st ttmldoc.Style, agent string, preserveSpace bool) {
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

// orderedTimes returns the cue times of a paragraph. An end before the begin fails, or lenient mode swaps them.
func (r *reader) orderedTimes(p *xmlraw.Node, iv interval) (time.Duration, time.Duration, *model.ParseError) {
	start, end := timecode.FromSeconds(iv.begin), timecode.FromSeconds(iv.end)
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

// runsToLines turns the runs into lines of core markup. A run that starts with a space loses it after text that
// ends with a space and at the start of a line, and the trailing spaces of a line go.
func (r *reader) runsToLines(runs []run) []string {
	var lines []string
	var line []run
	openAgent := ""
	runs = markAgentEnds(runs)
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

// markAgentEnds returns the runs with agentEnd set on the last run of each agent text. That is a run with visible
// text whose next run with visible text has another agent, or none. One agent that covers all the text is the
// speaker of the cue, so it gets no end.
func markAgentEnds(runs []run) []run {
	runs = slices.Clone(runs)
	var visible []*run
	for i := range runs {
		if visibleRun(runs[i]) {
			visible = append(visible, &runs[i])
		}
	}
	if len(visible) == 0 {
		return runs
	}
	if visible[0].agent != "" && !slices.ContainsFunc(visible, func(rn *run) bool { return rn.agent != visible[0].agent }) {
		return runs
	}
	for i, rn := range visible {
		if i == len(visible)-1 || visible[i+1].agent != rn.agent {
			rn.agentEnd = true
		}
	}
	return runs
}

func visibleRun(rn run) bool {
	return !rn.timed && !rn.lineBreak && strings.Trim(rn.text, markupcore.TrimSet) != ""
}

// runsToMarkup writes the runs of one line. A speaker tag opens before the text of its agent and closes after it,
// also across lines.
func runsToMarkup(runs []run, openAgent *string) string {
	var b strings.Builder
	var styled []markupcore.Run
	for _, rn := range runs {
		if rn.timed {
			b.WriteString(markupcore.StyleRuns(styled, false) + "<" + markupcore.CoreTimestamp(timecode.FromSeconds(rn.span.begin)) + ">")
			styled = nil
			continue
		}
		if visibleRun(rn) && rn.agent != *openAgent {
			b.WriteString(markupcore.StyleRuns(styled, false))
			if rn.agent != "" {
				b.WriteString(markupcore.VoiceTag(rn.agent, ""))
			}
			styled = nil
			*openAgent = rn.agent
		}
		styled = append(styled, rn.style.Run(rn.text))
		if rn.agentEnd && *openAgent != "" {
			b.WriteString(markupcore.StyleRuns(styled, false) + "</v>")
			styled = nil
			*openAgent = ""
		}
	}
	return b.String() + markupcore.StyleRuns(styled, false)
}

// setCueData sets the cue data and the alignment of the cue of paragraph p.
func (r *reader) setCueData(c *model.Cue, p *xmlraw.Node, sc scope, region string, hasRegion bool) *model.ParseError {
	attrs := readAttributes(p, "begin", "end", "dur", "xml:id")
	if _, ok := ttmldoc.PlainAttr(p, "region"); !ok && sc.hasRegion {
		attrs.Set("region", sc.region)
	}
	textAlign, hasTextAlign := sc.textAlign, sc.hasTextAlign
	if v, ok := r.styles.OwnProperties(p)["textAlign"]; ok {
		textAlign, hasTextAlign = v, true
	}
	data := model.NewData()
	if attrs.Len() > 0 {
		data.Set("attributes", attrs)
	}
	if a, ok := r.alignment(region, hasRegion, textAlign, hasTextAlign); ok {
		c.Position.Alignment = a
		data.Set("alignment", int64(a.Numpad()))
		c.Position.Offset = r.offset(region, hasRegion, a)
	}
	if data.Len() == 0 {
		return nil
	}
	if err := c.SetFormatData(formatDataKey, data); err != nil {
		return r.parseError(p.Line, err.Error())
	}
	return nil
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
	for i := range cues {
		cues[i].ExtractWordTags()
	}
	if err := s.AddCues(cues...); err != nil {
		return err
	}
	return nil
}

// xmlLines returns the lines of the serialised element, trimmed, without the empty ones, for warnings.
func xmlLines(n *xmlraw.Node) []string {
	var lines []string
	for line := range strings.SplitSeq(ttmldoc.Serialize(n), "\n") {
		if line = strings.Trim(line, markupcore.TrimSet); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func round3(f float64) float64 {
	return math.Round(f*1000) / 1000
}
