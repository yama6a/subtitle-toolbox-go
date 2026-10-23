package ass

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/phpkey"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// maxHourDigits is the digit count above which a time is 100000 hours or more, whatever its other fields.
const maxHourDigits = 6

var (
	sectionRegexp       = regexp.MustCompile(`^\[([A-Za-z][A-Za-z0-9+ ]*)\]$`)
	timeRegexp          = regexp.MustCompile(`^(\d+):([0-5]?\d):([0-5]?\d)\.(\d{1,3})$`)
	commaTimeRegexp     = regexp.MustCompile(`^[\t\n\v\f\r ]*\d+:\d{1,2}:\d{1,2}$`)
	commaFractionRegexp = regexp.MustCompile(`^\d{1,4}[\t\n\v\f\r ]*$`)
)

// event is a Dialogue or Comment line, which the reader reads after all sections.
type event struct {
	format  []string
	line    string
	number  int // the 1-based line number
	value   string
	comment bool
}

// file holds the file format data while the reader reads the sections.
type file struct {
	sectionOrder       []string
	scriptInfoComments []string
	scriptInfo         record
	stylesSection      string
	hasStylesSection   bool
	styleFormat        []string // nil is null
	styles             []record
	eventFormat        []string
	commentEvents      []record
	sectionNames       []string
	sections           map[string][]string
}

type reader struct {
	st       *parsing.State
	f        file
	title    string
	hasTitle bool
	events   []event
	cues     []model.Cue
	comments []parsing.ParsedComment
	// textLine is the first line in [Events] without a descriptor, or 0.
	textLine int
}

// looseTime is a time that lenient mode read with timecode.Loose.
type looseTime struct {
	text string
	d    time.Duration
}

// Read reads ASS or SSA text. It expects UTF-8, as stbox.Parse passes it.
func Read(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.ASS, err)
	}
	st := parsing.NewState(model.ASS, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return &model.Subtitle{}, st.Warnings(), nil
	}
	rd := &reader{st: st, f: file{sections: map[string][]string{}}}
	s, err := rd.read(string(content))
	if err != nil {
		return nil, nil, err
	}
	if err := st.NoCues(s.Len(), func() (int, bool) { return rd.textLine, rd.textLine > 0 }); err != nil {
		return nil, nil, err
	}
	return s, st.Warnings(), nil
}

func (rd *reader) read(content string) (*model.Subtitle, error) {
	if err := rd.readSections(content); err != nil {
		return nil, err
	}
	// An editor can save an empty project without [Events], so [Script Info] alone gives an empty subtitle.
	if !rd.hasSection(eventsSection) && !rd.hasSection(scriptInfoSection) {
		return nil, &model.ParseError{Format: model.ASS, Msg: "The subtitle has no [Events] section."}
	}
	if rd.f.eventFormat == nil {
		rd.f.eventFormat = rd.defaultEventFormat()
	}
	// The events come last, because a styles section after [Events] still sets their styles.
	for i, ev := range rd.events {
		if err := rd.readEvent(ev, i); err != nil {
			return nil, err
		}
	}

	s := &model.Subtitle{}
	if rd.hasTitle {
		s.SetMetadata(model.MetadataTitle, rd.title)
	}
	if err := s.SetFormatData(formatDataKey, rd.f.data()); err != nil {
		return nil, fmt.Errorf("%s file data: %w", model.ASS, err)
	}
	parsing.AddParsed(s, rd.cues, rd.comments)
	return s, nil
}

func (rd *reader) readSections(content string) error {
	section, inSection := "", false
	for n, line := range parsing.Lines(content) {
		line = phpTrim(line)
		if line == "" {
			continue
		}
		if m := sectionRegexp.FindStringSubmatch(line); m != nil {
			section, inSection = m[1], true
			rd.startSection(section)
			continue
		}
		if !inSection {
			continue
		}
		if err := rd.sectionLine(section, line, n+1); err != nil {
			return err
		}
	}
	return nil
}

// sectionLine reads the trimmed line of section, on the 1-based line number.
func (rd *reader) sectionLine(section, line string, number int) error {
	switch {
	case equalFold(section, scriptInfoSection):
		rd.scriptInfoLine(line)
	case isStylesSection(section):
		rd.styleLine(section, line)
	case equalFold(section, eventsSection):
		return rd.eventLine(line, number)
	default:
		if _, ok := rd.f.sections[section]; !ok {
			rd.f.sectionNames = append(rd.f.sectionNames, section)
		}
		rd.f.sections[section] = append(rd.f.sections[section], line)
	}
	return nil
}

