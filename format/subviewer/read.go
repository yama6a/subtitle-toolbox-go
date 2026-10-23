package subviewer

import (
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	startScript = "******** START SCRIPT ********"
	maxSeconds  = int64(timecode.MaxHours) * 3600
)

var (
	v1TimeRegexp    = regexp.MustCompile(`^\[(0*\d{1,5}):(\d{2}):(\d{2})\]$`)
	v2TimeRegexp    = regexp.MustCompile(`^(0*\d{1,5}):([0-5]\d):([0-5]\d)\.(\d{1,3}),(0*\d{1,5}):([0-5]\d):([0-5]\d)\.(\d{1,3})$`)
	clockLikeRegexp = regexp.MustCompile(`^\d+:\d{1,2}:\d{1,2}(?:\.\d{1,4})?$`)
)

type reader struct {
	st     *parsing.State
	s      *model.Subtitle
	header *model.Data

	// The state of a SubViewer 2 read. block counts the timing lines, good and bad.
	cues     []model.Cue
	cue      *model.Cue
	skipped  *skippedCue
	style    string
	hasStyle bool
	block    int
}

// Read reads SubViewer 1 or 2 from UTF-8 text.
func Read(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.SubViewer, err)
	}
	st := parsing.NewState(model.SubViewer, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}

	var lines []string
	for _, raw := range parsing.Lines(string(content)) {
		lines = append(lines, strings.Trim(raw, markupcore.PHPTrimSet))
	}
	rd := &reader{st: st, s: s, header: model.NewData()}
	var data *model.Data
	var cues []model.Cue
	if i := slices.Index(lines, startScript); i >= 0 {
		cues, data, err = rd.version1(lines[:i], lines[i+1:], i+2)
	} else {
		cues, data, err = rd.version2(lines)
	}
	if err != nil {
		return nil, nil, err
	}
	_ = s.SetFormatData(formatDataKey, data) // the data has the shape of the schema
	s.AddCues(cues...)
	return s, st.Warnings(), nil
}

// splitTag splits a line of the shape ^\[([^\]]+)\](.*)$ into the text in the brackets and the rest.
func splitTag(line string) (tag, rest string, ok bool) {
	if !strings.HasPrefix(line, "[") {
		return "", "", false
	}
	i := strings.IndexByte(line, ']')
	if i < 2 {
		return "", "", false
	}
	return line[1:i], line[i+1:], true
}

// asciiUpper raises a to z only, as PHP strtoupper does. It keeps every other byte, also of invalid UTF-8.
func asciiUpper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'a' <= c && c <= 'z' {
			b[i] = c - 'a' + 'A'
		}
	}
	return string(b)
}

// headerValue returns the header tags. A header whose tags are 0, 1 and so on in this order is a list, as it is in a
// PHP array. So an empty header is an empty list.
func (rd *reader) headerValue() any {
	keys := rd.header.Keys()
	for i, k := range keys {
		if k != strconv.Itoa(i) {
			return rd.header
		}
	}
	list := make([]any, len(keys))
	for i, k := range keys {
		list[i], _ = rd.header.Get(k)
	}
	return list
}

func (rd *reader) addHeaderTag(tag, value string) {
	switch tag {
	case "TITLE", "AUTHOR":
		key := model.MetadataTitle
		if tag == "AUTHOR" {
			key = model.MetadataAuthor
		}
		if value == "" {
			rd.s.DeleteMetadata(key)
		} else {
			rd.s.SetMetadata(key, value)
		}
	default:
		rd.header.Set(tag, value)
	}
}

// version1 reads the header lines and the script lines of SubViewer 1. firstLine is the line number of the first
// script line.
func (rd *reader) version1(header, script []string, firstLine int) ([]model.Cue, *model.Data, error) {
	delay, err := rd.version1Header(header)
	if err != nil {
		return nil, nil, err
	}
	cues, err := rd.version1Script(script, firstLine, delay)
	if err != nil {
		return nil, nil, err
	}
	return cues, model.NewData(model.Field{Key: "version", Value: int64(1)}, model.Field{Key: "header", Value: rd.headerValue()}), nil
}

