package ass

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
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
	section, inSection := "", false
	for n, line := range lines(content) {
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
		switch {
		case equalFold(section, scriptInfoSection):
			rd.scriptInfoLine(line)
		case isStylesSection(section):
			rd.styleLine(section, line)
		case equalFold(section, eventsSection):
			if err := rd.eventLine(line, n+1); err != nil {
				return nil, err
			}
		default:
			if _, ok := rd.f.sections[section]; !ok {
				rd.f.sectionNames = append(rd.f.sectionNames, section)
			}
			rd.f.sections[section] = append(rd.f.sections[section], line)
		}
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
		loose, err := rd.readEvent(ev, i)
		if err != nil {
			var perr *model.ParseError
			if !errors.As(err, &perr) {
				return nil, err
			}
			if err := rd.st.Fail(perr, ev.number, i, []string{ev.line}); err != nil {
				return nil, err
			}
			continue
		}
		for _, lt := range loose {
			msg := `The time "` + lt.text + `" is not in the form h:mm:ss.cc. The parser read it as ` + timecode.Seconds(lt.d) + " s."
			rd.st.Warn(msg, ev.number, i, []string{ev.line}, model.Repaired)
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

// lines yields the lines of content and their 0-based numbers. LF, a run of CR before LF, and a lone CR end a line.
func lines(content string) iter.Seq2[int, string] {
	return func(yield func(int, string) bool) {
		n := 0
		for {
			i := strings.IndexAny(content, "\r\n")
			if i < 0 {
				yield(n, content)
				return
			}
			if !yield(n, content[:i]) {
				return
			}
			next := i + 1
			if content[i] == '\r' {
				j := i
				for j < len(content) && content[j] == '\r' {
					j++
				}
				if j < len(content) && content[j] == '\n' {
					next = j + 1
				}
			}
			content = content[next:]
			n++
		}
	}
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
		if !isIntKey(name) {
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

// readEvent reads the event ev, number index, and returns the loose times that lenient mode read.
func (rd *reader) readEvent(ev event, index int) ([]looseTime, error) {
	value := ev.value
	if rd.st.Lenient() {
		value = joinCommaFractions(ev.format, value)
	}
	all, ok := combine(ev.format, value, false)
	if !ok {
		return nil, &model.ParseError{Line: ev.number, Msg: `The line "` + ev.line + `" has fewer fields than the Format line of the [Events] section.`}
	}
	var fields record
	for i, name := range all.names {
		if !isIntKey(name) {
			fields.set(name, all.values[i])
		}
	}
	startKey, okStart := fields.key("Start")
	endKey, okEnd := fields.key("End")
	textKey, okText := fields.key("Text")
	if !okStart || !okEnd || !okText {
		return nil, &model.ParseError{Line: ev.number, Msg: "The Format line of the [Events] section needs the fields Start, End and Text."}
	}
	startText, _ := fields.get(startKey)
	endText, _ := fields.get(endKey)
	startText, endText = strings.ReplaceAll(startText, "\x00", ","), strings.ReplaceAll(endText, "\x00", ",")
	fields.set(startKey, startText)
	fields.set(endKey, endText)
	text, _ := fields.get(textKey)

	if ev.comment {
		rd.f.commentEvents = append(rd.f.commentEvents, fields)
		rd.comments = append(rd.comments, parsing.ParsedComment{Text: text, CuesBefore: len(rd.cues)})
		return nil, nil
	}

	var loose []looseTime
	start, err := rd.time(startText, ev.number, &loose)
	if err != nil {
		return nil, err
	}
	styleName, _ := fields.field("Style")
	style, _ := eventStyle(rd.f.styles, styleName)
	m := textMarkup{styles: rd.f.styles}
	lines, numpad, err := m.convert(text, start, rd.f.scriptInfo.lastField("WrapStyle") == "2", ev.number, style)
	if err != nil {
		return nil, err
	}
	if numpad == 0 {
		numpad = styleAlignment(style, rd.legacyStyles())
	}
	name, _ := fields.field("Name")
	lines = markupcore.AddSpeaker(lines, name)

	end, err := rd.time(endText, ev.number, &loose)
	if err != nil {
		return nil, err
	}
	if start, end, err = rd.st.OrderedTimes(start, end, ev.number, index, []string{ev.line}, true); err != nil {
		return nil, err
	}
	c := model.Cue{Start: start, End: end}
	c.SetText(strings.Join(lines, "\n"))
	var alignment any
	if numpad > 0 {
		c.Alignment, _ = model.AlignmentFromNumpad(numpad)
		alignment = int64(numpad)
	}
	var kept record
	for i, n := range fields.names {
		if n != startKey && n != endKey && n != textKey {
			kept.set(n, fields.values[i])
		}
	}
	data := model.NewData(
		model.Field{Key: "fields", Value: kept.data()},
		model.Field{Key: "text", Value: text},
		model.Field{Key: "lines", Value: anyList(c.Lines)},
		model.Field{Key: "alignment", Value: alignment},
	)
	if err := c.SetFormatData(formatDataKey, data); err != nil {
		return nil, fmt.Errorf("%s cue data: %w", model.ASS, err)
	}
	rd.cues = append(rd.cues, c)
	return loose, nil
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
	return phpTrim(typ), strings.TrimLeft(value, phpTrimSet)
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
	var r record
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