// readEvent reads the event ev, number index. Lenient mode skips an event that fails and warns.
func (rd *reader) readEvent(ev event, index int) error {
	loose, err := rd.parseEvent(ev, index)
	if err != nil {
		var perr *model.ParseError
		if !errors.As(err, &perr) {
			return err
		}
		return rd.st.Fail(perr, ev.number, index, []string{ev.line})
	}
	for _, lt := range loose {
		msg := `The time "` + lt.text + `" is not in the form h:mm:ss.cc. The parser read it as ` + timecode.Seconds(lt.d) + " s."
		rd.st.Warn(msg, ev.number, index, []string{ev.line}, model.Repaired)
	}
	return nil
}

func (rd *reader) hasSection(name string) bool {
	for _, s := range rd.f.sectionOrder {
		if equalFold(s, name) {
			return true
		}
	}
	return false
}

func (rd *reader) startSection(name string) {
	rd.f.sectionOrder = append(rd.f.sectionOrder, name)
	if isStylesSection(name) {
		rd.f.stylesSection, rd.f.hasStylesSection = name, true
	}
	if equalFold(name, eventsSection) && rd.f.eventFormat == nil {
		rd.f.eventFormat = rd.defaultEventFormat()
	}
}

func (rd *reader) defaultEventFormat() []string {
	if isSSA(rd.f.scriptInfo, rd.f.stylesSection) {
		return ssaEventFields
	}
	return assEventFields
}

// isSSA reports whether a file is SSA: its ScriptType is v4.00, or its styles section is [V4 Styles].
func isSSA(scriptInfo record, stylesSection string) bool {
	return equalFold(scriptInfo.lastField("ScriptType"), ssaScriptType) || equalFold(stylesSection, ssaStylesSection)
}

func (rd *reader) scriptInfoLine(line string) {
	if strings.HasPrefix(line, ";") {
		rd.f.scriptInfoComments = append(rd.f.scriptInfoComments, line)
		return
	}
	key, value, ok := strings.Cut(line, ":")
	if !ok {
		return
	}
	key, value = phpTrim(key), phpTrim(value)
	if equalFold(key, "Title") {
		rd.title, rd.hasTitle = value, true
		return
	}
	rd.f.scriptInfo.set(key, value)
}

func (rd *reader) styleLine(section, line string) {
	typ, value := splitDescriptor(line)
	switch {
	case equalFold(typ, "Format"):
		rd.f.styleFormat = splitNames(value)
	case equalFold(typ, "Style"):
		if rd.f.styleFormat == nil {
			rd.f.styleFormat = assStyleFields
			if equalFold(section, ssaStylesSection) {
				rd.f.styleFormat = ssaStyleFields
			}
		}
		style, _ := combine(rd.f.styleFormat, value, true)
		rd.f.styles = append(rd.f.styles, style)
	}
}

func (rd *reader) eventLine(line string, number int) error {
	if rd.textLine == 0 && !strings.Contains(line, ":") && !strings.HasPrefix(line, ";") {
		rd.textLine = number
	}
	typ, value := splitDescriptor(line)
	if equalFold(typ, "Format") {
		rd.f.eventFormat = splitNames(value)
		return rd.checkFieldNames(line, number)
	}
	comment := equalFold(typ, "Comment")
	if comment || equalFold(typ, "Dialogue") {
		rd.events = append(rd.events, event{format: rd.f.eventFormat, line: line, number: number, value: value, comment: comment})
	}
	return nil
}

// checkFieldNames rejects a field name that is a number, because the cue data ass.fields cannot hold it. Lenient
// mode drops the values of such a field.
func (rd *reader) checkFieldNames(line string, number int) error {
	for _, name := range rd.f.eventFormat {
		if !phpkey.IsIntKey(name) {
			continue
		}
		msg := `The field name "` + name + `" in the Format line of the [Events] section is a number.`
		if !rd.st.Lenient() {
			return &model.ParseError{Format: model.ASS, Line: number, Msg: msg}
		}
		rd.st.Warn(msg+" The parser dropped the values of this field.", number, model.NoBlock, []string{line}, model.Repaired)
	}
	return nil
}

// eventFields are the fields of one event without the fields whose names are numbers, and the names of its
// Start, End and Text fields.
type eventFields struct {
	record
	start, end, text string
}