// version1Header reads the header tags and returns the delay in seconds.
func (rd *reader) version1Header(lines []string) (int64, error) {
	var delay int64
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			continue
		}
		tag, rest, ok := splitTag(line)
		if !ok {
			err := &model.ParseError{Line: i + 1, Msg: `The line "` + line + `" is not a SubViewer 1 header tag.`}
			if err := rd.st.Fail(err, i+1, 0, []string{line}); err != nil {
				return 0, err
			}
			continue
		}
		tag = asciiUpper(strings.Trim(tag, markupcore.PHPTrimSet))
		value := strings.Trim(rest, markupcore.PHPTrimSet)
		if value == "" && i+1 < len(lines) && lines[i+1] != "" && !strings.HasPrefix(lines[i+1], "[") {
			i++
			value = lines[i]
		}
		if tag == "DELAY" {
			delay, value = phpInt(value), "0"
		}
		rd.addHeaderTag(tag, value)
	}
	return delay, nil
}

type scriptEntry struct {
	cue      model.Cue
	ended    bool
	timeLine string
	line     int
}

func (rd *reader) version1Script(lines []string, firstLine int, delay int64) ([]model.Cue, error) {
	entries, err := rd.scanScript(lines, firstLine, delay)
	if err != nil {
		return nil, err
	}
	var cues []model.Cue
	for i, e := range entries {
		if !e.ended {
			e.cue.End = rd.endBeforeNext(entries, i)
			cues = append(cues, e.cue)
			continue
		}
		start, end, err := rd.st.OrderedTimes(e.cue.Start, e.cue.End, e.line, i, []string{e.timeLine}, true)
		if err != nil {
			if err := rd.st.Fail(err.(*model.ParseError), e.line, i, []string{e.timeLine}); err != nil { //nolint:errorlint,forcetypeassert // OrderedTimes fails with a *ParseError
				return nil, err
			}
			continue
		}
		e.cue.Start, e.cue.End = start, end
		cues = append(cues, e.cue)
	}
	return cues, nil
}

// scanScript reads the time lines and the text lines of the script into entries. A time line ends the cue before
// it, and a text line after a time line starts a cue.
func (rd *reader) scanScript(lines []string, firstLine int, delay int64) ([]scriptEntry, error) {
	var entries []scriptEntry
	var pending *time.Duration
	var timeLine string
	var timeLineNumber int
	for i, line := range lines {
		if pending != nil {
			t := *pending
			pending = nil
			if line != "" && !v1TimeRegexp.MatchString(line) {
				entries = append(entries, scriptEntry{cue: model.Cue{Start: t, End: t, Lines: pipeLines(line)}, timeLine: timeLine, line: timeLineNumber})
				continue
			}
			endLastCue(entries, t)
		}
		m := v1TimeRegexp.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		timeLineNumber, timeLine = firstLine+i, line
		t, err := version1Time(m, delay, line, timeLineNumber)
		if err != nil {
			if err := rd.st.Fail(err, timeLineNumber, len(entries), []string{line}); err != nil {
				return nil, err
			}
			continue
		}
		pending = &t
	}
	if pending != nil {
		endLastCue(entries, *pending)
	}
	return entries, nil
}

// endBeforeNext returns the start of the next cue in the file that does not start before entry i. Without one, the
// cue lasts the last cue duration.
func (rd *reader) endBeforeNext(entries []scriptEntry, i int) time.Duration {
	for _, next := range entries[i+1:] {
		if next.cue.Start >= entries[i].cue.Start {
			return next.cue.Start
		}
	}
	return rd.st.LastCueEnd(entries[i].cue.Start)
}

// endLastCue ends the last entry at t, unless a time line already ended it.
func endLastCue(entries []scriptEntry, t time.Duration) {
	if n := len(entries); n > 0 && !entries[n-1].ended {
		entries[n-1].cue.End = t
		entries[n-1].ended = true
	}
}

// pipeLines splits a SubViewer 1 text line at |. It keeps empty parts, as PHP does.
func pipeLines(line string) []string {
	parts := strings.Split(line, "|")
	for i, p := range parts {
		parts[i] = markupcore.EscapeText(p)
	}
	return parts
}

// version1Time returns the time of the match of v1TimeRegexp plus the delay. The delay is clamped far outside the
// limit, so the sum cannot overflow.
func version1Time(m []string, delay int64, line string, number int) (time.Duration, *model.ParseError) {
	hours, _ := strconv.Atoi("0" + m[1])
	minutes, _ := strconv.Atoi(m[2])
	seconds, _ := strconv.Atoi(m[3])
	if minutes > 59 || seconds > 59 {
		return 0, &model.ParseError{Line: number, Msg: `The time "` + line + `" is not valid.`}
	}
	total := int64(hours)*3600 + int64(minutes)*60 + int64(seconds) + min(max(delay, -2*maxSeconds), 2*maxSeconds)
	if total > -maxSeconds && total < maxSeconds {
		return time.Duration(total) * time.Second, nil
	}
	text := line
	if delay != 0 {
		text += " with DELAY " + strconv.FormatInt(delay, 10)
	}
	_, err := parsing.BoundedTime(timecode.MaxTime, text, number)
	return 0, err.(*model.ParseError) //nolint:errorlint,forcetypeassert // BoundedTime fails with a *ParseError
}

