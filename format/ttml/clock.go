package ttml

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/internal/ttmldoc"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

var (
	clockRegexp  = regexp.MustCompile(`^(\d{2,}):(\d{2}):(\d{2})(?:[.,](\d+)|:(\d{2})(?:\.(\d+))?)?$`)
	offsetRegexp = regexp.MustCompile(`^(\d+(?:\.\d+)?)(h|ms|m|s|f|t)$`)

	looseClockRegexp   = regexp.MustCompile(`^(\d+):(\d{1,2}):(\d{1,2})(?:[.,](\d+))?$`)
	looseMillisRegexp  = regexp.MustCompile(`^(\d+):(\d{1,2}):(\d{1,2}):(\d{3})$`)
	looseMinutesRegexp = regexp.MustCompile(`^(\d{1,2}):(\d{1,2})(?:[.,](\d+))?$`)
	bareMillisRegexp   = regexp.MustCompile(`^\d{1,15}$`)
	bareSecondsRegexp  = regexp.MustCompile(`^\d{1,15}\.\d{1,9}$`)
)

// clock reads time expressions with the timing parameters of one document, in float64 seconds.
// See https://www.w3.org/TR/ttml2/#timing-value-time-expression.
type clock struct {
	// frameRate is ttp:frameRate with the multiplier, smpteFrameRate the frame labels per second without it.
	frameRate, smpteFrameRate float64
	smpteTimeBase             bool
	dropMode                  string
	subFrameRate, tickRate    float64
	// hundredths reads the last field of hh:mm:ss:ff as hundredths of a second, a lenient repair.
	hundredths bool
}

// seconds returns the seconds of a clock time or an offset time, with no upper bound.
func (c clock) seconds(expression string) (float64, bool) {
	expression = strings.Trim(expression, markupcore.TrimSet)
	if m := clockRegexp.FindStringSubmatch(expression); m != nil {
		return c.clockTime(m), true
	}
	if m := offsetRegexp.FindStringSubmatch(expression); m != nil {
		return c.offsetTime(ttmldoc.LeadingFloat(m[1]), m[2]), true
	}
	return 0, false
}

// looseSeconds reads the time expressions that TTML does not allow but some tools write: one-digit clock fields,
// m:ss without hours, hh:mm:ss:fff with milliseconds, a bare integer of milliseconds, and a bare number with a
// fraction, which is in seconds. The expression is trimmed.
func looseSeconds(expression string) (float64, bool) {
	if m := looseClockRegexp.FindStringSubmatch(expression); m != nil {
		return clockSeconds(m[1], m[2], m[3], m[4]), true
	}
	if m := looseMillisRegexp.FindStringSubmatch(expression); m != nil {
		return clockSeconds(m[1], m[2], m[3], m[4]), true
	}
	if m := looseMinutesRegexp.FindStringSubmatch(expression); m != nil {
		return clockSeconds("0", m[1], m[2], m[3]), true
	}
	if bareMillisRegexp.MatchString(expression) {
		return float64(leadingInt(expression)) / 1000, true
	}
	if bareSecondsRegexp.MatchString(expression) {
		return ttmldoc.LeadingFloat(expression), true
	}
	return 0, false
}

// droppedFrames counts the frame labels that drop-frame time code skips before hh:mm:00, as in SMPTE ST 12-1.
// See https://www.w3.org/TR/ttml1/#time-expression-semantics-smpte.
func (c clock) droppedFrames(hours, minutes int64) int64 {
	switch c.dropMode {
	case "dropNTSC":
		return (hours*54 + minutes - minutes/10) * 2
	case "dropPAL":
		return (hours*27 + minutes/2 - minutes/20) * 4
	default:
		return 0
	}
}

// clockSeconds returns the seconds of the digit fields of a clock time as the decimal text
// "whole.fraction". An hour count too large for an int64 sum gives a value far above the time limit.
func clockSeconds(hours, minutes, secs, fraction string) float64 {
	h := strings.TrimLeft(hours, "0")
	const maxHourDigits = 12
	if len(h) > maxHourDigits {
		v, _ := strconv.ParseFloat(h, 64)
		return v * 3600
	}
	whole := leadingInt(hours)*3600 + leadingInt(minutes)*60 + leadingInt(secs)
	text := strconv.FormatInt(whole, 10)
	if fraction != "" {
		text += "." + fraction
	}
	v, _ := strconv.ParseFloat(text, 64)
	return v
}

