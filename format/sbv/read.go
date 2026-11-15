package sbv

import (
	"io"
	"iter"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/textblock"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const (
	// space is the white space of PCRE \s. The block split leaves only spaces inside a line.
	space = `[\t\n\v\f\r ]*`
	// looseTime is a time of the loose timing line: any hour digits, 1 or 2 minute and second digits, and a
	// fraction of 1 to 4 digits after ".", "," or ":".
	looseTime = `(\d+):([0-5]?\d):([0-5]?\d)[.,:](\d{1,4})`
)

var (
	timingLineRegexp      = regexp.MustCompile(`^\d+:\d\d:\d\d\.\d+,`)
	strictTimingRegexp    = regexp.MustCompile(`^\d+:[0-5]\d:[0-5]\d\.\d{3}` + space + `,` + space + `\d+:[0-5]\d:[0-5]\d\.\d{3}$`)
	looseTimingLineRegexp = regexp.MustCompile(`^` + looseTime + space + `[.,]` + space + looseTime + `$`)
	timeRegexp            = regexp.MustCompile(`^(\d+):([0-5]\d):([0-5]\d)\.(\d{3})$`)
)

type reader struct {
	st    *parsing.State
	rules textblock.Rules
}

// Read reads SBV text. It expects UTF-8, as stbox.Parse passes it.
func Read(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	st, content, ok, err := parsing.ReadText(r, model.SBV, common, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}
	rd := &reader{st: st}
	rd.rules = textblock.Rules{IsTimingLine: rd.isTimingLine}
	var cues []model.Cue
	index := 0
	for b := range rd.blocks(string(content)) {
		parsed, err := rd.parseBlock(b, index)
		if err != nil {
			return nil, nil, err
		}
		cues = append(cues, parsed...)
		index++
	}
	s.AddCues(cues...)
	return s, st.Warnings(), nil
}

// blocks yields the blocks of content after the join of cue text blocks, which warns in lenient mode.
func (rd *reader) blocks(content string) iter.Seq[textblock.Block] {
	return textblock.JoinCueTextBlocks(textblock.SplitAtEmptyLines(parsing.Lines(content)), rd.rules, rd.st)
}

func (rd *reader) isTimingLine(line string) bool {
	return isTimingLine(line, rd.st.Lenient())
}

// isTimingLine reports whether the reader reads line as a timing line. Lenient mode also reads a loose timing line.
func isTimingLine(line string, lenient bool) bool {
	return timingLineRegexp.MatchString(line) || lenient && looseTimingLineRegexp.MatchString(line)
}

// parseBlock returns the cues of block b, number index, split before each timing line. Lenient mode skips a
// part that fails, and warns for each split and each skip.
func (rd *reader) parseBlock(b textblock.Block, index int) ([]model.Cue, error) {
	return rd.rules.ReadParts(rd.st, b, index, rd.parseCue)
}

// parseCue parses the lines of one cue, whose timing line is on line.
func (rd *reader) parseCue(lines []string, index, line int) (model.Cue, error) {
	timing := lines[0]
	start, end, loose, err := rd.timing(timing, index, line)
	if err != nil {
		return model.Cue{}, err
	}
	if start, end, err = rd.st.OrderedTimes(start, end, line, index, lines, true); err != nil {
		return model.Cue{}, err
	}
	c := model.Cue{Start: start, End: end}
	for _, l := range lines[1:] {
		c.AddLine(markupcore.EscapeText(l))
	}
	if loose {
		rd.st.Warn("Block #"+strconv.Itoa(index)+" has a timing line with other separators or fraction digits than SBV uses. The parser read it.",
			line, index, lines, model.Repaired)
	}
	return c, nil
}

// timing reads a timing line. Lenient mode also reads a loose line, which loose reports.
func (rd *reader) timing(text string, index, line int) (start, end time.Duration, loose bool, err error) {
	if rd.st.Lenient() && !strictTimingRegexp.MatchString(text) {
		if m := looseTimingLineRegexp.FindStringSubmatch(text); m != nil {
			if start, err = looseDuration(m[1:5], line); err != nil {
				return 0, 0, false, err
			}
			end, err = looseDuration(m[5:9], line)
			return start, end, err == nil, err
		}
	}
	first, second, found := strings.Cut(text, ",")
	if !found || strings.Contains(second, ",") {
		return 0, 0, false, &model.ParseError{Line: line, Msg: "Block #" + strconv.Itoa(index) +
			" has no timing line on its first line. The line is " + parsing.Quote(text) + "."}
	}
	if start, err = strictDuration(first, line); err != nil {
		return 0, 0, false, err
	}
	end, err = strictDuration(second, line)
	return start, end, false, err
}

func strictDuration(text string, line int) (time.Duration, error) {
	text = strings.Trim(text, markupcore.TrimSet)
	m := timeRegexp.FindStringSubmatch(text)
	if m == nil {
		return 0, parsing.InvalidTime(text, line)
	}
	return parsing.BoundedTime(timecode.HMS(m[1], m[2], m[3])+time.Duration(atoi(m[4]))*time.Millisecond, text, line)
}

// looseDuration reads the hour, minute, second and fraction digits of a loose time. The fraction rounds half up
// to whole milliseconds.
func looseDuration(fields []string, line int) (time.Duration, error) {
	tenthsOfMillis := atoi(fields[3] + strings.Repeat("0", 4-len(fields[3])))
	d := timecode.HMS(fields[0], fields[1], fields[2]) + time.Duration((tenthsOfMillis+5)/10)*time.Millisecond
	return parsing.BoundedTime(d, fields[0]+":"+fields[1]+":"+fields[2]+"."+fields[3], line)
}

// atoi reads a string of at most 9 digits.
func atoi(digits string) int {
	n := 0
	for i := range len(digits) {
		n = 10*n + int(digits[i]-'0')
	}
	return n
}
