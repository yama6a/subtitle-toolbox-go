package ass

import (
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/position"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/ssatag"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

var (
	styleTagRegexp = regexp.MustCompile(`^<(/?)([bius])>$`)
	fontRegexp     = regexp.MustCompile(`(?i)^<font\b[^>]*>$`)
	// tagKindRegexp reads the kind of a block tag. A later tag of the kinds b, i, u, s and c replaces an earlier
	// one, and \clip is no c tag.
	tagKindRegexp = regexp.MustCompile(`^\\([biusc])(?:[^a-z]|$)`)
)

// fileData holds the file format data that the writer writes, with the defaults for absent keys.
type fileData struct {
	sectionOrder       []string
	scriptInfoComments []string
	scriptInfo         record
	stylesSection      string
	hasStylesSection   bool
	styleFormat        []string // nil is null
	styles             []record
	eventFormat        []string // nil is null
	commentEvents      []record
	sections           *model.Data
}

// writer holds the settings of one Write call.
type writer struct {
	ssa        bool
	stripTags  bool
	karaokeTag string
	// styles are the styles before the Style option, so the option changes the look of every cue without its own tag.
	styles []record
	legacy bool
}

// part is an override tag or a text run of a converted Text field.
type part struct {
	tag   bool
	value string
}

// Write writes s as ASS, or as SSA when s was read from SSA. The text \N, \n or \h gets U+2060 WORD JOINER after the
// backslash, so it reads back as text.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	s, err := writing.SkipImageCues(s, model.ASS, common)
	if err != nil {
		return WriteReport{}, err
	}
	if err := opts.Validate(); err != nil {
		return WriteReport{}, err
	}
	stored, _ := s.FormatData(formatDataKey)
	f := readFileData(stored)
	wr := &writer{
		ssa:        isSSA(f.scriptInfo, f.stylesSection),
		stripTags:  common.StripTags,
		karaokeTag: opts.KaraokeTag.String(),
		styles:     f.styles,
		legacy:     f.hasStylesSection && equalFold(f.stylesSection, ssaStylesSection),
	}
	if opts.Style != "" {
		if err := f.applyStyle(opts.Style); err != nil {
			return WriteReport{}, err
		}
	}

	var body []byte
	for i, section := range f.order() {
		if i > 0 {
			body = append(body, '\n')
		}
		body = append(body, "["+section+"]\n"...)
		for _, l := range wr.sectionLines(s, &f, section) {
			body = append(body, l+"\n"...)
		}
	}
	return WriteReport{}, writing.Write(w, model.ASS, body, common, true)
}

// order returns the sections to write: the stored order with [Script Info] first and [Events] last when the
// order lacks them, and each name once.
func (f *fileData) order() []string {
	order := slices.Clone(f.sectionOrder)
	if !containsFold(order, scriptInfoSection) {
		order = append([]string{scriptInfoSection}, order...)
	}
	if !containsFold(order, eventsSection) {
		order = append(order, eventsSection)
	}
	var unique []string
	for _, section := range order {
		if !slices.Contains(unique, section) {
			unique = append(unique, section)
		}
	}
	return unique
}

func (wr *writer) sectionLines(s *model.Subtitle, f *fileData, section string) []string {
	switch {
	case equalFold(section, scriptInfoSection):
		return f.scriptInfoLines(s)
	case f.hasStylesSection && equalFold(section, f.stylesSection):
		return f.styleLines()
	case equalFold(section, eventsSection):
		return wr.eventLines(s, f)
	default:
		lines, _ := f.sections.Strings(section)
		return lines
	}
}