// parseEvent reads the event ev, number index, and returns the loose times that lenient mode read. A Comment
// event becomes a comment.
func (rd *reader) parseEvent(ev event, index int) ([]looseTime, error) {
	f, err := rd.fields(ev)
	if err != nil {
		return nil, err
	}
	text, _ := f.get(f.text)
	if ev.comment {
		rd.f.commentEvents = append(rd.f.commentEvents, f.record)
		rd.comments = append(rd.comments, parsing.ParsedComment{Text: text, CuesBefore: len(rd.cues)})
		return nil, nil
	}

	var loose []looseTime
	startText, _ := f.get(f.start)
	start, err := rd.time(startText, ev.number, &loose)
	if err != nil {
		return nil, err
	}
	styleName, _ := f.field("Style")
	m := textMarkup{styles: rd.f.styles, style: eventStyle(rd.f.styles, styleName), line: ev.number}
	lines, numpad, err := m.convert(text, start, rd.f.scriptInfo.lastField("WrapStyle") == "2")
	if err != nil {
		return nil, err
	}
	if numpad == 0 {
		numpad = styleAlignment(m.style, rd.legacyStyles())
	}
	name, _ := f.field("Name")
	lines = markupcore.AddSpeaker(lines, name)

	endText, _ := f.get(f.end)
	end, err := rd.time(endText, ev.number, &loose)
	if err != nil {
		return nil, err
	}
	if start, end, err = rd.st.OrderedTimes(start, end, ev.number, index, []string{ev.line}, true); err != nil {
		return nil, err
	}
	c, err := newCue(start, end, lines, numpad, f, text)
	if err != nil {
		return nil, err
	}
	rd.cues = append(rd.cues, c)
	return loose, nil
}

// fields splits the value of ev into its fields. Lenient mode first joins the comma fractions of the times.
func (rd *reader) fields(ev event) (eventFields, error) {
	value := ev.value
	if rd.st.Lenient() {
		value = joinCommaFractions(ev.format, value)
	}
	all, ok := combine(ev.format, value, false)
	if !ok {
		return eventFields{}, &model.ParseError{Line: ev.number, Msg: `The line "` + ev.line + `" has fewer fields than the Format line of the [Events] section.`}
	}
	f := eventFields{record: record{names: make([]string, 0, len(all.names)), values: make([]string, 0, len(all.names))}}
	for i, name := range all.names {
		if !phpkey.IsIntKey(name) {
			f.set(name, all.values[i])
		}
	}
	var okStart, okEnd, okText bool
	f.start, okStart = f.key("Start")
	f.end, okEnd = f.key("End")
	f.text, okText = f.key("Text")
	if !okStart || !okEnd || !okText {
		return eventFields{}, &model.ParseError{Line: ev.number, Msg: "The Format line of the [Events] section needs the fields Start, End and Text."}
	}
	for _, k := range []string{f.start, f.end} {
		v, _ := f.get(k)
		f.set(k, strings.ReplaceAll(v, "\x00", ","))
	}
	return f, nil
}

// newCue returns the cue of an event with its cue data: the fields other than Start, End and Text, the Text
// field, the cue lines and the alignment, or null for none.
func newCue(start, end time.Duration, lines []string, numpad int, f eventFields, text string) (model.Cue, error) {
	c := model.Cue{Start: start, End: end}
	c.SetText(strings.Join(lines, "\n"))
	var alignment any
	if numpad > 0 {
		c.Alignment, _ = model.AlignmentFromNumpad(numpad)
		alignment = int64(numpad)
	}
	kept := record{names: make([]string, 0, len(f.names)), values: make([]string, 0, len(f.names))}
	for i, n := range f.names {
		if n != f.start && n != f.end && n != f.text {
			kept.set(n, f.values[i])
		}
	}
	data := model.NewData(
		model.Field{Key: "fields", Value: kept.data()},
		model.Field{Key: "text", Value: text},
		model.Field{Key: "lines", Value: anyList(c.Lines)},
		model.Field{Key: "alignment", Value: alignment},
	)
	if err := c.SetFormatData(formatDataKey, data); err != nil {
		return model.Cue{}, fmt.Errorf("%s cue data: %w", model.ASS, err)
	}
	return c, nil
}

func (rd *reader) legacyStyles() bool {
	return rd.f.hasStylesSection && equalFold(rd.f.stylesSection, ssaStylesSection)
}