// skippedCue is a cue with a bad timing line. Its warning comes when the next timing line or the file end comes, so
// that it holds the text lines too.
type skippedCue struct {
	line, block int
	lines       []string
	msg         string
}

func (rd *reader) version2(lines []string) ([]model.Cue, *model.Data, error) {
	for i, line := range lines {
		if line == "" {
			continue
		}
		if err := rd.version2Line(line, i+1); err != nil {
			return nil, nil, err
		}
	}
	rd.flush()

	data := model.NewData(model.Field{Key: "version", Value: int64(2)}, model.Field{Key: "header", Value: rd.headerValue()})
	if rd.hasStyle {
		data.Set("style", rd.style)
	}
	return rd.cues, data, nil
}

func (rd *reader) version2Line(line string, number int) error {
	next, nextSkipped, isTiming, err := rd.version2Timing(line, number, rd.block)
	if err != nil {
		return err
	}
	switch {
	case isTiming:
		rd.flush()
		rd.cue, rd.skipped = next, nextSkipped
		rd.block++
	case isStyleLine(line):
		if rd.cue == nil && !rd.hasStyle {
			rd.style, rd.hasStyle = line, true
		}
	case rd.skipped != nil:
		rd.skipped.lines = append(rd.skipped.lines, line)
	case rd.cue != nil:
		rd.cue.Lines = append(rd.cue.Lines, brLines(line)...)
	default:
		return rd.version2Header(line, number)
	}
	return nil
}

// flush ends the open cue and records the warning of a skipped cue.
func (rd *reader) flush() {
	if rd.cue != nil && len(rd.cue.Lines) > 0 {
		rd.cues = append(rd.cues, *rd.cue)
	}
	if sk := rd.skipped; sk != nil {
		rd.st.Warn(sk.msg, sk.line, sk.block, sk.lines, model.Skipped)
	}
}

// brLines splits a text line at [br] in lower case. It trims each part and drops the empty ones.
func brLines(line string) []string {
	var lines []string
	for part := range strings.SplitSeq(line, "[br]") {
		if part = strings.Trim(part, markupcore.PHPTrimSet); part != "" {
			lines = append(lines, markupcore.EscapeText(part))
		}
	}
	return lines
}

// version2Timing reads line as a timing line. False means that it is none. A line in the shape of a timing line with
// a bad or loose time fails in strict mode. Lenient mode gives a cue with a warning for a loose time, and a skipped
// cue for a bad time.
func (rd *reader) version2Timing(line string, number, block int) (*model.Cue, *skippedCue, bool, error) {
	var start, end time.Duration
	if m := v2TimeRegexp.FindStringSubmatch(line); m != nil {
		start, end = clock(m[1:5]), clock(m[5:9])
	} else if !looksLikeTimingLine(line) {
		return nil, nil, false, nil
	} else if !rd.st.Lenient() {
		err := &model.ParseError{Line: number, Msg: `The timing line "` + line + `" is not in the form hh:mm:ss.ff,hh:mm:ss.ff.`}
		return nil, nil, true, rd.st.Fail(err, number, block, []string{line})
	} else if loose, ok := rd.looseTimes(line, number, block); ok {
		start, end = loose[0], loose[1]
	} else {
		return nil, skip(line, number, block, `The timing line "`+line+`" has a time that is not valid.`), true, nil
	}

	start, end, err := rd.st.OrderedTimes(start, end, number, block, []string{line}, true)
	if err != nil {
		if !rd.st.Lenient() {
			return nil, nil, true, err
		}
		return nil, skip(line, number, block, err.(*model.ParseError).Msg), true, nil //nolint:errorlint,forcetypeassert // OrderedTimes fails with a *ParseError
	}
	return &model.Cue{Start: start, End: end}, nil, true, nil
}

func skip(line string, number, block int, msg string) *skippedCue {
	return &skippedCue{line: number, block: block, lines: []string{line}, msg: msg}
}

