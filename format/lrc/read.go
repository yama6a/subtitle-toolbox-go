package lrc

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/blocks"
	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const wordTimePattern = `<` + timestampPattern + `>`

var (
	timeTagRegexp         = regexp.MustCompile(timeTagPattern)
	timeTagsRegexp        = regexp.MustCompile(`^(?:` + timeTagPattern + `)+`)
	brokenTimeTagRegexp   = regexp.MustCompile(`^\[[ \t]*\d`)
	idTagRegexp           = regexp.MustCompile(`^\[([A-Za-z][A-Za-z0-9_]*|#):(.*)\]$`)
	offsetRegexp          = regexp.MustCompile(`^[+-]?\d+$`)
	wordTimeRegexp        = regexp.MustCompile(wordTimePattern)
	spacesAfterWordTime   = regexp.MustCompile(wordTimePattern + `[ \t]+`)
	spacesBeforeFinalTime = regexp.MustCompile(`[ \t]+` + wordTimePattern + `$`)
)

const maxMillis = int64(timecode.MaxTime / time.Millisecond)

// entry is one time tag on the timeline. cue is the index of its cue, or -1 for a time tag without text.
type entry struct {
	time time.Duration
	cue  int
}

type reader struct {
	st       *parsing.State
	offset   int64
	cues     []model.Cue
	timeline []entry
	comments []parsing.ParsedComment
	idTags   *model.Data
	spaced   bool
}

// Read reads LRC and enhanced LRC from UTF-8 text.
func Read(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.LRC, err)
	}
	st := parsing.NewState(model.LRC, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}
	text := string(content)
	if st.Lenient() {
		warnBrokenTimeTags(st, text)
	}
	lines := cleanLines(text)
	rd := &reader{st: st, offset: findOffset(lines), idTags: &model.Data{}}
	if err := rd.readLines(s, lines); err != nil {
		return nil, nil, err
	}
	if err := rd.setFileData(s); err != nil {
		return nil, nil, err
	}
	if err := rd.assignEnds(); err != nil {
		return nil, nil, err
	}
	parsing.AddParsed(s, rd.cues, rd.comments)
	if err := st.NoCues(s.Len(), func() (int, bool) { return textLineWithoutCues(text) }); err != nil {
		return nil, nil, err
	}
	return s, st.Warnings(), nil
}

func cleanLine(line string) string {
	return strings.Trim(blocks.CollapseSpaces(line), markupcore.PHPTrimSet)
}