// leadingInt reads the integer at the start of s, for the digit strings of the patterns. A value beyond int64 saturates.
func leadingInt(s string) int64 {
	v := ttmldoc.LeadingFloat(s)
	switch {
	case v >= math.MaxInt64:
		return math.MaxInt64
	case v <= math.MinInt64:
		return math.MinInt64
	default:
		n, err := strconv.ParseInt(strings.TrimLeft(s, " \t\n\r\v\f"), 10, 64)
		if err != nil {
			return int64(v)
		}
		return n
	}
}

// clockTime returns the seconds of the submatches of clockRegexp: hours, minutes, seconds, then a fraction or
// frames and sub-frames.
func (c clock) clockTime(m []string) float64 {
	secs := clockSeconds(m[1], m[2], m[3], m[4])
	if m[4] != "" {
		return secs
	}
	if c.hundredths && m[5] != "" {
		return secs + float64(leadingInt(m[5]))/100
	}
	frames := float64(leadingInt(m[5]))
	if m[6] != "" {
		frames += float64(leadingInt(m[6])) / c.subFrameRate
	}
	if c.smpteTimeBase {
		return (secs*c.smpteFrameRate + frames - float64(c.droppedFrames(leadingInt(m[1]), leadingInt(m[2])))) / c.frameRate
	}
	return secs + frames/c.frameRate
}

func (c clock) offsetTime(count float64, unit string) float64 {
	switch unit {
	case "h":
		return count * 3600
	case "m":
		return count * 60
	case "s":
		return count
	case "ms":
		return count / 1000
	case "f":
		return count / c.frameRate
	default:
		return count / c.tickRate
	}
}

// interval returns the interval of n inside the interval of its parent. The end is the earliest of the parent end,
// the parent begin plus the own end, and the own begin plus the duration.
func (r *reader) interval(n *xmlraw.Node, parent interval) (interval, *model.ParseError) {
	iv := interval{begin: parent.begin, end: parent.end, hasEnd: parent.hasEnd}
	if _, ok := ttmldoc.PlainAttr(n, "begin"); ok {
		var err *model.ParseError
		if iv.begin, err = r.offsetTime(parent.begin, n, "begin"); err != nil {
			return interval{}, err
		}
	}
	for _, attr := range []struct {
		name string
		base float64
	}{{"end", parent.begin}, {"dur", iv.begin}} {
		v, _ := ttmldoc.PlainAttr(n, attr.name)
		if strings.Trim(v, markupcore.TrimSet) == "" {
			continue
		}
		t, err := r.offsetTime(attr.base, n, attr.name)
		if err != nil {
			return interval{}, err
		}
		if !iv.hasEnd || t < iv.end {
			iv.end, iv.hasEnd = t, true
		}
	}
	return iv, nil
}

// offsetTime returns base plus the time of the attribute. Lenient mode reads the forms of looseSeconds too.
func (r *reader) offsetTime(base float64, n *xmlraw.Node, attr string) (float64, *model.ParseError) {
	v, _ := ttmldoc.PlainAttr(n, attr)
	expression := strings.Trim(v, markupcore.TrimSet)
	secs, err := r.boundedExpression(expression)
	if err == nil {
		return r.bounded(base+secs, expression, n.Line)
	}
	if !r.st.Lenient() {
		return 0, err
	}
	loose, ok := looseSeconds(expression)
	if !ok {
		return 0, err
	}
	t, berr := r.bounded(base+loose, expression, n.Line)
	if berr != nil {
		return 0, berr
	}
	block := model.NoBlock
	if r.isInParagraph(n) {
		block = r.paragraph
	}
	r.st.Warn("The time expression \""+expression+"\" is not valid. The parser read it as "+timecode.Number(loose)+"s.",
		n.Line, block, nil, model.Repaired)
	return t, nil
}

func (r *reader) boundedExpression(expression string) (float64, *model.ParseError) {
	secs, ok := r.clock.seconds(expression)
	if !ok {
		return 0, r.parseError(model.NoLine, "the time expression \""+expression+"\" is not valid")
	}
	return r.bounded(secs, expression, model.NoLine)
}

// bounded returns secs when the time rounds to whole milliseconds below 100000 hours in size.
func (r *reader) bounded(secs float64, text string, line int) (float64, *model.ParseError) {
	if !(math.Abs(secs) < timecode.MaxTime.Seconds()) || timecode.FromSeconds(math.Abs(secs)) >= timecode.MaxTime {
		return 0, r.parseError(line, "the time \""+text+"\" is not below "+strconv.Itoa(timecode.MaxHours)+" hours")
	}
	return secs, nil
}