// readFileData reads the stored file data. An absent key takes its default. A present key keeps its value, also
// when it is null or empty.
func readFileData(d *model.Data) fileData {
	f := fileData{
		sectionOrder:     []string{scriptInfoSection, assStylesSection, eventsSection},
		scriptInfo:       record{names: []string{"ScriptType", "PlayResX", "PlayResY", "ScaledBorderAndShadow"}, values: []string{"v4.00+", "384", "288", "yes"}},
		stylesSection:    assStylesSection,
		hasStylesSection: true,
		styleFormat:      assStyleFields,
		styles:           []record{{names: assStyleFields, values: defaultStyle}},
		eventFormat:      assEventFields,
	}
	if _, ok := d.Get("sectionOrder"); ok {
		f.sectionOrder, _ = d.Strings("sectionOrder")
	}
	if _, ok := d.Get("scriptInfoComments"); ok {
		f.scriptInfoComments, _ = d.Strings("scriptInfoComments")
	}
	if v, ok := d.Get("scriptInfo"); ok {
		f.scriptInfo = recordOf(v)
	}
	if _, ok := d.Get("stylesSection"); ok {
		f.stylesSection, f.hasStylesSection = d.String("stylesSection")
	}
	if _, ok := d.Get("styleFormat"); ok {
		f.styleFormat, _ = d.Strings("styleFormat")
	}
	if _, ok := d.Get("styles"); ok {
		f.styles = records(d, "styles")
	}
	if _, ok := d.Get("eventFormat"); ok {
		f.eventFormat, _ = d.Strings("eventFormat")
	}
	f.commentEvents = records(d, "commentEvents")
	f.sections, _ = d.Object("sections")
	return f
}

func records(d *model.Data, key string) []record {
	v, _ := d.Get(key)
	var items []any
	switch v := v.(type) {
	case []any:
		items = v
	case *model.Data:
		for _, k := range v.Keys() {
			item, _ := v.Get(k)
			items = append(items, item)
		}
	}
	out := make([]record, len(items))
	for i, item := range items {
		out[i] = recordOf(item)
	}
	return out
}

func containsFold(names []string, name string) bool {
	return slices.ContainsFunc(names, func(n string) bool { return equalFold(n, name) })
}

func (f *fileData) scriptInfoLines(s *model.Subtitle) []string {
	lines := slices.Clone(f.scriptInfoComments)
	if title, ok := s.Metadata(model.MetadataTitle); ok {
		lines = append(lines, "Title: "+markupcore.ToSingleLine(title))
	}
	for i, k := range f.scriptInfo.names {
		lines = append(lines, k+": "+f.scriptInfo.values[i])
	}
	return lines
}

func (f *fileData) styleLines() []string {
	format := f.styleFormat
	if format == nil {
		format = assStyleFields
	}
	lines := make([]string, 0, 1+len(f.styles))
	lines = append(lines, "Format: "+strings.Join(format, ", "))
	for _, style := range f.styles {
		lines = append(lines, "Style: "+joinFields(style, format))
	}
	return lines
}

// joinFields returns the values of r in the order of format, joined with commas. A missing value is empty.
func joinFields(r record, format []string) string {
	values := make([]string, len(format))
	for i, name := range format {
		values[i], _ = r.get(name)
	}
	return strings.Join(values, ",")
}

func (wr *writer) eventLines(s *model.Subtitle, f *fileData) []string {
	format := f.eventFormat
	if format == nil {
		format = assEventFields
	}
	stored := slices.Clone(f.commentEvents)
	used := make([]bool, len(stored))
	comments := make([][]string, s.Len()+1)
	for _, c := range s.Comments() {
		at := min(c.BeforeCue, s.Len())
		comments[at] = append(comments[at], c.Text)
	}
	lines := []string{"Format: " + strings.Join(format, ", ")}
	var last time.Duration
	for i := range s.Len() + 1 {
		c, ok := s.Cue(i)
		at := last
		if ok {
			at = c.Start
		}
		for _, text := range comments[i] {
			lines = append(lines, commentLine(text, format, stored, used, at))
		}
		if ok {
			lines = append(lines, wr.dialogueLine(c, format))
			last = c.End
		}
	}
	return lines
}