// cleanLines returns the lines of content that are not empty after cleanLine. A line of spaces goes too, so it
// never becomes the text of a time tag before it, as it does in PHP.
func cleanLines(content string) []string {
	var lines []string
	for _, line := range parsing.Lines(content) {
		if line = cleanLine(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func warnBrokenTimeTags(st *parsing.State, content string) {
	block := 0
	for n, line := range parsing.Lines(content) {
		if line = cleanLine(line); line == "" {
			continue
		}
		if brokenTimeTagRegexp.MatchString(line) && !timeTagsRegexp.MatchString(line) {
			st.Warn(`The line "`+line+`" has a time tag that is not valid.`, n+1, block, []string{line}, model.Skipped)
		}
		block++
	}
}

func textLineWithoutCues(content string) (int, bool) {
	for n, line := range parsing.Lines(content) {
		if line = cleanLine(line); line != "" && isPlainText(line) {
			return n + 1, true
		}
	}
	return 0, false
}

func isPlainText(line string) bool {
	return !timeTagsRegexp.MatchString(line) && !idTagRegexp.MatchString(line) && !brokenTimeTagRegexp.MatchString(line)
}

// findOffset clamps a value beyond int64, as PHP (int) does.
func findOffset(lines []string) int64 {
	for _, line := range lines {
		m := idTagRegexp.FindStringSubmatch(line)
		if m == nil || markupcore.ASCIILower(m[1]) != "offset" {
			continue
		}
		if value := strings.Trim(m[2], markupcore.PHPTrimSet); offsetRegexp.MatchString(value) {
			offset, _ := strconv.ParseInt(value, 10, 64)
			return offset
		}
	}
	return 0
}

func (rd *reader) readLines(s *model.Subtitle, lines []string) error {
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		loc := timeTagsRegexp.FindStringIndex(line)
		if loc == nil {
			rd.addIDTag(s, line)
			continue
		}
		text := line[loc[1]:]
		if strings.Trim(text, markupcore.PHPTrimSet) == "" && i+1 < len(lines) && isPlainText(lines[i+1]) {
			i++
			text = lines[i]
		}
		err := rd.readTimedLine(line[:loc[1]], text)
		var perr *model.ParseError
		if errors.As(err, &perr) {
			err = rd.st.Fail(perr, model.NoLine, i, []string{line})
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (rd *reader) readTimedLine(tags, text string) error {
	text, err := rd.convertWordTimes(strings.Trim(text, markupcore.PHPTrimSet))
	if err != nil {
		return err
	}
	var starts []time.Duration
	for _, m := range timeTagRegexp.FindAllStringSubmatchIndex(tags, -1) {
		start, err := rd.tagTime(tags, m)
		if err != nil {
			return err
		}
		starts = append(starts, start)
	}
	for _, start := range starts {
		cue := -1
		if text != "" {
			cue = len(rd.cues)
			rd.cues = append(rd.cues, model.Cue{Start: start, Lines: []string{text}})
		}
		rd.timeline = append(rd.timeline, entry{time: start, cue: cue})
	}
	return nil
}

func (rd *reader) convertWordTimes(text string) (string, error) {
	if !strings.Contains(text, "<") {
		return markupcore.EscapeText(text), nil
	}
	dropped := false
	text = spacesAfterWordTime.ReplaceAllStringFunc(text, func(m string) string {
		dropped = true
		return strings.TrimRight(m, " \t")
	})
	if m := spacesBeforeFinalTime.FindStringSubmatchIndex(text); m != nil {
		dropped = true
		text = text[:m[0]] + strings.TrimLeft(text[m[0]:], " \t")
	}
	rd.spaced = rd.spaced || dropped

	var b strings.Builder
	copied := 0
	for _, m := range wordTimeRegexp.FindAllStringSubmatchIndex(text, -1) {
		d, err := rd.tagTime(text, m)
		if err != nil {
			return "", err
		}
		b.WriteString(markupcore.EscapeText(text[copied:m[0]]))
		b.WriteString("<" + markupcore.CoreTimestamp(d) + ">")
		copied = m[1]
	}
	b.WriteString(markupcore.EscapeText(text[copied:]))
	return b.String(), nil
}

// tagTime computes in whole milliseconds, which is exact, so the bound check needs no rounding first.
func (rd *reader) tagTime(text string, m []int) (time.Duration, error) {
	group := func(i int) string {
		if m[2*i] < 0 {
			return ""
		}
		return text[m[2*i]:m[2*i+1]]
	}
	hours, minutes := atoi(group(1)), atoi(group(2))
	if group(3) != "" {
		minutes = atoi(group(3))
	}
	ms := ((hours*60+minutes)*60 + atoi(group(4))) * 1000
	switch fraction := group(5); len(fraction) {
	case 1:
		ms += atoi(fraction) * 100
	case 2:
		ms += atoi(fraction) * 10
	case 3:
		ms += atoi(fraction)
	}
	tag := text[m[0]:m[1]]
	switch {
	case rd.offset == 0:
	case rd.offset <= -maxMillis:
		return parsing.BoundedTime(timecode.MaxTime, tag, model.NoLine)
	default:
		ms = max(0, ms-rd.offset)
	}
	return parsing.BoundedTime(time.Duration(min(ms, maxMillis))*time.Millisecond, tag, model.NoLine)
}

func atoi(digits string) int64 {
	n, _ := strconv.ParseInt(digits, 10, 64)
	return n
}

func (rd *reader) addIDTag(s *model.Subtitle, line string) {
	m := idTagRegexp.FindStringSubmatch(line)
	if m == nil {
		return
	}
	tag, value := markupcore.ASCIILower(m[1]), strings.Trim(m[2], markupcore.PHPTrimSet)
	if tag == "#" {
		rd.comments = append(rd.comments, parsing.ParsedComment{Text: value, CuesBefore: len(rd.cues)})
		return
	}
	if i := slices.IndexFunc(metadataTags, func(t struct{ tag, key string }) bool { return t.tag == tag }); i >= 0 {
		s.SetMetadata(metadataTags[i].key, value)
		return
	}
	if tag != "offset" || !offsetRegexp.MatchString(value) {
		rd.idTags.Set(tag, value)
	}
}

func (rd *reader) setFileData(s *model.Subtitle) error {
	data := &model.Data{}
	if rd.idTags.Len() > 0 {
		data.Set("idTags", rd.idTags)
	}
	if rd.spaced {
		data.Set("spacedWordTimes", true)
	}
	if err := s.SetFormatData(formatDataKey, data); err != nil {
		return fmt.Errorf("%s file data: %w", model.LRC, err)
	}
	return nil
}

func (rd *reader) assignEnds() error {
	slices.SortStableFunc(rd.timeline, func(a, b entry) int { return cmp.Compare(a.time, b.time) })
	endLine := model.NewData(model.Field{Key: "endLine", Value: true})
	for i, e := range rd.timeline {
		if e.cue < 0 {
			continue
		}
		c := &rd.cues[e.cue]
		if i+1 == len(rd.timeline) {
			c.End = rd.st.LastCueEnd(e.time)
			continue
		}
		next := rd.timeline[i+1]
		c.End = next.time
		if next.cue < 0 {
			if err := c.SetFormatData(formatDataKey, endLine); err != nil {
				return fmt.Errorf("%s cue data: %w", model.LRC, err)
			}
		}
	}
	return nil
}
