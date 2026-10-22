package ttml

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

var (
	clockRegexp  = regexp.MustCompile(`^(\d{2,}):(\d{2}):(\d{2})(?:[.,](\d+)|:(\d{2})(?:\.(\d+))?)?$`)
	offsetRegexp = regexp.MustCompile(`^(\d+(?:\.\d+)?)(h|ms|m|s|f|t)$`)

	looseClockRegexp   = regexp.MustCompile(`^(\d+):(\d{1,2}):(\d{1,2})(?:[.,](\d+))?$`)
	looseMillisRegexp  = regexp.MustCompile(`^(\d+):(\d{1,2}):(\d{1,2}):(\d{3})$`)
	looseMinutesRegexp = regexp.MustCompile(`^(\d{1,2}):(\d{1,2})(?:[.,](\d+))?$`)
	bareMillisRegexp   = regexp.MustCompile(`^\d{1,15}$`)
	bareSecondsRegexp  = regexp.MustCompile(`^\d{1,15}\.\d{1,9}$`)

	phpFloatRegexp = regexp.MustCompile(`^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?`)
)

// clock reads time expressions with the timing parameters of one document, in float64 seconds as PHP computes them.
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
	expression = strings.Trim(expression, markupcore.PHPTrimSet)
	if m := clockRegexp.FindStringSubmatch(expression); m != nil {
		return c.clockTime(m), true
	}
	if m := offsetRegexp.FindStringSubmatch(expression); m != nil {
		return c.offsetTime(phpFloat(m[1]), m[2]), true
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
		return float64(phpInt(expression)) / 1000, true
	}
	if bareSecondsRegexp.MatchString(expression) {
		return phpFloat(expression), true
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

// clockSeconds returns the seconds of the digit fields of a clock time, as PHP reads the decimal text
// "whole.fraction". An hour count too large for an int64 sum gives a value far above the time limit.
func clockSeconds(hours, minutes, secs, fraction string) float64 {
	h := strings.TrimLeft(hours, "0")
	const maxHourDigits = 12
	if len(h) > maxHourDigits {
		v, _ := strconv.ParseFloat(h, 64)
		return v * 3600
	}
	whole := phpInt(hours)*3600 + phpInt(minutes)*60 + phpInt(secs)
	text := strconv.FormatInt(whole, 10)
	if fraction != "" {
		text += "." + fraction
	}
	v, _ := strconv.ParseFloat(text, 64)
	return v
}

// phpFloat reads s as the PHP cast (float) does: leading white space, then the longest decimal number, else 0.
func phpFloat(s string) float64 {
	m := phpFloatRegexp.FindString(strings.TrimLeft(s, " \t\n\r\v\f"))
	v, _ := strconv.ParseFloat(m, 64)
	return v
}

// phpInt reads s as the PHP cast (int) does, for the digit strings of the patterns. A value beyond int64 saturates.
func phpInt(s string) int64 {
	v := phpFloat(s)
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
		return secs + float64(phpInt(m[5]))/100
	}
	frames := float64(phpInt(m[5]))
	if m[6] != "" {
		frames += float64(phpInt(m[6])) / c.subFrameRate
	}
	if c.smpteTimeBase {
		return (secs*c.smpteFrameRate + frames - float64(c.droppedFrames(phpInt(m[1]), phpInt(m[2])))) / c.frameRate
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
