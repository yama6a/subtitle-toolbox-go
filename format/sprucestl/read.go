package sprucestl

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/stringlist"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

const (
	cueFields = `(\d{2,}):([0-5]\d):([0-5]\d):(\d{2})`
)

var (
	// The groups are the 4 fields of the start, the first separator, the 4 fields of the end and the text.
	cueRegexp       = regexp.MustCompile(`^` + cueFields + `([ \t]*,[ \t]*)` + cueFields + `[ \t]*,[ \t]*(.*)$`)
	lineBreakRegexp = regexp.MustCompile(`[ \t]*\|[ \t]*`)
	toggleRegexp    = regexp.MustCompile(`\^[BIU]`)
)

// cueLine is the parsed shape of a cue line.
type cueLine struct {
	start, end [4]string
	separator  string
	text       string
}

func parseCueLine(line string) (cueLine, bool) {
	m := cueRegexp.FindStringSubmatch(line)
	if m == nil {
		return cueLine{}, false
	}
	return cueLine{start: [4]string(m[1:5]), separator: m[5], end: [4]string(m[6:10]), text: m[10]}, true
}

// Read reads Spruce STL from UTF-8 text.
func Read(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	if err := opts.Validate(); err != nil {
		return nil, nil, err
	}
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.SpruceSTL, err)
	}
	st := parsing.NewState(model.SpruceSTL, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}

	lines := splitLines(string(content))

	rate := opts.FrameRate
	if rate.IsZero() {
		rate = guessFrameRate(st, lines)
	}
	rd := &reader{st: st, rate: rate, style: defaultStyle(), inHeader: true}
	for i, line := range lines {
		if err := rd.line(i+1, line); err != nil {
			return nil, nil, err
		}
	}
	if err := s.SetFormatData(formatDataKey, rd.fileData()); err != nil {
		return nil, nil, err
	}
	parsing.AddParsed(s, rd.cues, rd.comments)
	return s, st.Warnings(), nil
}

