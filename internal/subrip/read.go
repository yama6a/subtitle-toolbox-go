// Package subrip holds the SubRip block reader and writer that format/srt and stream share.
package subrip

import (
	"fmt"
	"iter"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/blocks"
	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/phpkey"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	arrow = "-->"
	// space is the white space of PCRE \s. The pre-pass and the block split leave only spaces inside a line.
	space         = "\t\n\v\f\r "
	vttKey        = string(model.WebVTT)
	formatDataKey = "srt"
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

// Reader reads the blocks of a SubRip file one at a time.
type Reader struct {
	st    *parsing.State
	rules blocks.Rules
	cut   func() (int, bool)
}

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

// NewReader returns a reader that records its warnings and reads its mode from st.
func NewReader(st *parsing.State) *Reader {
	rd := &Reader{st: st}
	rd.rules = blocks.Rules{IsTimingLine: rd.isTimingLine, HasCueNumbers: true}
	return rd
}

// CutAt sets a function that returns the last line read and true when an error ended the input.
// Blocks then drops the block that this error cut.
func (rd *Reader) CutAt(cut func() (int, bool)) {
	rd.cut = cut
}

// Blocks yields the blocks of lines after the join of cue text blocks, which warns in lenient mode.
// The keys of lines are 0-based line numbers.
func (rd *Reader) Blocks(lines iter.Seq2[int, string]) iter.Seq[blocks.Block] {
	onJoin := func(block, line int, lines []string) {
		if rd.st.Lenient() {
			rd.st.Warn(blocks.JoinWarning(block, line), line, block, lines, model.Repaired)
		}
	}
	split := blocks.SplitAtEmptyLines(lines)
	if rd.cut != nil {
		split = blocks.DropCut(split, rd.cut)
	}
	return blocks.JoinCueTextBlocks(split, rd.rules, nil, onJoin)
}

func (rd *Reader) isTimingLine(line string) bool {
	if rd.st.Lenient() {
		return isLenientTimingLine(line)
	}
	return strictTimingLineRegexp.MatchString(line)
}

// isLenientTimingLine reports whether lenient mode reads line as a timing line. Strict mode reads fewer lines.
func isLenientTimingLine(line string) bool {
	return lenientTimingLineRegexp.MatchString(fullWidth.Replace(line))
}

// ParseBlock returns the cues of block b, number index, split before each timing line. Lenient mode skips a
// part that fails, and warns for each split and each skip.
func (rd *Reader) ParseBlock(b blocks.Block, index int) ([]model.Cue, error) {
	return rd.rules.ReadParts(b, index, blocks.PartReader{
		Lenient: rd.st.Lenient(),
		Split: func(msg string, line int) {
			rd.st.Warn(msg, line, index, b.Lines, model.Repaired)
		},
		Parse: func(lines []string, line int) (model.Cue, error) {
			return rd.parsePart(lines, index, line)
		},
		Fail: func(err *model.ParseError, line int, lines []string) error {
			return rd.st.Fail(err, line, index, lines)
		},
	})
}

// parsePart parses one part of a block, which starts at line. Lenient mode reads a part that starts with its
// timing line as if a cue number came before it.
func (rd *Reader) parsePart(part []string, index, line int) (model.Cue, error) {
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
func (rd *Reader) parseCue(lines, shown []string, index, line int) (model.Cue, error) {
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
func (rd *Reader) warnTimingRepairs(t timing, loose []looseTime, index, line int, shown []string) {
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

// readTimingLine splits a timing line at the first arrow that does not directly follow a "-". Strict mode reads
// only "-->". Lenient mode reads any run of "-" before ">" and the full-width delimiters.
func (rd *Reader) readTimingLine(line string) (timing, bool) {
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

func (rd *Reader) arrowLength(s string) int {
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
func (rd *Reader) time(text string, line int, loose *[]looseTime) (time.Duration, error) {
	text = strings.Trim(text, markupcore.PHPTrimSet)
	if m := timeRegexp.FindStringSubmatch(text); m != nil {
		return time.Duration(atoi(m[1]))*time.Hour + time.Duration(atoi(m[2]))*time.Minute +
			time.Duration(atoi(m[3]))*time.Second + fraction(m[4]+m[5]), nil
	}
	// A last field of 1 digit without a fraction is a time that the end of the file cut off.
	cutOff := len(text) >= 2 && text[len(text)-2] == ':' && isDigit(text[len(text)-1])
	if !rd.st.Lenient() || cutOff {
		return 0, invalidTime(text, line)
	}
	d, ok := timecode.Loose(text, ",.:", true)
	if !ok {
		return 0, invalidTime(text, line)
	}
	if d >= timecode.MaxTime {
		return parsing.BoundedTime(d, text, line)
	}
	if !containsTime(*loose, text) {
		*loose = append(*loose, looseTime{text: text, d: d})
	}
	return d, nil
}

func invalidTime(text string, line int) *model.ParseError {
	return &model.ParseError{Line: line, Msg: `The time "` + text + `" is not valid.`}
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
func (rd *Reader) applyTextAfterEndTime(c *model.Cue, rest string, numpad, index, line int, shown []string) error {
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
		model.Field{Key: "x1", Value: phpkey.DigitsToInt(m[1])},
		model.Field{Key: "x2", Value: phpkey.DigitsToInt(m[2])},
		model.Field{Key: "y1", Value: phpkey.DigitsToInt(m[3])},
		model.Field{Key: "y2", Value: phpkey.DigitsToInt(m[4])},
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