// time reads a Start or End value of the event on line. Lenient mode also reads the forms of timecode.Loose and
// adds them to loose.
func (rd *reader) time(text string, line int, loose *[]looseTime) (time.Duration, error) {
	t := phpTrim(text)
	if m := timeRegexp.FindStringSubmatch(t); m != nil {
		return parsing.BoundedTime(clock(m[1], m[2], m[3])+fraction(m[4]), text, line)
	}
	if rd.st.Lenient() {
		if d, ok := timecode.Loose(t, ".,:", false); ok {
			*loose = append(*loose, looseTime{text: text, d: d})
			return parsing.BoundedTime(d, text, line)
		}
	}
	return 0, &model.ParseError{Line: line, Msg: `The time "` + text + `" is not valid.`}
}

// clock returns the time of the fields. Hours of more than 6 digits cannot be below 100000, so they give
// timecode.MaxTime and the multiplication cannot wrap.
func clock(hours, minutes, seconds string) time.Duration {
	hours = strings.TrimLeft(hours, "0")
	if len(hours) > maxHourDigits {
		return timecode.MaxTime
	}
	h, _ := strconv.Atoi("0" + hours)
	m, _ := strconv.Atoi(minutes)
	s, _ := strconv.Atoi(seconds)
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(s)*time.Second
}

// fraction returns the time of 1 to 3 digits after the decimal point: "5" is 500 ms and "05" is 50 ms.
func fraction(digits string) time.Duration {
	ms, _ := strconv.Atoi((digits + "00")[:3])
	return time.Duration(ms) * time.Millisecond
}

// splitDescriptor returns the trimmed descriptor before the first colon of line and the value after it.
func splitDescriptor(line string) (string, string) {
	typ, value, _ := strings.Cut(line, ":")
	return phpTrim(typ), strings.TrimLeft(value, markupcore.PHPTrimSet)
}

func splitNames(value string) []string {
	names := strings.Split(value, ",")
	for i, n := range names {
		names[i] = phpTrim(n)
	}
	return names
}

// combine maps the comma-separated values to the field names. The last field keeps any further commas and its
// white space. With pad, missing values are empty. Without it, too few values return false.
func combine(format []string, value string, pad bool) (record, bool) {
	values := strings.SplitN(value, ",", len(format))
	if len(values) < len(format) {
		if !pad {
			return record{}, false
		}
		values = append(values, make([]string, len(format)-len(values))...)
	}
	r := record{names: make([]string, 0, len(format)), values: make([]string, 0, len(format))}
	for i, name := range format {
		v := values[i]
		if i < len(format)-1 {
			v = phpTrim(v)
		}
		r.set(name, v)
	}
	return r, true
}

// joinCommaFractions joins a Start or End value such as "0:00:01" with the next value "50" into one field,
// as "0:00:01" NUL "50", when another value follows. The reader turns the NUL into a comma after the split.
func joinCommaFractions(format []string, value string) string {
	values := strings.Split(value, ",")
	for i, name := range format[:len(format)-1] {
		if (equalFold(name, "Start") || equalFold(name, "End")) && i+2 < len(values) &&
			commaTimeRegexp.MatchString(values[i]) && commaFractionRegexp.MatchString(values[i+1]) {
			values[i] += "\x00" + values[i+1]
			values = append(values[:i+1], values[i+2:]...)
		}
	}
	return strings.Join(values, ",")
}

func (f file) data() *model.Data {
	var stylesSection, styleFormat any
	if f.hasStylesSection {
		stylesSection = f.stylesSection
	}
	if f.styleFormat != nil {
		styleFormat = anyList(f.styleFormat)
	}
	styles := make([]any, len(f.styles))
	for i, s := range f.styles {
		styles[i] = s.data()
	}
	comments := make([]any, len(f.commentEvents))
	for i, c := range f.commentEvents {
		comments[i] = c.data()
	}
	var sections any = []any{}
	if len(f.sectionNames) > 0 {
		d := model.NewData()
		for _, name := range f.sectionNames {
			d.Set(name, anyList(f.sections[name]))
		}
		sections = d
	}
	return model.NewData(
		model.Field{Key: "sectionOrder", Value: anyList(f.sectionOrder)},
		model.Field{Key: "scriptInfoComments", Value: anyList(f.scriptInfoComments)},
		model.Field{Key: "scriptInfo", Value: f.scriptInfo.data()},
		model.Field{Key: "stylesSection", Value: stylesSection},
		model.Field{Key: "styleFormat", Value: styleFormat},
		model.Field{Key: "styles", Value: styles},
		model.Field{Key: "eventFormat", Value: anyList(f.eventFormat)},
		model.Field{Key: "commentEvents", Value: comments},
		model.Field{Key: "sections", Value: sections},
	)
}