// commentLine writes the first unused stored Comment event with the text, or a new Comment event at the time at.
func commentLine(text string, format []string, stored []record, used []bool, at time.Duration) string {
	for i, ev := range stored {
		if v, ok := ev.field("Text"); ok && v == text && !used[i] {
			used[i] = true
			return "Comment: " + joinFields(ev, format)
		}
	}
	t := clockText(at)
	values := make([]string, len(format))
	for i, name := range format {
		switch {
		case equalFold(name, "Start"), equalFold(name, "End"):
			values[i] = t
		case equalFold(name, "Text"):
			values[i] = strings.ReplaceAll(text, "\n", `\N`)
		default:
			values[i] = defaultFieldValue(name)
		}
	}
	return "Comment: " + strings.Join(values, ",")
}

func (wr *writer) dialogueLine(c model.Cue, format []string) string {
	data, _ := c.FormatData(formatDataKey)
	fieldsValue, _ := data.Get("fields")
	fields := recordOf(fieldsValue)
	var text, name string
	if wr.unchanged(c, data, fields) {
		text, _ = data.String("text")
		name, _ = fields.field("Name")
	} else {
		styleName, ok := fields.field("Style")
		if !ok {
			styleName = "Default"
		}
		style := eventStyle(wr.styles, styleName)
		c.Lines = c.TaggedLines()
		text, name = wr.convertLines(c, style)
	}
	values := make([]string, len(format))
	for i, field := range format {
		switch {
		case equalFold(field, "Start"):
			values[i] = clockText(c.Start)
		case equalFold(field, "End"):
			values[i] = clockText(c.End)
		case equalFold(field, "Text"):
			values[i] = text
		case equalFold(field, "Name"):
			values[i] = name
		default:
			v, ok := fields.get(field)
			if !ok {
				v = defaultFieldValue(field)
			}
			values[i] = v
		}
	}
	return "Dialogue: " + strings.Join(values, ",")
}

// unchanged reports whether the cue has the lines, the speaker, the words, the word lengths and the alignment
// that the stored cue data encodes, so the writer writes its stored Text field.
func (wr *writer) unchanged(c model.Cue, data *model.Data, fields record) bool {
	if wr.stripTags {
		return false
	}
	text, ok := data.String("text")
	if !ok {
		return false
	}
	lines, ok := data.Strings("lines")
	if !ok {
		return false
	}
	stored := model.Cue{End: c.End, Lines: lines}
	stored.ExtractWordTags()
	if !slices.Equal(stored.Lines, c.Lines) || !sameSpeaker(stored, c) || !sameWords(stored.Words, c.Words) {
		return false
	}
	return wr.lengthsUnchanged(c, text, fields) && position.AlignmentUnchanged(data, c.Position.Alignment)
}

// sameSpeaker reports whether both cues have the same speaker. A cue without visible text has no voice tag in its
// lines, so its speaker, which the writer does not use for stored text, is not compared.
func sameSpeaker(stored, c model.Cue) bool {
	return stored.Speaker == c.Speaker || stored.Speaker == "" && c.PlainText() == ""
}

// sameWords reports whether both lists have the same texts and starts.
func sameWords(a, b []model.Word) bool {
	return slices.EqualFunc(a, b, func(x, y model.Word) bool { return x.Text == y.Text && x.Start == y.Start })
}

// lengthsUnchanged reports whether each word still has the length of its karaoke syllable in text, or ends where
// the next word starts, or at the cue end for the last word, as the words of library JSON version 1 do. It returns
// true when text has no karaoke tags or a number of syllables other than the number of words.
func (wr *writer) lengthsUnchanged(c model.Cue, text string, fields record) bool {
	words := c.Words
	styleName, _ := fields.field("Style")
	m := textMarkup{styles: wr.styles, style: eventStyle(wr.styles, styleName)}
	if _, _, err := m.convert(text, 0, false); err != nil {
		return true
	}
	var lengths []time.Duration
	for _, s := range m.syllables {
		if s.hasText {
			lengths = append(lengths, s.end-s.start)
		}
	}
	if len(lengths) != len(words) {
		return true
	}
	for i, w := range words {
		next := c.End
		if i+1 < len(words) {
			next = words[i+1].Start
		}
		if (w.End-w.Start-lengths[i]).Abs() > time.Millisecond && w.End != next {
			return false
		}
	}
	return true
}

