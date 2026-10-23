package microdvd

import (
	"errors"
	"fmt"
	"io"
	"iter"
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

// lenientFrameRate is the default of mantas-done/subtitles, pysubs2 and subtitle_it, as a plain float and not 24000/1001.
const lenientFrameRate = 23.976

const frameRateOption = "ReadOptions.MicroDVD.FrameRate"

var frameRateLineRegexp = regexp.MustCompile(`^\{1\}\{1\}(\d+(?:\.\d+)?)$`)

// styleTags holds the style tags of {y:...} codes in the order of the tag nesting.
var styleTags = [...]string{"b", "i", "u", "s"} //nolint:gochecknoglobals // a fixed list

// rawLine is a non-empty input line after the trim.
type rawLine struct {
	number int // 1-based
	block  int // counts the non-empty lines from 0
	text   string
}

// cue holds the frames and the text of a line that has the cue shape {start}{end}text.
type cue struct {
	start, end string
	text       string
}

// style is the color and the style tags of a line or of a whole cue.
type style struct {
	color string
	tags  []string
}

// Read reads MicroDVD from UTF-8 text.
func Read(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	if err := opts.Validate(); err != nil {
		return nil, nil, err
	}
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.MicroDVD, err)
	}
	st := parsing.NewState(model.MicroDVD, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}

	lines := nonEmptyLines(string(content))
	fps, lines, hasLine, err := frameRate(st, lines, opts.FrameRate)
	if err != nil {
		return nil, nil, err
	}
	data := model.NewData(model.Field{Key: "frameRate", Value: fps})
	if hasLine {
		data.Set("frameRateLine", true)
	}
	_ = s.SetFormatData(formatDataKey, data) // the data has the shape of the schema

	cues, err := readCues(st, lines, fps)
	if err != nil {
		return nil, nil, err
	}
	s.AddCues(cues...)
	return s, st.Warnings(), nil
}

// readCues reads each line as a cue. Lenient mode skips a line that fails.
func readCues(st *parsing.State, lines []rawLine, fps float64) ([]model.Cue, error) {
	cues := make([]model.Cue, 0, len(lines))
	for _, l := range lines {
		c, err := readCue(st, l, fps)
		if err == nil {
			cues = append(cues, c)
			continue
		}
		var perr *model.ParseError
		if !errors.As(err, &perr) {
			return nil, err
		}
		if err := st.Fail(perr, l.number, l.block, []string{l.text}); err != nil {
			return nil, err
		}
	}
	return cues, nil
}

func nonEmptyLines(content string) []rawLine {
	var out []rawLine
	for n, line := range parsing.Lines(content) {
		if line = strings.Trim(line, markupcore.PHPTrimSet); line != "" {
			out = append(out, rawLine{number: n + 1, block: len(out), text: line})
		}
	}
	return out
}

// frameRate returns the frame rate of the read and lines without the {1}{1}<fps> line. hasLine reports that line.
func frameRate(st *parsing.State, lines []rawLine, option float64) (fps float64, rest []rawLine, hasLine bool, err error) {
	if first := firstLine(st, lines); first >= 0 {
		if m := frameRateLineRegexp.FindStringSubmatch(lines[first].text); m != nil {
			fps, err := lineFrameRate(m[1], option)
			return fps, slices.Delete(slices.Clone(lines), first, first+1), err == nil, err
		}
	}
	switch {
	case option != 0:
		return option, lines, false, nil
	case st.Lenient():
		st.Warn("The file has no {1}{1}<fps> line. The parser used "+timecode.Number(lenientFrameRate)+" fps.",
			model.NoLine, model.NoBlock, nil, model.Repaired)
		return lenientFrameRate, lines, false, nil
	default:
		return 0, nil, false, &model.ParseError{
			Format: model.MicroDVD,
			Msg: "The frame rate is unknown. Start the file with {1}{1}<fps>, set the frame rate, or read the file in " +
				"lenient mode for " + timecode.Number(lenientFrameRate) + " fps.",
			Option: frameRateOption,
		}
	}
}

