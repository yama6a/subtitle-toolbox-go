package qttext

import (
	"errors"
	"fmt"
	"io"
	"math/bits"
	"slices"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/phpkey"
	"github.com/yama6a/subtitle-toolbox-go/internal/stringlist"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

type sample struct {
	start time.Duration
	line  int
	lines []string
}

type reader struct {
	st       *parsing.State
	scale    int64
	relative bool
}

// Read reads QuickTime text from UTF-8 text.
func Read(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.QuickTimeText, err)
	}
	st := parsing.NewState(model.QuickTimeText, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}

	header, body := splitHeader(string(content))
	if err := checkHeader(st, header); err != nil {
		return nil, nil, err
	}
	rd := &reader{st: st, scale: timeScale(header), relative: isRelative(header)}
	var running style
	readLines(header, &running)
	samples, paired, err := rd.samples(body, len(header))
	if err != nil {
		return nil, nil, err
	}
	cues, err := rd.cues(samples, &running)
	if err != nil {
		return nil, nil, err
	}
	if err := setFileData(s, header, paired); err != nil {
		return nil, nil, err
	}
	s.AddCues(cues...)
	return s, st.Warnings(), nil
}

func splitHeader(content string) (header, body []string) {
	var lines []string
	for _, line := range parsing.Lines(content) {
		lines = append(lines, line)
	}
	n := 0
	for n < len(lines) && !isTimestampLine(lines[n]) {
		n++
	}
	return lines[:n], lines[n:]
}

func setFileData(s *model.Subtitle, header []string, paired bool) error {
	data := &model.Data{}
	data.Set("header", stringlist.Any(header))
	if paired {
		data.Set("pairedTimestamps", true)
	}
	if err := s.SetFormatData(formatDataKey, data); err != nil {
		return fmt.Errorf("%s file data: %w", model.QuickTimeText, err)
	}
	return nil
}

func checkHeader(st *parsing.State, header []string) error {
	if len(header) > 0 && headerRegexp.MatchString(header[0]) {
		return nil
	}
	const msg = "The file does not start with {QTtext}."
	if !st.Lenient() {
		return &model.ParseError{Format: model.QuickTimeText, Line: 1, Msg: msg}
	}
	st.Warn(msg+" The parser read it as QuickTime text.", 1, model.NoBlock, nil, model.Repaired)
	return nil
}

type sampler struct {
	rd       *reader
	samples  []sample
	previous time.Duration
	paired   bool
	// blank is true after an empty line that follows a sample without text.
	blank bool
	// skipping is true after a timestamp line that lenient mode skipped, until the next timestamp line.
	skipping bool
}

// samples keeps a line that is no timestamp line, as raw text, with the sample before it.
func (rd *reader) samples(lines []string, offset int) ([]sample, bool, error) {
	sp := &sampler{rd: rd}
	for i, line := range lines {
		trimmed := strings.Trim(line, markupcore.PHPTrimSet)
		m := timestampRegexp.FindStringSubmatch(trimmed)
		if m == nil {
			sp.addText(line, trimmed)
		} else if err := sp.addStamp(m, line, trimmed, offset+i+1); err != nil {
			return nil, false, err
		}
	}
	return sp.samples, sp.paired, nil
}

func (sp *sampler) addText(line, trimmed string) {
	if sp.skipping {
		return
	}
	// Subtitle Edit writes an empty line after the end timestamp of each cue.
	n := len(sp.samples)
	sp.blank = sp.blank || (trimmed == "" && n > 1 && len(sp.samples[n-1].lines) == 0)
	if trimmed != "" && n > 0 {
		sp.samples[n-1].lines = append(sp.samples[n-1].lines, line)
		sp.blank = false
	}
}

