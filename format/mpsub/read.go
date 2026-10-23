package mpsub

import (
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

var (
	headerRegexp  = regexp.MustCompile(`^([A-Z]+)=(.*?)([ \t]+#.*)?$`)
	timingRegexp  = regexp.MustCompile(`^(-?\d+(?:\.\d+)?)[ \t]+(-?\d+(?:\.\d+)?)$`)
	leadingDigits = regexp.MustCompile(`^\d+`)
)

// reader holds the state of one read. The line loop in Read is the state machine of docs/formats.md.
type reader struct {
	st   *parsing.State
	s    *model.Subtitle
	data *model.Data
	cues []model.Cue

	// fps is the frame rate of the FORMAT line, 0 for seconds.
	fps       float64
	hasFormat bool
	// position is the end of the last timing line in seconds. It stays unrounded, as in PHP, so that rounding
	// errors of fractional frames do not add up differently.
	position float64
	// timings counts the timing lines so far, good and bad. It is the block index of warnings.
	timings int

	open      *model.Cue
	openLine  int
	openBlock int
	openText  string
	skipping  bool
}

// Read reads MPSub from UTF-8 text.
func Read(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.MPSub, err)
	}
	st := parsing.NewState(model.MPSub, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}

	rd := &reader{st: st, s: s, data: model.NewData()}
	for n, raw := range parsing.Lines(string(content)) {
		if err := rd.line(strings.Trim(raw, markupcore.PHPTrimSet), n+1); err != nil {
			return nil, nil, err
		}
	}
	if err := rd.endCue(); err != nil {
		return nil, nil, err
	}
	_ = s.SetFormatData(formatDataKey, rd.data) // the data holds strings, as the schema says
	s.AddCues(rd.cues...)
	return s, st.Warnings(), nil
}

func (rd *reader) line(line string, number int) error {
	switch {
	case rd.open != nil && line != "":
		rd.open.Lines = append(rd.open.Lines, markupcore.EscapeText(line))
		return nil
	case rd.open != nil:
		return rd.endCue()
	case rd.skipping:
		rd.skipping = line != ""
		return nil
	case line == "" || strings.HasPrefix(line, "#"):
		return nil
	}
	if m := headerRegexp.FindStringSubmatch(line); m != nil {
		return rd.header(m[1], strings.Trim(m[2], markupcore.PHPTrimSet), line, number)
	}
	return rd.startCue(line, number)
}

func (rd *reader) header(key, value, line string, number int) error {
	switch key {
	case "FORMAT":
		rd.hasFormat = true
		fps, err := frameRate(value, number)
		if err != nil {
			return rd.st.Fail(err, number, rd.timings, []string{line})
		}
		rd.fps = fps
	case "TITLE":
		rd.setMetadata(model.MetadataTitle, value)
	case "AUTHOR":
		rd.setMetadata(model.MetadataAuthor, value)
	default:
		if value != "" {
			rd.data.Set(key, value)
		}
	}
	return nil
}

func (rd *reader) setMetadata(key, value string) {
	if value == "" {
		rd.s.DeleteMetadata(key)
		return
	}
	rd.s.SetMetadata(key, value)
}

// frameRate returns the frame rate of a FORMAT value, 0 for TIME. Only the leading integer counts, as in MPlayer
// and FFmpeg, so 29.97 gives 29. A run of digits beyond the int64 range counts as its maximum, as PHP (int) reads it.
func frameRate(value string, line int) (float64, *model.ParseError) {
	if value == "TIME" {
		return 0, nil
	}
	digits := leadingDigits.FindString(value)
	if digits == "" {
		return 0, &model.ParseError{Line: line, Msg: `The FORMAT value "` + value + `" is not known.`}
	}
	n, _ := strconv.ParseInt(digits, 10, 64)
	if n == 0 {
		return 0, &model.ParseError{Line: line, Msg: `The frame rate "` + value + `" is not valid.`}
	}
	return float64(n), nil
}

func (rd *reader) startCue(line string, number int) error {
	block := rd.timings
	rd.timings++
	if rd.st.Lenient() && !rd.hasFormat {
		rd.st.Warn("The file has no FORMAT line before line "+strconv.Itoa(number)+". The parser read the times as seconds.",
			number, block, []string{line}, model.Repaired)
		rd.hasFormat = true
	}
	c, err := rd.timingLine(line, number)
	if err != nil {
		rd.skipping = true
		return rd.st.Fail(err, number, block, []string{line})
	}
	rd.open, rd.openLine, rd.openBlock, rd.openText = &c, number, block, line
	return nil
}

// timingLine returns the cue of a timing line without text and moves the position to its end.
func (rd *reader) timingLine(line string, number int) (model.Cue, *model.ParseError) {
	m := timingRegexp.FindStringSubmatch(line)
	if m == nil {
		return model.Cue{}, &model.ParseError{Line: number, Msg: `The line "` + line + `" is not a header, a comment or a timing line.`}
	}
	wait, duration := rd.seconds(m[1]), rd.seconds(m[2])
	if duration < 0 {
		return model.Cue{}, &model.ParseError{Line: number, Msg: `The timing line "` + line + `" has a negative duration.`}
	}
	startSecs := rd.position + wait
	endSecs := startSecs + duration
	start, err := cueTime(startSecs, line, number)
	if err != nil {
		return model.Cue{}, err
	}
	end, err := cueTime(endSecs, line, number)
	if err != nil {
		return model.Cue{}, err
	}
	rd.position = endSecs
	return model.Cue{Start: start, End: end}, nil
}

// seconds converts a number of the timing line to seconds. Fractional frames are allowed.
func (rd *reader) seconds(number string) float64 {
	v, _ := strconv.ParseFloat(number, 64) // digits beyond the float range give +Inf or -Inf, which cueTime rejects
	if rd.fps == 0 {
		return v
	}
	return v / rd.fps
}

// cueTime returns secs as a time rounded to whole milliseconds. It returns an error unless the result is below
// 100000 hours in size. The caller passes the sum of the relative times, because that is the time of the cue.
func cueTime(secs float64, line string, number int) (time.Duration, *model.ParseError) {
	if d, ok := timeOf(secs); ok {
		return d, nil
	}
	_, err := parsing.BoundedTime(timecode.MaxTime, line, number)
	return 0, err.(*model.ParseError) //nolint:errorlint,forcetypeassert // BoundedTime fails with a *ParseError
}

// timeOf rounds secs to whole milliseconds. False means that the result is 100000 hours or more in size. The check
// of secs comes first, because a larger value would overflow a time.Duration.
func timeOf(secs float64) (time.Duration, bool) {
	if math.Abs(secs) < timecode.MaxTime.Seconds() {
		if d := timecode.FromSeconds(secs); d > -timecode.MaxTime && d < timecode.MaxTime {
			return d, true
		}
	}
	return 0, false
}

// endCue ends the open cue. A cue without text lines fails.
func (rd *reader) endCue() error {
	c := rd.open
	if c == nil {
		return nil
	}
	rd.open = nil
	if len(c.Lines) == 0 {
		return rd.st.Fail(&model.ParseError{Line: rd.openLine, Msg: "The cue has no text lines."}, rd.openLine, rd.openBlock, []string{rd.openText})
	}
	rd.cues = append(rd.cues, *c)
	return nil
}
