package srt

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/blocks"
	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	arrow = "-->"
	// space is the white space of PCRE \s. The pre-pass and the block split leave only spaces inside a line.
	space      = "\t\n\v\f\r "
	phpTrimSet = " \t\n\r\x00\v"
	vttKey     = string(model.WebVTT)
)

var (
	strictTimingLineRegexp  = regexp.MustCompile(`^\d+:\d\d:\d\d[^\t\n\v\f\r ]*?[\t\n\v\f\r ]*-->`)
	lenientTimingLineRegexp = regexp.MustCompile(`^\d+:\d{1,2}[^\t\n\v\f\r ]*?[\t\n\v\f\r ]*-+>`)
	timeRegexp              = regexp.MustCompile(`^(\d{1,3}):([0-5]\d):([0-5]\d)(?:[,.](\d{1,3})|:(\d{3}))?$`)
	coordinatesRegexp       = regexp.MustCompile(`^X1:(\d+)[\t\n\v\f\r ]+X2:(\d+)[\t\n\v\f\r ]+Y1:(\d+)[\t\n\v\f\r ]+Y2:(\d+)$`)
	settingRegexp           = regexp.MustCompile(`^(vertical|line|position|size|align):([^\t\n\v\f\r ]+)$`)
)

// fullWidth replaces the full-width delimiters that Chinese and Japanese tools write in timing lines.
var fullWidth = strings.NewReplacer("\uff1a", ":", "\uff0c", ",", "\uff0e", ".", "\u3002", ".") //nolint:gochecknoglobals // a replacer holds no state

type reader struct {
	st    *parsing.State
	rules blocks.Rules
}

// timing holds the parts of a timing line.
type timing struct {
	start, arrow, end string
	// rest is the text after the end time.
	rest      string
	fullWidth bool
}

// looseTime is a time that lenient mode read with timecode.Loose.
type looseTime struct {
	text string
	d    time.Duration
}

// Read reads SubRip text. It expects UTF-8, as stbox.Parse passes it.
func Read(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.SubRip, err)
	}
	st := parsing.NewState(model.SubRip, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}
	rd := newReader(st)
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

func newReader(st *parsing.State) *reader {
	rd := &reader{st: st}
	rd.rules = blocks.Rules{IsTimingLine: rd.isTimingLine, HasCueNumbers: true}
	return rd
}