// convertLines returns the Text field and the speaker name of the core markup of c. It leaves out the tags
// that the style of the event implies.
func (wr *writer) convertLines(c model.Cue, style record) (string, string) {
	text := markupcore.RubyAsText(strings.Join(c.Lines, "\n"))
	speaker := c.Speaker
	if speaker == "" {
		speaker, _ = markupcore.Speaker(text)
	}
	name := strings.ReplaceAll(speaker, ",", "")

	var parts []part
	numpad := c.Position.Alignment.Numpad()
	styleNumpad := styleAlignment(style, wr.legacy)
	if styleNumpad == 0 {
		styleNumpad = model.BottomCenter.Numpad()
	}
	if numpad != styleNumpad {
		if wr.ssa {
			parts = append(parts, part{true, `\a` + ssatag.LegacyCode(numpad)})
		} else {
			parts = append(parts, part{true, `\an` + strconv.Itoa(numpad)})
		}
	}
	var tokens []string
	if wr.stripTags {
		tokens = []string{markupcore.StripAllTags(text)}
	} else {
		tokens = markupcore.SplitTags(markupcore.WebVTTColorsToFont(text))
		for _, name := range removeStyleTags(tokens, styleTags(style)) {
			parts = append(parts, part{true, `\` + name + "0"})
		}
	}
	parts = append(parts, wr.convertTokens(tokens, c)...)
	return mergeBlocks(parts), name
}

// mergeBlocks joins adjacent tags into one {...} block. In one block the last \b, \i, \u, \s or \c wins, so an
// earlier tag of the same kind goes.
func mergeBlocks(parts []part) string {
	var b strings.Builder
	type blockTag struct{ kind, value string }
	var block []blockTag
	for _, p := range append(parts, part{}) {
		if p.tag {
			kind := ""
			if m := tagKindRegexp.FindStringSubmatch(p.value); m != nil {
				kind = m[1]
			}
			if kind != "" {
				block = slices.DeleteFunc(block, func(t blockTag) bool { return t.kind == kind })
			}
			block = append(block, blockTag{kind, p.value})
			continue
		}
		if len(block) > 0 {
			b.WriteByte('{')
			for _, t := range block {
				b.WriteString(t.value)
			}
			b.WriteByte('}')
			block = nil
		}
		b.WriteString(p.value)
	}
	return b.String()
}

// removeStyleTags empties the outer tag pairs that the style implies, such as <i> around all text of an italic
// style. It returns the style tags that do not wrap all text, which the writer turns off at the start.
// The tokens hold text runs at the even indexes and tags at the odd indexes.
func removeStyleTags(tokens, styleTags []string) []string {
	remaining := slices.Clone(styleTags)
	for {
		i, ok := outerStyleTag(tokens, remaining)
		if !ok {
			return remaining
		}
		name := tokens[i][1:2]
		remaining = slices.DeleteFunc(remaining, func(t string) bool { return t == name })
		closing, _ := closingIndex(tokens, i)
		tokens[closing] = ""
		tokens[i] = ""
	}
}

// outerStyleTag returns the index of a first <b>, <i>, <u> or <s> tag of names whose pair wraps all text.
func outerStyleTag(tokens, names []string) (int, bool) {
	for i, token := range tokens {
		if i%2 == 0 {
			if token != "" {
				return 0, false
			}
			continue
		}
		m := styleTagRegexp.FindStringSubmatch(token)
		if m == nil || m[1] != "" {
			continue
		}
		if !slices.Contains(names, m[2]) {
			return 0, false
		}
		closing, ok := closingIndex(tokens, i)
		if !ok {
			return 0, false
		}
		for j := closing + 1; j < len(tokens); j += 2 {
			if tokens[j] != "" {
				return 0, false
			}
		}
		return i, true
	}
	return 0, false
}

func closingIndex(tokens []string, opening int) (int, bool) {
	name := tokens[opening][1 : len(tokens[opening])-1]
	depth := 0
	for i := opening; i < len(tokens); i += 2 {
		switch tokens[i] {
		case "<" + name + ">":
			depth++
		case "</" + name + ">":
			depth--
		}
		if depth == 0 {
			return i, true
		}
	}
	return 0, false
}

// convertTokens returns the override tags and the escaped text of the tokens.
func (wr *writer) convertTokens(tokens []string, c model.Cue) []part {
	leading, syllables := karaokeSyllables(tokens, c)
	var parts []part
	if leading >= 0 {
		parts = append(parts, part{true, `\` + wr.karaokeTag + strconv.FormatInt(leading, 10)})
	}
	tc := tokenConverter{karaokeTag: wr.karaokeTag, syllables: syllables}
	for i, token := range tokens {
		switch {
		case i%2 == 0 && token != "":
			parts = append(parts, part{false, escapeText(markupcore.DecodeEntities(token))})
		case i%2 == 1:
			if tag, ok := tc.tag(token); ok {
				parts = append(parts, part{true, tag})
			}
		}
	}
	return parts
}

// tokenConverter turns the core markup tags of one cue into override tags.
type tokenConverter struct {
	karaokeTag string
	syllables  []syllableTime
	next       int
	// colors holds the colors of the open <font> tags, "" for a <font> without a color.
	colors []string
}

// tag returns the override tag of a core markup tag, or false for a tag that writes nothing.
func (tc *tokenConverter) tag(token string) (string, bool) {
	if m := styleTagRegexp.FindStringSubmatch(token); m != nil {
		if m[1] == "" {
			return `\` + m[2] + "1", true
		}
		return `\` + m[2] + "0", true
	}
	if fontRegexp.MatchString(token) {
		color := fontColor(token)
		tc.colors = append(tc.colors, color)
		return colorTag(color), color != ""
	}
	if equalFold(token, "</font>") {
		return tc.closeFont()
	}
	if _, ok := markupcore.WordTimestamp(token); ok {
		s := tc.syllables[tc.next]
		tc.next++
		if s.skip {
			return "", false
		}
		tag := `\` + tc.karaokeTag + strconv.FormatInt(s.length, 10)
		if s.gap > 0 {
			tag = `\` + tc.karaokeTag + strconv.FormatInt(s.gap, 10) + tag
		}
		return tag, true
	}
	return "", false
}

// closeFont returns \c, or the color of the outer <font> again, when the closed <font> had a color.
func (tc *tokenConverter) closeFont() (string, bool) {
	n := len(tc.colors)
	if n == 0 {
		return "", false
	}
	closed := tc.colors[n-1]
	tc.colors = tc.colors[:n-1]
	if closed == "" {
		return "", false
	}
	for i := len(tc.colors) - 1; i >= 0; i-- {
		if tc.colors[i] != "" {
			return colorTag(tc.colors[i]), true
		}
	}
	return `\c`, true
}

// fontColor returns the color of a <font> tag as 6 upper-case hex digits in RGB order, or "".
func fontColor(tag string) string {
	attr, ok := markupcore.FontColor(tag)
	if !ok {
		return ""
	}
	hex, ok := markupcore.ColorHex(attr)
	if !ok {
		return ""
	}
	return strings.ToUpper(hex[1:])
}

func colorTag(rgb string) string {
	return `\c&H` + markupcore.RGBToBGR(rgb) + "&"
}

// textEscapes writes LF and U+00A0 as their ASS codes. Text that already holds such a code gets a word joiner after the
// backslash, so it reads back as text.
//
//nolint:gochecknoglobals // a replacer holds no state
var textEscapes = strings.NewReplacer("\n", `\N`, nbsp, `\h`,
	`\N`, `\`+writing.WordJoiner+`N`, `\n`, `\`+writing.WordJoiner+`n`, `\h`, `\`+writing.WordJoiner+`h`)

func escapeText(text string) string {
	return textEscapes.Replace(text)
}

// syllableTime is the length of one karaoke syllable and the gap before it, in centiseconds. A syllable with skip
// set belongs to a timestamp that no word starts at, and writes nothing.
type syllableTime struct {
	length, gap int64
	skip        bool
}

// karaokeSyllables turns the word timestamps into karaoke syllables, one per timestamp token. A syllable lasts
// until the end of its word when the next word starts later, and a gap syllable fills the time between. The last
// syllable lasts until the cue end. leading is the time before the first word when text or a skipped timestamp
// comes before it, else -1.
func karaokeSyllables(tokens []string, c model.Cue) (leading int64, syllables []syllableTime) {
	start := centiseconds(c.Start)
	times, textBefore := timestampTimes(tokens, start)
	if len(times) == 0 {
		return -1, nil
	}
	kept, byWord := wordTimestamps(times, c.Words)
	syllables = make([]syllableTime, len(times))
	for i := range syllables {
		syllables[i].skip = !slices.Contains(kept, i)
	}
	fillSyllables(syllables, times, kept, c.Words, byWord, max(start, centiseconds(c.End)))
	if !textBefore && kept[0] == 0 {
		return -1, syllables
	}
	return times[kept[0]] - start, syllables
}

// fillSyllables sets the length and the gap of the syllables of the kept timestamps. byWord says that words has one
// word for each kept timestamp, so a word end before the next start makes a gap.
func fillSyllables(syllables []syllableTime, times []int64, kept []int, words []model.Word, byWord bool, cueEnd int64) {
	for k, i := range kept {
		next := cueEnd
		if k+1 < len(kept) {
			next = times[kept[k+1]]
		}
		syllables[i].length = max(0, next-times[i])
		if !byWord || k+1 == len(kept) {
			continue
		}
		if end := centiseconds(words[k].End); end >= times[i] && end < next {
			syllables[i].length = end - times[i]
			syllables[kept[k+1]].gap = next - end
		}
	}
}

// wordTimestamps returns the indexes of the timestamps that the words start at, and whether it found one for each
// word. When it finds none for a word, it returns the indexes of all timestamps.
func wordTimestamps(times []int64, words []model.Word) (kept []int, byWord bool) {
	next := 0
	for _, w := range words {
		for next < len(times) && times[next] != max(0, centiseconds(w.Start)) {
			next++
		}
		if next == len(times) {
			kept = nil
			break
		}
		kept = append(kept, next)
		next++
	}
	if kept != nil && len(kept) == len(words) {
		return kept, true
	}
	kept = make([]int, len(times))
	for i := range kept {
		kept[i] = i
	}
	return kept, false
}

// timestampTimes returns the time of each word timestamp token in centiseconds, not before start, and whether
// text comes before the first one.
func timestampTimes(tokens []string, start int64) (times []int64, textBefore bool) {
	for i, token := range tokens {
		if i%2 == 1 {
			if d, ok := markupcore.WordTimestamp(token); ok {
				times = append(times, max(start, centiseconds(d)))
				continue
			}
		}
		if len(times) == 0 && i%2 == 0 && phpTrim(token) != "" {
			textBefore = true
		}
	}
	return times, textBefore
}

// centiseconds rounds d to centiseconds, half away from zero, up to the largest count below the time limit. The
// reader adds each \k value to the start and checks the sum against the limit.
func centiseconds(d time.Duration) int64 {
	return min(int64(d.Round(centisecond)/centisecond), timecode.Units(timecode.MaxTime, centisecond))
}

// clockText writes d as h:mm:ss.cc. A negative time writes as 0.
func clockText(d time.Duration) string {
	cs := max(0, centiseconds(d))
	return strconv.FormatInt(cs/360000, 10) + ":" + pad2(cs/6000%60) + ":" + pad2(cs/100%60) + "." + pad2(cs%100)
}

func pad2(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}

func defaultFieldValue(field string) string {
	switch {
	case equalFold(field, "Layer"), equalFold(field, "MarginL"), equalFold(field, "MarginR"), equalFold(field, "MarginV"):
		return "0"
	case equalFold(field, "Marked"):
		return "Marked=0"
	case equalFold(field, "Style"):
		return "Default"
	default:
		return ""
	}
}