// splitLines returns the lines of content. The text after the last line end is no line.
func splitLines(content string) []string {
	var lines []string
	for _, line := range parsing.Lines(content) {
		lines = append(lines, line)
	}
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

// guessFrameRate returns 30 fps when a frame number of a cue line is 25 or more, else 25 fps. In lenient mode it
// warns when the file has a cue line.
//
// Adapted from mantas-done/subtitles, src/Code/Converters/StlConverter.php
// (https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/src/Code/Converters/StlConverter.php), MIT.
func guessFrameRate(st *parsing.State, lines []string) model.FrameRate {
	highest, found := 0, false
	for _, line := range lines {
		if c, ok := parseCueLine(strings.Trim(line, markupcore.PHPTrimSet)); ok {
			found = true
			highest = max(highest, atoi(c.start[3]), atoi(c.end[3]))
		}
	}
	rate := model.FrameRate{Num: 25, Den: 1}
	if highest >= 25 {
		rate.Num = 30
	}
	if found && st.Lenient() {
		st.Warn("The file does not give its frame rate. The parser read the frames at "+timecode.Number(rate.Float64())+" fps.",
			model.NoLine, model.NoBlock, nil, model.Repaired)
	}
	return rate
}

func atoi(digits string) int {
	n, _ := strconv.Atoi(digits)
	return n
}

type reader struct {
	st    *parsing.State
	rate  model.FrameRate
	style style

	inHeader  bool
	header    []string
	separator string
	lineBreak string
	commands  []string
	cues      []model.Cue
	comments  []parsing.ParsedComment
}

func (rd *reader) line(number int, line string) error {
	trimmed := strings.Trim(line, markupcore.PHPTrimSet)
	if cl, ok := parseCueLine(trimmed); ok {
		return rd.cue(number, line, cl)
	}
	isCommand, isComment := strings.HasPrefix(trimmed, "$"), strings.HasPrefix(trimmed, "//")
	if trimmed != "" && !isCommand && !isComment {
		err := &model.ParseError{Line: number, Msg: "The line " + parsing.Quote(trimmed) + " is no Spruce STL subtitle, command or comment."}
		return rd.st.Fail(err, number, model.NoBlock, []string{line})
	}
	rd.style.apply(line)
	switch {
	case rd.inHeader:
		rd.header = append(rd.header, line)
	case isComment:
		rd.comments = append(rd.comments, parsing.ParsedComment{Text: trimmed[2:], CuesBefore: len(rd.cues)})
	case trimmed != "":
		rd.commands = append(rd.commands, line)
	}
	return nil
}

func (rd *reader) cue(number int, line string, cl cueLine) error {
	rd.inHeader = false
	c, err := rd.readCue(number, line, cl)
	if err != nil {
		var perr *model.ParseError
		if !errors.As(err, &perr) {
			return err
		}
		return rd.st.Fail(perr, number, model.NoBlock, []string{line})
	}
	if rd.separator == "" {
		rd.separator = cl.separator
	}
	if rd.lineBreak == "" {
		rd.lineBreak = lineBreakRegexp.FindString(cl.text)
	}
	c.Position.Alignment = rd.style.alignment()
	c.Lines = cueText(cl.text, rd.style)
	if len(rd.commands) > 0 {
		_ = c.SetFormatData(formatDataKey, model.NewData(model.Field{Key: "commands", Value: stringlist.Any(rd.commands)})) // the data has the shape of the schema
		rd.commands = nil
	}
	rd.cues = append(rd.cues, c)
	return nil
}

func (rd *reader) readCue(number int, line string, cl cueLine) (model.Cue, error) {
	start, err := rd.time(cl.start, number)
	if err != nil {
		return model.Cue{}, err
	}
	end, err := rd.time(cl.end, number)
	if err != nil {
		return model.Cue{}, err
	}
	start, end, err = rd.st.OrderedTimes(start, end, number, model.NoBlock, []string{line}, true)
	return model.Cue{Start: start, End: end}, err
}

// time returns the time of the fields hours, minutes, seconds and frames in the clock model of model.FrameRate.
func (rd *reader) time(fields [4]string, line int) (time.Duration, error) {
	text := strings.Join(fields[:], ":")
	frames := atoi(fields[3])
	if frames >= frameLimit(rd.rate) {
		return 0, &model.ParseError{Line: line, Msg: `The frame number in "` + text + `" is not below the frame rate of ` +
			timecode.Number(rd.rate.Float64()) + " fps."}
	}
	hours, err := strconv.Atoi(fields[0])
	if err != nil {
		hours = timecode.MaxHours // more digits than an int holds
	}
	if hours >= timecode.MaxHours {
		return parsing.BoundedTime(timecode.MaxTime, text, line)
	}
	d := rd.rate.ClockDuration(model.Timecode{Hours: hours, Minutes: atoi(fields[1]), Seconds: atoi(fields[2]), Frames: frames})
	return parsing.BoundedTime(d.Round(time.Millisecond), text, line)
}

// cueText returns the lines of text in core markup. The toggles ^B, ^I and ^U flip the style of the commands, and the
// style carries over from one line of the cue to the next.
func cueText(text string, st style) []string {
	bold, italic, underline := st.bold, st.italic, st.underline
	var lines []string
	for _, part := range lineBreakRegexp.Split(text, -1) {
		part = strings.Trim(part, markupcore.PHPTrimSet)
		var runs []markupcore.Run
		from := 0
		for _, loc := range toggleRegexp.FindAllStringIndex(part, -1) {
			runs = appendRun(runs, part[from:loc[0]], bold, italic, underline)
			switch part[loc[0]+1] {
			case 'B':
				bold = !bold
			case 'I':
				italic = !italic
			case 'U':
				underline = !underline
			}
			from = loc[1]
		}
		runs = appendRun(runs, part[from:], bold, italic, underline)
		if line := strings.Trim(markupcore.StyleRuns(runs, true), markupcore.PHPTrimSet); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func appendRun(runs []markupcore.Run, text string, bold, italic, underline bool) []markupcore.Run {
	if text == "" {
		return runs
	}
	return append(runs, markupcore.Run{Text: text, Bold: bold, Italic: italic, Underline: underline})
}

func (rd *reader) fileData() *model.Data {
	data := model.NewData(
		model.Field{Key: "frameRate", Value: rd.rate.Float64()},
		model.Field{Key: "header", Value: stringlist.Any(rd.header)},
	)
	if rd.separator != "" {
		data.Set("separator", rd.separator)
	}
	if rd.lineBreak != "" {
		data.Set("lineBreak", rd.lineBreak)
	}
	if len(rd.commands) > 0 {
		data.Set("trailer", stringlist.Any(rd.commands))
	}
	return data
}