// blocks yields the blocks of content after the join of cue text blocks, which warns in lenient mode.
func (rd *reader) blocks(content string) iter.Seq[blocks.Block] {
	onJoin := func(block, line int, lines []string) {
		if rd.st.Lenient() {
			rd.st.Warn(blocks.JoinWarning(block, line), line, block, lines, model.Repaired)
		}
	}
	return blocks.JoinCueTextBlocks(blocks.SplitAtEmptyLines(lines(content)), rd.rules, nil, onJoin)
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

func (rd *reader) isTimingLine(line string) bool {
	if rd.st.Lenient() {
		return lenientTimingLineRegexp.MatchString(fullWidth.Replace(line))
	}
	return strictTimingLineRegexp.MatchString(line)
}

// parseBlock returns the cues of block b, number index, split before each timing line. Lenient mode skips a
// part that fails, and warns for each split and each skip.
func (rd *reader) parseBlock(b blocks.Block, index int) ([]model.Cue, error) {
	parts := rd.rules.SplitAtTimingLines(b.Lines)
	if rd.st.Lenient() {
		for _, p := range parts {
			if p.Line > 0 {
				line := b.Line + p.Line
				msg := "Block #" + strconv.Itoa(index) + " has no empty line before line " + strconv.Itoa(line) + ". The parser split the block there."
				rd.st.Warn(msg, line, index, b.Lines, model.Repaired)
			}
		}
	}
	cues := make([]model.Cue, 0, len(parts))
	for _, p := range parts {
		line := b.Line + p.Line
		c, err := rd.parsePart(p.Lines, index, line)
		if err == nil {
			cues = append(cues, c)
			continue
		}
		var perr *model.ParseError
		if !errors.As(err, &perr) {
			return nil, err
		}
		if perr.Line > 0 {
			line = perr.Line
		}
		if err := rd.st.Fail(perr, line, index, p.Lines); err != nil {
			return nil, err
		}
	}
	return cues, nil
}

// parsePart parses one part of a block, which starts at line. Lenient mode reads a part that starts with its
// timing line as if a cue number came before it.
func (rd *reader) parsePart(part []string, index, line int) (model.Cue, error) {
	if !rd.st.Lenient() || !rd.isTimingLine(part[0]) {
		return rd.parseCue(part, part, index, line)
	}
	c, err := rd.parseCue(append([]string{"0"}, part...), part, index, line-1)
	if err != nil {
		return model.Cue{}, err
	}
	msg := "Block #" + strconv.Itoa(index) + " has no cue number on line " + strconv.Itoa(line) + ". The parser read the cue without it."
	rd.st.Warn(msg, line, index, part, model.Repaired)
	return c, nil
}

// parseCue parses the lines of one cue: the cue number at line, the timing line and the text.
// Warnings hold shown, the lines as read.
func (rd *reader) parseCue(lines, shown []string, index, line int) (model.Cue, error) {
	block := "Block #" + strconv.Itoa(index)
	if !blocks.IsNumeric(lines[0]) {
		return model.Cue{}, &model.ParseError{Line: line, Msg: block + " has no cue number on its first line. The line is " + parsing.Quote(lines[0]) + "."}
	}
	if len(lines) < 2 {
		return model.Cue{}, &model.ParseError{Line: line, Msg: block + " has no timing line on its second line."}
	}
	line++
	t, ok := rd.readTimingLine(lines[1])
	if !ok {
		return model.Cue{}, &model.ParseError{Line: line, Msg: block + " has no timing line on its second line. The line is " + parsing.Quote(lines[1]) + "."}
	}

	var loose []looseTime
	start, err := rd.time(t.start, line, &loose)
	if err != nil {
		return model.Cue{}, err
	}
	end, err := rd.time(t.end, line, &loose)
	if err != nil {
		return model.Cue{}, err
	}
	if start, end, err = rd.st.OrderedTimes(start, end, line, index, shown, true); err != nil {
		return model.Cue{}, err
	}

	c := model.Cue{Start: start, End: end}
	text, numpad := cueText(lines[2:])
	c.SetText(text)
	c.Alignment, _ = model.AlignmentFromNumpad(numpad)
	if err := rd.applyTextAfterEndTime(&c, t.rest, numpad, index, line, shown); err != nil {
		return model.Cue{}, err
	}
	rd.warnTimingRepairs(t, loose, index, line, shown)
	return c, nil
}

// warnTimingRepairs warns for the loose times, the full-width delimiters and the arrow of a timing line.
func (rd *reader) warnTimingRepairs(t timing, loose []looseTime, index, line int, shown []string) {
	block := "Block #" + strconv.Itoa(index)
	for _, lt := range loose {
		rd.st.Warn(block+` has the time "`+lt.text+`", which is not in the form hh:mm:ss,mmm. The parser read it as `+timecode.Seconds(lt.d)+" s.",
			line, index, shown, model.Repaired)
	}
	if t.fullWidth {
		rd.st.Warn(block+" has full-width delimiters in its timing line. The parser read them as ASCII.", line, index, shown, model.Repaired)
	}
	if t.arrow != arrow {
		rd.st.Warn(block+` has the arrow "`+t.arrow+`" in its timing line. The parser read it as "-->".`, line, index, shown, model.Repaired)
	}
}

// readTimingLine splits a timing line at the first arrow whose text before it, without trailing white space,
// does not end in "-". Strict mode reads only "-->". Lenient mode reads any run of "-" before ">" and the
// full-width delimiters.
func (rd *reader) readTimingLine(line string) (timing, bool) {
	text := line
	if rd.st.Lenient() {
		text = fullWidth.Replace(line)
	}
	for i := 0; i <= len(text); i++ {
		if i > 0 && text[i-1] == '-' {
			continue
		}
		k := i
		for k < len(text) && strings.IndexByte(space, text[k]) >= 0 {
			k++
		}
		n := rd.arrowLength(text[k:])
		if n == 0 {
			continue
		}
		after := strings.TrimLeft(text[k+n:], space)
		end, rest := after, ""
		if j := strings.IndexAny(after, space); j >= 0 {
			end, rest = after[:j], strings.TrimLeft(after[j:], space)
		}
		return timing{start: text[:i], arrow: text[k : k+n], end: end, rest: rest, fullWidth: text != line}, true
	}
	return timing{}, false
}

// arrowLength returns the length of the arrow at the start of s, or 0.
func (rd *reader) arrowLength(s string) int {
	if !rd.st.Lenient() {
		if strings.HasPrefix(s, arrow) {
			return len(arrow)
		}
		return 0
	}
	n := 0
	for n < len(s) && s[n] == '-' {
		n++
	}
	if n == 0 || n == len(s) || s[n] != '>' {
		return 0
	}
	return n + 1
}

// time reads a time of the timing line at line. Lenient mode also reads the forms of timecode.Loose and
// adds them to loose, once per text.
func (rd *reader) time(text string, line int, loose *[]looseTime) (time.Duration, error) {
	text = strings.Trim(text, phpTrimSet)
	if m := timeRegexp.FindStringSubmatch(text); m != nil {
		return time.Duration(atoi(m[1]))*time.Hour + time.Duration(atoi(m[2]))*time.Minute +
			time.Duration(atoi(m[3]))*time.Second + fraction(m[4]+m[5]), nil
	}
	// A last field of 1 digit without a fraction is a time that the end of the file cut off.
	cutOff := len(text) >= 2 && text[len(text)-2] == ':' && isDigit(text[len(text)-1])
	if rd.st.Lenient() && !cutOff {
		if d, ok := timecode.Loose(text, ",.:", true); ok {
			if !containsTime(*loose, text) {
				*loose = append(*loose, looseTime{text: text, d: d})
			}
			return d, nil
		}
	}
	return 0, &model.ParseError{Line: line, Msg: `The time "` + text + `" is not valid.`}
}

func containsTime(loose []looseTime, text string) bool {
	for _, lt := range loose {
		if lt.text == text {
			return true
		}
	}
	return false
}

// fraction returns the time of the digits after the decimal separator: "5" is 500 ms and "05" is 50 ms.
func fraction(digits string) time.Duration {
	ms := 0
	for i := range 3 {
		ms *= 10
		if i < len(digits) {
			ms += int(digits[i] - '0')
		}
	}
	return time.Duration(ms) * time.Millisecond
}

// applyTextAfterEndTime stores the text after the end time as SubRip coordinates or as WebVTT cue settings.
// Strict mode fails on unknown text. Lenient mode ignores it and warns.
func (rd *reader) applyTextAfterEndTime(c *model.Cue, rest string, numpad, index, line int, shown []string) error {
	if coordinates, ok := readCoordinates(rest); ok {
		return setFormatData(c, formatDataKey, model.NewData(model.Field{Key: "coordinates", Value: coordinates}))
	}
	settings, unknown := cueSettings(rest)
	if unknown != "" && !rd.st.Lenient() {
		return &model.ParseError{Line: line, Msg: `The text "` + unknown + `" after the end time is not valid.`}
	}
	if settings.Len() > 0 {
		if numpad != 0 {
			settings.Set("alignment", int64(numpad))
		}
		if err := setFormatData(c, vttKey, settings); err != nil {
			return err
		}
	}
	if unknown != "" {
		rd.st.Warn("Block #"+strconv.Itoa(index)+` has the unknown text "`+unknown+`" after the end time. The parser ignored it.`,
			line, index, shown, model.Repaired)
	}
	return nil
}

func setFormatData(c *model.Cue, key string, d *model.Data) error {
	if err := c.SetFormatData(key, d); err != nil {
		return fmt.Errorf("%s cue data: %w", key, err)
	}
	return nil
}

// readCoordinates reads text such as "X1:10 X2:20 Y1:30 Y2:40".
func readCoordinates(text string) (*model.Data, bool) {
	m := coordinatesRegexp.FindStringSubmatch(text)
	if m == nil {
		return nil, false
	}
	return model.NewData(
		model.Field{Key: "x1", Value: saturatedInt(m[1])},
		model.Field{Key: "x2", Value: saturatedInt(m[2])},
		model.Field{Key: "y1", Value: saturatedInt(m[3])},
		model.Field{Key: "y2", Value: saturatedInt(m[4])},
	), true
}

// cueSettings returns the WebVTT cue settings of text in their first place with their last value, and the
// other tokens joined with 1 space. Converters such as yt-dlp copy the settings of a WebVTT cue there.
func cueSettings(text string) (*model.Data, string) {
	settings := model.NewData()
	var unknown []string
	for _, token := range strings.FieldsFunc(text, func(r rune) bool { return r < 0x80 && strings.ContainsRune(space, r) }) {
		if m := settingRegexp.FindStringSubmatch(token); m != nil {
			settings.Set(m[1], m[2])
		} else {
			unknown = append(unknown, token)
		}
	}
	return settings, strings.Join(unknown, " ")
}

// saturatedInt reads digits as PHP (int) does: a value above the int64 range becomes the maximum.
func saturatedInt(digits string) int64 {
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return math.MaxInt64
	}
	return n
}

func atoi(digits string) int {
	n := 0
	for i := range len(digits) {
		n = 10*n + int(digits[i]-'0')
	}
	return n
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
