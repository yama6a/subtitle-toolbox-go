package transcript

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

var (
	htmlTimeRegexp = regexp.MustCompile(`^(?:(\d+):)?(\d+):([0-5]?\d)(?:[.,](\d+))?$`)
	lineBreakTag   = regexp.MustCompile(`(?i)<br[ \t\n\v\f\r]*/?>`)
)

// maxClockDigits is the digit count above which hours or minutes reach 100000 hours alone.
const maxClockDigits = 9

type htmlParagraph struct {
	line       int
	speaker    string
	hasSpeaker bool
	timeText   string
	timeLine   int
	hasTime    bool
	lines      []string
}

// ReadHTML reads the cite, time and p elements of an HTML transcript from r, without an HTML parser. Other markup
// is ignored and no html element is needed. A time and its paragraphs make one cue. Each cue ends at the next later
// start, and the last one lasts the last cue duration. A cite names the speaker of the next paragraph.
func ReadHTML(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.HTMLTranscript, err)
	}
	st := parsing.NewState(model.HTMLTranscript, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}
	text := strings.ReplaceAll(strings.ReplaceAll(string(content), "\r\n", "\n"), "\r", "\n")

	cues, err := htmlCues(st, htmlParagraphs(text))
	if err != nil {
		return nil, nil, err
	}
	starts := make([]time.Duration, len(cues))
	for i, c := range cues {
		starts[i] = c.Start
	}
	for i, next := range laterStarts(starts) {
		if next >= 0 {
			cues[i].End = cues[next].Start
		} else {
			cues[i].End = st.LastCueEnd(cues[i].Start)
		}
	}
	s.AddCues(cues...)
	return s, st.Warnings(), nil
}

// laterStarts returns, for each start, the index of the first start after it in the list that is later than it, or
// -1 for none. One pass from the back finds every index.
func laterStarts(starts []time.Duration) []int {
	next := make([]int, len(starts))
	var stack []int // indexes after i, whose starts rise from the top of the stack to the bottom
	for i := len(starts) - 1; i >= 0; i-- {
		for len(stack) > 0 && starts[stack[len(stack)-1]] <= starts[i] {
			stack = stack[:len(stack)-1]
		}
		next[i] = -1
		if len(stack) > 0 {
			next[i] = stack[len(stack)-1]
		}
		stack = append(stack, i)
	}
	return next
}

// htmlCues returns a cue without an end for each paragraph that has lines. A paragraph with a bad time goes to Fail.
func htmlCues(st *parsing.State, paragraphs []htmlParagraph) ([]model.Cue, error) {
	var cues []model.Cue
	for i, p := range paragraphs {
		if len(p.lines) == 0 {
			continue
		}
		start, perr := p.start()
		if perr != nil {
			block := p.lines[0]
			if p.hasTime {
				block = p.timeText
			}
			if err := st.Fail(perr, perr.Line, i, []string{block}); err != nil {
				return nil, err
			}
			continue
		}
		c := model.Cue{Start: start, Speaker: p.speaker}
		for _, l := range p.lines {
			c.AddLine(l)
		}
		cues = append(cues, c)
	}
	return cues, nil
}

// htmlParagraphs groups the elements into paragraphs. A cite after a speaker or text, and a time after a time or
// text, start a new paragraph.
func htmlParagraphs(content string) []htmlParagraph {
	var out []htmlParagraph
	line, last := 1, 0
	for _, el := range scanElements(content) {
		line += strings.Count(content[last:el.offset], "\n")
		last = el.offset
		if len(out) == 0 || out[len(out)-1].endsBefore(el.name) {
			out = append(out, htmlParagraph{line: line})
		}
		cur := &out[len(out)-1]
		switch el.name {
		case nameCite:
			cur.speaker = strings.Trim(strings.TrimRight(htmlText(el.inner), ":"), markupcore.PHPTrimSet)
			cur.hasSpeaker = true
		case nameTime:
			cur.timeText, cur.timeLine, cur.hasTime = htmlText(el.inner), line, true
		default:
			cur.lines = append(cur.lines, htmlLines(el.inner)...)
		}
	}
	return out
}

// endsBefore reports whether an element with this name starts a new paragraph.
func (p htmlParagraph) endsBefore(name int) bool {
	switch name {
	case nameCite:
		return p.hasSpeaker || len(p.lines) > 0
	case nameTime:
		return p.hasTime || len(p.lines) > 0
	default:
		return false
	}
}

// htmlText removes the tags, decodes the entities and joins each run of space, tab, CR and LF to one space.
func htmlText(html string) string {
	return strings.Trim(collapseSpace(markupcore.PlainText(html)), markupcore.PHPTrimSet)
}

// htmlLines splits the inner HTML of a p at each br and returns the escaped text of each part, without empty ones.
func htmlLines(inner string) []string {
	var out []string
	for _, part := range lineBreakTag.Split(inner, -1) {
		if t := htmlText(part); t != "" {
			out = append(out, markupcore.EscapeText(t))
		}
	}
	return out
}

func (p htmlParagraph) start() (time.Duration, *model.ParseError) {
	if !p.hasTime {
		return 0, &model.ParseError{Line: p.line, Msg: "The paragraph has no <time>."}
	}
	m := htmlTimeRegexp.FindStringSubmatch(p.timeText)
	if m == nil {
		return 0, &model.ParseError{Line: p.timeLine, Msg: `The time "` + p.timeText + `" is not valid.`}
	}
	hours, minutes := strings.TrimLeft(m[1], "0"), strings.TrimLeft(m[2], "0")
	if len(hours) > maxClockDigits || len(minutes) > maxClockDigits {
		return 0, parsing.OverLimit(p.timeText, p.timeLine)
	}
	h, _ := strconv.ParseInt("0"+hours, 10, 64)
	mi, _ := strconv.ParseInt("0"+minutes, 10, 64)
	sec, _ := strconv.ParseInt(m[3], 10, 64)
	millis := ((h*60+mi)*60+sec)*1000 + fractionMillis(m[4])
	if millis >= int64(timecode.MaxTime/time.Millisecond) {
		return 0, parsing.OverLimit(p.timeText, p.timeLine)
	}
	return time.Duration(millis) * time.Millisecond, nil
}

// fractionMillis returns the milliseconds of the decimal fraction digits, rounded half up.
func fractionMillis(digits string) int64 {
	var n int64
	for i := range 3 {
		n *= 10
		if i < len(digits) {
			n += int64(digits[i] - '0')
		}
	}
	if len(digits) > 3 && digits[3] >= '5' {
		n++
	}
	return n
}