func (sp *sampler) addStamp(m []string, line, trimmed string, number int) error {
	start, err := sp.rd.time(m, number)
	if err == nil && sp.rd.relative {
		start, err = parsing.BoundedTime(sp.previous+start, m[0][1:len(m[0])-1], number)
	}
	var perr *model.ParseError
	if errors.As(err, &perr) {
		sp.skipping = true
		return sp.rd.st.Fail(perr, number, model.NoBlock, []string{line})
	}
	sp.skipping = false
	sp.paired = sp.paired || sp.blank
	sp.blank = false
	sp.previous = start
	sm := sample{start: start, line: number}
	if rest := trimmed[len(m[0]):]; strings.Trim(rest, markupcore.PHPTrimSet) != "" {
		sm.lines = []string{rest}
	}
	sp.samples = append(sp.samples, sm)
	return nil
}

func (rd *reader) cues(samples []sample, running *style) ([]model.Cue, error) {
	var cues []model.Cue
	for i, sm := range samples {
		if len(sm.lines) == 0 {
			continue
		}
		end := rd.st.LastCueEnd(sm.start)
		if i+1 < len(samples) {
			end = samples[i+1].start
		}
		start, end, err := rd.st.OrderedTimes(sm.start, end, sm.line, model.NoBlock, sm.lines, true)
		var perr *model.ParseError
		if errors.As(err, &perr) {
			err = rd.st.Fail(perr, sm.line, model.NoBlock, sm.lines)
		}
		if err != nil {
			return nil, err
		}
		if perr != nil {
			continue
		}
		text := readLines(sm.lines, running)
		if len(text) == 0 {
			continue
		}
		c := model.Cue{Start: start, End: end, Lines: text}
		if hasDescriptor(sm.lines) {
			data := model.NewData(model.Field{Key: "source", Value: stringlist.Any(sm.lines)})
			if err := c.SetFormatData(formatDataKey, data); err != nil {
				return nil, fmt.Errorf("%s cue data: %w", model.QuickTimeText, err)
			}
		}
		cues = append(cues, c)
	}
	return cues, nil
}

func hasDescriptor(lines []string) bool {
	return slices.ContainsFunc(lines, descriptorRegexp.MatchString)
}

// time computes in integers, so the 100000 hour limit applies to the time after the rounding to milliseconds.
func (rd *reader) time(m []string, line int) (time.Duration, error) {
	text, fraction := m[0][1:len(m[0])-1], m[4]
	var units int64
	if rd.scale > 0 && fraction != "" {
		units = phpkey.DigitsToInt(fraction)
		if units >= rd.scale {
			msg := fmt.Sprintf("The timestamp %q counts %s units, but the time scale is %d per second.", m[0], fraction, rd.scale)
			if !rd.st.Lenient() {
				return 0, &model.ParseError{Line: line, Msg: msg}
			}
			rd.st.Warn(msg+" The parser added them to the seconds.", line, model.NoBlock, []string{m[0]}, model.Repaired)
		}
	}

	hours := phpkey.DigitsToInt(m[1])
	if hours >= timecode.MaxHours {
		return parsing.BoundedTime(timecode.MaxTime, text, line)
	}
	ms := ((hours*60+phpkey.DigitsToInt(m[2]))*60 + phpkey.DigitsToInt(m[3])) * 1000
	switch {
	case fraction == "":
	case rd.scale == 0:
		ms += decimalMillis(fraction)
	default:
		whole, part := uint64(units)/uint64(rd.scale), uint64(units)%uint64(rd.scale)
		if whole >= timecode.MaxHours*3600 {
			return parsing.BoundedTime(timecode.MaxTime, text, line)
		}
		ms += int64(whole)*1000 + int64(scaledMillis(part, uint64(rd.scale)))
	}
	return parsing.BoundedTime(time.Duration(ms)*time.Millisecond, text, line)
}

func decimalMillis(digits string) int64 {
	var ms int64
	for i := range 3 {
		ms *= 10
		if i < len(digits) {
			ms += int64(digits[i] - '0')
		}
	}
	if len(digits) > 3 && digits[3] >= '5' {
		ms++
	}
	return ms
}

// scaledMillis returns part/scale seconds in whole milliseconds, rounded half up. It needs part < scale.
func scaledMillis(part, scale uint64) uint64 {
	hi, lo := bits.Mul64(part, 1000)
	ms, rem := bits.Div64(hi, lo, scale)
	if rem >= scale-rem {
		ms++
	}
	return ms
}