// firstLine returns the index of the line that may hold the frame rate, or -1. Strict mode takes the first line.
// Lenient mode takes the first line with the cue shape, so the frame rate line may follow junk such as a release name.
func firstLine(st *parsing.State, lines []rawLine) int {
	for i, l := range lines {
		if _, ok := parseCueShape(l.text); !st.Lenient() || ok {
			return i
		}
	}
	return -1
}

// lineFrameRate returns option when it is set, else the number of a {1}{1}<fps> line. The option wins without a
// check of the line.
func lineFrameRate(number string, option float64) (float64, error) {
	if option != 0 {
		return option, nil
	}
	fps, _ := strconv.ParseFloat(number, 64) // a value beyond the float range is +Inf, which the check rejects
	if checkFrameRate(fps, "", "") != nil {
		return 0, &model.ParseError{Format: model.MicroDVD, Msg: frameRateMessage("The frame rate", fps)}
	}
	return fps, nil
}

// parseCueShape splits a line of the shape ^\{(\d+)\}\{(\d+)\}(.*)$.
func parseCueShape(line string) (cue, bool) {
	start, rest, ok := frameNumber(line)
	if !ok {
		return cue{}, false
	}
	end, rest, ok := frameNumber(rest)
	if !ok {
		return cue{}, false
	}
	return cue{start: start, end: end, text: rest}, true
}

// frameNumber splits {digits} off the start of s.
func frameNumber(s string) (digits, rest string, ok bool) {
	if !strings.HasPrefix(s, "{") {
		return "", "", false
	}
	n := 1
	for n < len(s) && s[n] >= '0' && s[n] <= '9' {
		n++
	}
	if n == 1 || n == len(s) || s[n] != '}' {
		return "", "", false
	}
	return s[1:n], s[n+1:], true
}

func readCue(st *parsing.State, l rawLine, fps float64) (model.Cue, error) {
	shape, ok := parseCueShape(l.text)
	if !ok {
		return model.Cue{}, &model.ParseError{Line: l.number, Msg: `The line "` + l.text + `" is not a MicroDVD cue.`}
	}
	start, err := frameTime(shape.start, fps, l.number)
	if err != nil {
		return model.Cue{}, err
	}
	end, err := frameTime(shape.end, fps, l.number)
	if err != nil {
		return model.Cue{}, err
	}
	if start, end, err = st.OrderedTimes(start, end, l.number, l.block, []string{l.text}, true); err != nil {
		return model.Cue{}, err
	}
	c := model.Cue{Start: start, End: end}
	lines, data := cueText(shape.text)
	c.Lines = lines
	_ = c.SetFormatData(formatDataKey, model.NewData(model.Field{Key: "lines", Value: data})) // the data has the shape of the schema
	return c, nil
}

// frameTime returns the time of a frame number. Digits beyond the int64 range read as its maximum, as PHP (int)
// reads them. The seconds are checked against the limit before the conversion to a duration, which would overflow,
// and the duration after the rounding.
func frameTime(digits string, fps float64, line int) (time.Duration, error) {
	n, _ := strconv.ParseInt(digits, 10, 64)
	secs := float64(n) / fps
	d := timecode.MaxTime
	if secs < timecode.MaxTime.Seconds() {
		d = timecode.FromSeconds(secs)
	}
	return parsing.BoundedTime(d, "{"+digits+"}", line)
}

// textLine is a kept | line of a cue: its text, its raw code prefix, the codes outside core markup and its style.
type textLine struct {
	text, codes, other string
	style              style
}

// cueText returns the lines of the text after the frames in core markup, and the lines entry of the format data.
// Only the codes at the start of each | line count. An upper-case code letter styles every line of the cue, a
// lower-case letter its own line.
func cueText(text string) ([]string, []any) {
	var cueStyle style
	var kept []textLine
	for part := range strings.SplitSeq(text, "|") {
		if l, ok := parseLine(part, &cueStyle); ok {
			kept = append(kept, l)
		}
	}
	lines := make([]string, len(kept))
	data := make([]any, len(kept))
	for i, l := range kept {
		lines[i], data[i] = l.markup(cueStyle)
	}
	return lines, data
}