// looseTimes reads the 2 fields of a timing line with timecode.Loose and warns. False means that a field is no time,
// or that it rounds to the time limit.
func (rd *reader) looseTimes(line string, number, block int) ([2]time.Duration, bool) {
	first, second, _ := strings.Cut(line, ",")
	start, ok := timecode.Loose(first, ".", false)
	end, ok2 := timecode.Loose(second, ".", false)
	if !ok || !ok2 || start >= timecode.MaxTime || end >= timecode.MaxTime {
		return [2]time.Duration{}, false
	}
	rd.st.Warn(`The timing line "`+line+`" is not in the form hh:mm:ss.ff,hh:mm:ss.ff. The parser read it as `+
		timecode.Seconds(start)+" s to "+timecode.Seconds(end)+" s.", number, block, []string{line}, model.Repaired)
	return [2]time.Duration{start, end}, true
}

// clock returns the time of the groups hours, minutes, seconds and fraction of v2TimeRegexp. The fraction has 1 to 3
// digits, so "5" is 500 ms and "05" is 50 ms.
func clock(g []string) time.Duration {
	hours, _ := strconv.Atoi("0" + g[0])
	minutes, _ := strconv.Atoi(g[1])
	seconds, _ := strconv.Atoi(g[2])
	millis, _ := strconv.Atoi((g[3] + "00")[:3])
	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second +
		time.Duration(millis)*time.Millisecond
}

// looksLikeTimingLine reports whether line has 2 fields split at a comma, is no valid timing line, and has at least
// one field of the shape h:m:s or h:m:s.f. Loose and bad times such as 00:75:06.00 have this shape.
func looksLikeTimingLine(line string) bool {
	first, second, ok := strings.Cut(line, ",")
	return ok && !strings.Contains(second, ",") && !v2TimeRegexp.MatchString(line) &&
		(clockLikeRegexp.MatchString(first) || clockLikeRegexp.MatchString(second))
}

func isStyleLine(line string) bool {
	return strings.HasPrefix(line, "[") &&
		(strings.Contains(line, "[COLF]") || strings.Contains(line, "[SIZE]") || strings.Contains(line, "[FONT]") || strings.Contains(line, "[STYLE]"))
}

// isBlockTag reports whether tag marks a block of the SubViewer 2 header. The reader skips it.
func isBlockTag(tag string) bool {
	return tag == "INFORMATION" || tag == "END INFORMATION" || tag == "SUBTITLE"
}

func (rd *reader) version2Header(line string, number int) error {
	tag, rest, ok := splitTag(line)
	if !ok {
		err := &model.ParseError{Line: number, Msg: `The line "` + line + `" is neither a header tag nor a timing line.`}
		return rd.st.Fail(err, number, 0, []string{line})
	}
	tag = asciiUpper(strings.Trim(tag, markupcore.PHPTrimSet))
	if !isBlockTag(tag) {
		rd.addHeaderTag(tag, strings.Trim(rest, markupcore.PHPTrimSet))
	}
	return nil
}

// phpInt reads the number at the start of s as the PHP cast (int) does. It skips leading white space, reads a sign,
// digits, a fraction and an exponent, and cuts off the fraction. A finite value beyond the int64 range gives its limit, and one beyond the float range gives 0.
func phpInt(s string) int64 {
	s = strings.TrimLeft(s, " \t\n\r\v\f")
	s = s[:numberEnd(s)]
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	f, _ := strconv.ParseFloat(s, 64) // an empty string gives 0, a value beyond the float range gives +Inf or -Inf
	switch {
	case math.IsInf(f, 0):
		return 0 // PHP casts INF to 0, but a finite value beyond the int64 range to its limit
	case f >= math.MaxInt64:
		return math.MaxInt64
	case f <= math.MinInt64:
		return math.MinInt64
	}
	return int64(f)
}

// numberEnd returns the length of the number at the start of s, 0 when there is none.
func numberEnd(s string) int {
	i := 0
	if strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") {
		i++
	}
	digits := countDigits(s[i:])
	i += digits
	if strings.HasPrefix(s[i:], ".") {
		if fraction := countDigits(s[i+1:]); digits+fraction > 0 {
			i += 1 + fraction
			digits += fraction
		}
	}
	if digits == 0 {
		return 0
	}
	return i + exponentLength(s[i:])
}

// exponentLength returns the length of the exponent e, E, a sign and digits at the start of s, 0 when there is none.
func exponentLength(s string) int {
	if s == "" || s[0] != 'e' && s[0] != 'E' {
		return 0
	}
	sign := 0
	if len(s) > 1 && (s[1] == '+' || s[1] == '-') {
		sign = 1
	}
	if digits := countDigits(s[1+sign:]); digits > 0 {
		return 1 + sign + digits
	}
	return 0
}

func countDigits(s string) int {
	n := 0
	for n < len(s) && s[n] >= '0' && s[n] <= '9' {
		n++
	}
	return n
}