// parseLine reads the codes of one | line into its own style and into cue. False means that the line has no text.
func parseLine(part string, cue *style) (textLine, bool) {
	l := textLine{codes: codePrefix(part)}
	var other strings.Builder
	for code := range codes(l.codes) {
		letter, value := code[1], code[3:len(code)-1]
		target := &l.style
		if letter >= 'A' && letter <= 'Z' {
			target = cue
		}
		if !applyCode(target, letter|0x20, value) {
			other.WriteString(code)
		}
	}
	l.other = other.String()
	l.text = strings.Trim(part[len(l.codes):], markupcore.PHPTrimSet)
	return l, l.text != ""
}

// markup returns the line in core markup with the styles of the line and of cue, and its format data entry.
func (l textLine) markup(cue style) (string, *model.Data) {
	color := l.style.color
	if color == "" {
		color = cue.color
	}
	tags := orderedTags(cue.tags, l.style.tags)
	run := markupcore.Run{
		Text: l.text, Color: color,
		Bold: slices.Contains(tags, "b"), Italic: slices.Contains(tags, "i"),
		Underline: slices.Contains(tags, "u"), Strike: slices.Contains(tags, "s"),
	}
	tagList := make([]any, len(tags))
	for i, tag := range tags {
		tagList[i] = tag
	}
	var storedColor any
	if color != "" {
		storedColor = color
	}
	return markupcore.StyleRuns([]markupcore.Run{run}, false), model.NewData(
		model.Field{Key: "codes", Value: l.codes},
		model.Field{Key: "otherCodes", Value: l.other},
		model.Field{Key: "color", Value: storedColor},
		model.Field{Key: "tags", Value: tagList},
	)
}

// codePrefix returns the run of codes at the start of line, as ^(?:\{[A-Za-z]:[^}]*\})* matches it.
func codePrefix(line string) string {
	n := 0
	for {
		rest := line[n:]
		if len(rest) < 3 || rest[0] != '{' || !isASCIILetter(rest[1]) || rest[2] != ':' {
			return line[:n]
		}
		end := strings.IndexByte(rest[3:], '}')
		if end < 0 {
			return line[:n]
		}
		n += 3 + end + 1
	}
}

// codes yields the codes of a prefix that codePrefix returned.
func codes(prefix string) iter.Seq[string] {
	return func(yield func(string) bool) {
		for prefix != "" {
			end := strings.IndexByte(prefix, '}') + 1
			if !yield(prefix[:end]) {
				return
			}
			prefix = prefix[end:]
		}
	}
}

// applyCode applies a {y:...} or {c:$BBGGRR} code to s. It returns false for any other code.
func applyCode(s *style, letter byte, value string) bool {
	switch letter {
	case 'y':
		tags, ok := styleCode(value)
		s.tags = append(s.tags, tags...)
		return ok
	case 'c':
		if len(value) != 7 || value[0] != '$' || strings.Trim(value[1:], "0123456789abcdefABCDEF") != "" {
			return false
		}
		s.color = "#" + markupcore.ASCIILower(markupcore.BGRToRGB(value[1:]))
		return true
	default:
		return false
	}
}

// styleCode returns the style tags of the value of a {y:...} code: only the letters b, i, u and s in any case,
// commas and spaces.
func styleCode(value string) ([]string, bool) {
	if value == "" || strings.Trim(value, "biusBIUS, ") != "" {
		return nil, false
	}
	var tags []string
	for i := range len(value) {
		if c := value[i]; c != ',' && c != ' ' {
			tags = append(tags, string(c|0x20))
		}
	}
	return tags, true
}

// orderedTags returns the style tags of cue and line without repeats, in the order b, i, u, s.
func orderedTags(cue, line []string) []string {
	var out []string
	for _, tag := range styleTags {
		for _, t := range [][]string{cue, line} {
			if slices.Contains(t, tag) {
				out = append(out, tag)
				break
			}
		}
	}
	return out
}

func isASCIILetter(c byte) bool {
	return 'a' <= c|0x20 && c|0x20 <= 'z'
}
